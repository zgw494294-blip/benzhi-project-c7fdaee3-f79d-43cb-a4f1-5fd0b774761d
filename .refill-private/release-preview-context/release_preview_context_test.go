package releasepreviewcontext_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func TestReleasePreviewHonorsRequestCancellation(t *testing.T) {
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })

	aggregate := &domain.Aggregate{
		Package: domain.InterviewPackage{
			ID: "preview-context", Topic: "地方记忆", ParticipantCode: "P-01",
			OwnerName: "资料负责人", IntendedScope: "公开展示",
			Status: domain.StatusApprovalPending, Version: 7, CreatedAt: now, UpdatedAt: now,
		},
		Consent: &domain.ConsentRecord{
			PackageID: "preview-context", Terms: "同意公开", AllowedUses: []string{"公开展示"},
			AttributionPreference: "匿名", ConfirmedAt: now, ConfirmedBy: "参与者", TermsDigest: "sha256:terms",
		},
		Segments: []domain.TranscriptSegment{{
			ID: "s1", PackageID: "preview-context", Sequence: 1,
			SourceText: "可公开内容", Decision: domain.DecisionPublic, PublicText: "可公开内容", Revision: 1,
		}},
		Revisions: []domain.SegmentRevision{}, Findings: []domain.ReviewFinding{}, Events: []domain.BusinessEvent{}, Round: 1,
	}
	_, _, err = repository.Commit(store.CommitRequest{
		PackageID: "preview-context", ExpectedVersion: 0, IdempotencyKey: "seed-preview",
		RequestDigest: "sha256:seed-preview", Create: aggregate,
		Mutate: func(_ *domain.Aggregate, _ func() uint64) (json.RawMessage, error) {
			return json.RawMessage(`{"seeded":true}`), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var clockCalls atomic.Int32
	service := application.NewServiceWithClock(repository, func() time.Time {
		clockCalls.Add(1)
		return now
	})
	preCanceled, cancelPreCanceled := context.WithCancel(context.Background())
	cancelPreCanceled()
	if _, err := service.PreviewReleaseContext(preCanceled, "preview-context"); !errors.Is(err, context.Canceled) {
		t.Errorf("预取消请求应返回 context.Canceled，实际为 %v", err)
	}
	if calls := clockCalls.Load(); calls != 0 {
		t.Errorf("预取消请求不应进入 Clock，实际调用 %d 次", calls)
	}

	duringBuild, cancelDuringBuild := context.WithCancel(context.Background())
	service = application.NewServiceWithClock(repository, func() time.Time {
		cancelDuringBuild()
		return now
	})
	if _, err := service.PreviewReleaseContext(duringBuild, "preview-context"); !errors.Is(err, context.Canceled) {
		t.Errorf("构造预览时取消应返回 context.Canceled，实际为 %v", err)
	}
}

package canceledapprovalpersists_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/httpui"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func TestCanceledApprovalDoesNotPersist(t *testing.T) {
	now := time.Date(2026, 8, 25, 9, 30, 0, 0, time.UTC)
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	aggregate := readyForApproval(t, now)
	_, _, err = repository.Commit(store.CommitRequest{
		PackageID:      aggregate.Package.ID,
		IdempotencyKey: "private-fixture",
		RequestDigest:  "sha256:private-fixture",
		Create:         aggregate,
	})
	if err != nil {
		t.Fatal(err)
	}

	stableService := application.NewServiceWithClock(repository, func() time.Time { return now })
	preview, err := stableService.PreviewRelease(aggregate.Package.ID)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelingService := application.NewServiceWithClock(repository, func() time.Time {
		cancel()
		return now
	})
	server := httpui.New(cancelingService, slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil)))
	body, err := json.Marshal(application.ApproveReleaseCommand{
		WriteMeta: application.WriteMeta{
			ExpectedVersion: aggregate.Package.Version,
			IdempotencyKey:  "cancelled-approval",
			Actor:           "开放负责人",
			Role:            application.RoleReleaseManager,
		},
		PreviewManifestDigest: preview.ManifestDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/packages/approval-case/release/approve", bytes.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if ctx.Err() != context.Canceled {
		t.Fatal("受控 Clock 未取消请求")
	}
	stored, err := repository.Get(aggregate.Package.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Package.Status != domain.StatusApprovalPending || stored.Package.Version != aggregate.Package.Version || stored.Manifest != nil || stored.Credential != nil {
		t.Fatalf("请求已取消但批准仍被持久化：HTTP %d，状态 %s，版本 %d", response.Code, stored.Package.Status, stored.Package.Version)
	}
}

func readyForApproval(t *testing.T, now time.Time) *domain.Aggregate {
	t.Helper()
	aggregate, err := domain.NewAggregate("approval-case", "地方口述史", "P-17", "资料负责人", "公开展示", "整理员", now)
	if err != nil {
		t.Fatal(err)
	}
	consent := domain.ConsentRecord{
		Terms:                 "同意将整理后的访谈内容用于公开展示",
		AllowedUses:           []string{"公开展示"},
		AttributionPreference: "使用参与者代号",
		ConfirmedAt:           now,
		ConfirmedBy:           "参与者 P-17",
	}
	if err := aggregate.ConfirmConsent(consent, "整理员", now); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.AddSegment("segment-1", 1, "可公开的访谈内容", "整理员", now); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.ClassifySegment("segment-1", domain.DecisionPublic, nil, nil, "整理员", now); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.CompleteClassification("整理员", now); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.SubmitReview("整理员", now); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.ReviewSegment("segment-1", domain.VerdictApproved, "", "伦理复核员", now); err != nil {
		t.Fatal(err)
	}
	if aggregate.Package.Status != domain.StatusApprovalPending {
		t.Fatalf("测试夹具未进入待批准状态：%s", aggregate.Package.Status)
	}
	return aggregate
}

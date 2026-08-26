package finalized_idempotency_restart_test

import (
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func TestReleasedPackageRetrySurvivesRestart(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewServiceWithClock(repository, func() time.Time { return now })

	create := application.CreatePackageCommand{
		WriteMeta:       application.WriteMeta{IdempotencyKey: "create-once", Actor: "整理员", Role: application.RoleOrganizer},
		ID:              "restart-replay",
		Topic:           "河运记忆",
		ParticipantCode: "P-17",
		OwnerName:       "资料负责人",
		IntendedScope:   "馆内展览",
	}
	view, replayed, err := service.CreatePackage(create)
	if err != nil || replayed {
		t.Fatalf("首次创建失败: replayed=%v err=%v", replayed, err)
	}
	view, _, err = service.ConfirmConsent(create.ID, application.ConfirmConsentCommand{
		WriteMeta:             writeMeta(view.Package.Version, "consent", application.RoleOrganizer, "整理员"),
		Terms:                 "参与者同意用于馆内展览",
		AllowedUses:           []string{"馆内展览"},
		AttributionPreference: "使用代号",
		ConfirmedAt:           now,
		ConfirmedBy:           "参与者 P-17",
	})
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = service.AddSegment(create.ID, application.AddSegmentCommand{
		WriteMeta: writeMeta(view.Package.Version, "segment", application.RoleOrganizer, "整理员"),
		ID:        "s1", Sequence: 1, SourceText: "码头清晨开始装卸。",
	})
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = service.ClassifySegment(create.ID, application.ClassifySegmentCommand{
		WriteMeta: writeMeta(view.Package.Version, "classify", application.RoleOrganizer, "整理员"),
		SegmentID: "s1", Decision: domain.DecisionPublic,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = service.CompleteClassification(create.ID, application.CompleteClassificationCommand{
		WriteMeta: writeMeta(view.Package.Version, "classification-complete", application.RoleOrganizer, "整理员"),
	})
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = service.SubmitReview(create.ID, application.SubmitReviewCommand{
		WriteMeta: writeMeta(view.Package.Version, "submit-review", application.RoleOrganizer, "整理员"),
	})
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = service.ReviewSegment(create.ID, application.ReviewSegmentCommand{
		WriteMeta: writeMeta(view.Package.Version, "approve-segment", application.RoleReviewer, "伦理复核员"),
		SegmentID: "s1", Verdict: domain.VerdictApproved,
	})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewRelease(create.ID)
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = service.ApproveRelease(create.ID, application.ApproveReleaseCommand{
		WriteMeta:             writeMeta(view.Package.Version, "release", application.RoleReleaseManager, "开放负责人"),
		PreviewManifestDigest: preview.ManifestDigest,
	})
	if err != nil || view.Package.Status != domain.StatusReleased {
		t.Fatalf("签发失败: status=%s err=%v", view.Package.Status, err)
	}
	_, replayed, err = service.CreatePackage(create)
	if err != nil || !replayed {
		t.Fatalf("关闭前相同创建请求应命中幂等记录: replayed=%v err=%v", replayed, err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	replayedView, replayed, err := application.NewServiceWithClock(reopened, func() time.Time { return now }).CreatePackage(create)
	if err != nil || !replayed {
		t.Fatalf("重启后相同创建请求未返回已提交结果: replayed=%v err=%v", replayed, err)
	}
	if replayedView.Package.Version != 1 || replayedView.Package.Status != domain.StatusDraft {
		t.Fatalf("重放响应被当前冻结状态替换: version=%d status=%s", replayedView.Package.Version, replayedView.Package.Status)
	}
}

func writeMeta(version uint64, key, role, actor string) application.WriteMeta {
	return application.WriteMeta{ExpectedVersion: version, IdempotencyKey: key, Role: role, Actor: actor}
}

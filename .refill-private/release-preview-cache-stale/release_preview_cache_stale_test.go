package release_preview_cache_stale_test

import (
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func TestReleasePreviewCacheTracksFrozenVersion(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	reader := application.NewServiceWithClock(repository, clock)
	writer := application.NewServiceWithClock(repository, clock)

	view, _, err := writer.CreatePackage(application.CreatePackageCommand{
		WriteMeta:       application.WriteMeta{IdempotencyKey: "create", Actor: "整理员", Role: application.RoleOrganizer},
		ID:              "preview-cache-package",
		Topic:           "老街口述史",
		ParticipantCode: "P-17",
		OwnerName:       "资料负责人",
		IntendedScope:   "公开展示",
	})
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = writer.ConfirmConsent(view.Package.ID, application.ConfirmConsentCommand{
		WriteMeta:             writeMeta(view, "consent", "整理员", application.RoleOrganizer),
		Terms:                 "同意脱敏后公开",
		AllowedUses:           []string{"公开展示"},
		AttributionPreference: "使用代号",
		ConfirmedAt:           now,
		ConfirmedBy:           "参与者",
	})
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = writer.AddSegment(view.Package.ID, application.AddSegmentCommand{
		WriteMeta: writeMeta(view, "segment", "整理员", application.RoleOrganizer),
		ID:        "s1", Sequence: 1, SourceText: "这段内容可以公开。",
	})
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = writer.ClassifySegment(view.Package.ID, application.ClassifySegmentCommand{
		WriteMeta: writeMeta(view, "classify", "整理员", application.RoleOrganizer),
		SegmentID: "s1", Decision: domain.DecisionPublic,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = writer.CompleteClassification(view.Package.ID, application.CompleteClassificationCommand{
		WriteMeta: writeMeta(view, "classification-complete", "整理员", application.RoleOrganizer),
	})
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = writer.SubmitReview(view.Package.ID, application.SubmitReviewCommand{
		WriteMeta: writeMeta(view, "submit-review", "整理员", application.RoleOrganizer),
	})
	if err != nil {
		t.Fatal(err)
	}
	view, _, err = writer.ReviewSegment(view.Package.ID, application.ReviewSegmentCommand{
		WriteMeta: writeMeta(view, "approve-segment", "伦理复核员", application.RoleReviewer),
		SegmentID: "s1", Verdict: domain.VerdictApproved,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Package.Status != domain.StatusApprovalPending {
		t.Fatalf("准备数据状态 = %s", view.Package.Status)
	}

	preview, err := reader.PreviewRelease(view.Package.ID)
	if err != nil {
		t.Fatal(err)
	}
	if preview.PackageVersion != view.Package.Version || preview.ManifestDigest == "" {
		t.Fatalf("预览未反映仓储版本 %d: %#v", view.Package.Version, preview)
	}

	view, _, err = writer.ApproveRelease(view.Package.ID, application.ApproveReleaseCommand{
		WriteMeta:             writeMeta(view, "release", "开放负责人", application.RoleReleaseManager),
		PreviewManifestDigest: preview.ManifestDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Package.Status != domain.StatusReleased {
		t.Fatalf("批准后状态 = %s", view.Package.Status)
	}

	frozenPreview, err := reader.PreviewRelease(view.Package.ID)
	if err != nil {
		t.Fatal(err)
	}
	if frozenPreview.PackageVersion != view.Package.Version || !frozenPreview.Frozen {
		t.Fatalf("仓储已冻结到版本 %d，但预览仍复用版本 %d（frozen=%v）", view.Package.Version, frozenPreview.PackageVersion, frozenPreview.Frozen)
	}
}

func writeMeta(view application.PackageView, key, actor, role string) application.WriteMeta {
	return application.WriteMeta{
		ExpectedVersion: view.Package.Version,
		IdempotencyKey:  key,
		Actor:           actor,
		Role:            role,
	}
}

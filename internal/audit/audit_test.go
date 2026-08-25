package audit

import (
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

func TestManifestDigestIsDeterministicAndCredentialVerifies(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	manifest := domain.FrozenManifest{PackageID: "p", Topic: "主题", ParticipantCode: "P", IntendedScope: "公开展示", TermsDigest: "sha256:terms", ConsentSummary: "摘要", FrozenBy: "负责人", FrozenAt: now, Segments: []domain.FrozenSegment{{ID: "s", Sequence: 1, PublicText: "公开文本", RiskTags: []string{"privacy"}}}}
	first, err := ManifestDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Digest = first
	second, err := ManifestDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("摘要不稳定")
	}
	credential, err := IssueCredential(manifest, 42, "负责人", now)
	if err != nil {
		t.Fatal(err)
	}
	result := VerifyCredential(&manifest, &credential)
	if !result.Valid {
		t.Fatalf("校验失败: %s", result.Message)
	}
	manifest.Segments[0].PublicText = "被篡改"
	if VerifyCredential(&manifest, &credential).Valid {
		t.Fatal("篡改后仍通过校验")
	}
}

func TestManifestPreviewIsReadOnlyAndMatchesFrozenContent(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	aggregate, err := domain.NewAggregate("p2", "主题", "P2", "负责人", "公开展示", "整理员", now)
	if err != nil {
		t.Fatal(err)
	}
	consent := domain.ConsentRecord{Terms: "同意公开", AllowedUses: []string{"公开展示"}, AttributionPreference: "匿名", ConfirmedAt: now, ConfirmedBy: "参与者"}
	if err := aggregate.ConfirmConsent(consent, "整理员", now); err != nil {
		t.Fatal(err)
	}
	_ = aggregate.AddSegments([]domain.SegmentInput{{ID: "omit", Sequence: 30, SourceText: "不公开"}, {ID: "public", Sequence: 10, SourceText: "公开文本"}}, "整理员", now)
	_ = aggregate.ClassifySegments([]domain.ClassificationInput{{SegmentID: "public", Decision: domain.DecisionPublic}, {SegmentID: "omit", Decision: domain.DecisionOmit}}, "整理员", now)
	aggregate.Package.Status = domain.StatusApprovalPending
	version := aggregate.Package.Version
	events := len(aggregate.Events)
	first, err := BuildManifestPreview(aggregate, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildManifestPreview(aggregate, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if first.ManifestDigest != second.ManifestDigest || first.PublicCount != 1 || first.ExcludedCount != 1 || first.Segments[0].ID != "public" || first.ExcludedSegmentIDs[0] != "omit" || aggregate.Package.Version != version || len(aggregate.Events) != events {
		t.Fatalf("预览内容或只读语义错误: %#v", first)
	}
	manifest, err := BuildManifest(aggregate, "开放负责人", now.Add(3*time.Minute))
	if err != nil || manifest.Digest != first.ManifestDigest {
		t.Fatalf("正式清单与预览摘要不一致: %v", err)
	}
	if err := aggregate.SetFrozen(manifest, "开放负责人", now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	frozen, err := BuildManifestPreview(aggregate, now.Add(4*time.Minute))
	if err != nil || !frozen.Frozen || !frozen.Consistent || frozen.ManifestDigest != first.ManifestDigest {
		t.Fatalf("冻结后预览错误: %#v, %v", frozen, err)
	}
}

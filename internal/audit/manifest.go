package audit

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

func BuildManifest(aggregate *domain.Aggregate, approver string, now time.Time) (domain.FrozenManifest, error) {
	if aggregate == nil || aggregate.Consent == nil {
		return domain.FrozenManifest{}, domain.ErrIncomplete
	}
	if aggregate.Package.Status != domain.StatusApprovalPending {
		return domain.FrozenManifest{}, domain.ErrInvalidTransition
	}
	segments := make([]domain.FrozenSegment, 0, len(aggregate.Segments))
	ordered := append([]domain.TranscriptSegment(nil), aggregate.Segments...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Sequence < ordered[j].Sequence })
	for _, segment := range ordered {
		if segment.Decision == domain.DecisionOmit {
			continue
		}
		if domain.NormalizeText(segment.PublicText) == "" {
			return domain.FrozenManifest{}, domain.ErrIncomplete
		}
		segments = append(segments, domain.FrozenSegment{ID: segment.ID, Sequence: segment.Sequence, PublicText: strings.TrimSpace(segment.PublicText), RiskTags: normalizeTags(segment.RiskTags)})
	}
	if len(segments) == 0 {
		return domain.FrozenManifest{}, domain.ErrIncomplete
	}
	consentSummary := fmt.Sprintf("允许用途：%s；署名偏好：%s", strings.Join(aggregate.Consent.AllowedUses, "、"), aggregate.Consent.AttributionPreference)
	manifest := domain.FrozenManifest{PackageID: aggregate.Package.ID, Topic: aggregate.Package.Topic, ParticipantCode: aggregate.Package.ParticipantCode, IntendedScope: aggregate.Package.IntendedScope, TermsDigest: aggregate.Consent.TermsDigest, ConsentSummary: consentSummary, Segments: segments, FrozenBy: domain.NormalizeText(approver), FrozenAt: now.UTC()}
	digest, err := ManifestDigest(manifest)
	if err != nil {
		return domain.FrozenManifest{}, err
	}
	manifest.Digest = digest
	return manifest, nil
}

func ManifestDigest(manifest domain.FrozenManifest) (string, error) {
	copyValue := manifest
	copyValue.Digest = ""
	// 冻结人和冻结时间是签发元数据；内容摘要只绑定负责人预览过的
	// 同意边界、顺序和公开文本，因而预览与稍后正式冻结可使用同一摘要。
	copyValue.FrozenBy = ""
	copyValue.FrozenAt = time.Time{}
	return JSONDigest(copyValue)
}

type ManifestPreview struct {
	PackageID          string                 `json:"packageID"`
	PackageVersion     uint64                 `json:"packageVersion"`
	ConsentSummary     string                 `json:"consentSummary"`
	Segments           []domain.FrozenSegment `json:"segments"`
	ExcludedSegmentIDs []string               `json:"excludedSegmentIDs"`
	PublicCount        int                    `json:"publicCount"`
	ExcludedCount      int                    `json:"excludedCount"`
	ManifestDigest     string                 `json:"manifestDigest"`
	TermsDigest        string                 `json:"termsDigest"`
	GeneratedAt        time.Time              `json:"generatedAt"`
	Frozen             bool                   `json:"frozen"`
	Consistent         bool                   `json:"consistent"`
}

func BuildManifestPreview(aggregate *domain.Aggregate, now time.Time) (ManifestPreview, error) {
	if aggregate == nil {
		return ManifestPreview{}, domain.ErrNotFound
	}
	var manifest domain.FrozenManifest
	var err error
	frozen := aggregate.Package.Status.IsFrozen()
	if frozen {
		if aggregate.Manifest == nil {
			return ManifestPreview{}, domain.ErrIntegrity
		}
		manifest = *aggregate.Manifest
	} else {
		manifest, err = BuildManifest(aggregate, "", now)
		if err != nil {
			return ManifestPreview{}, err
		}
	}
	digest, err := ManifestDigest(manifest)
	if err != nil {
		return ManifestPreview{}, err
	}
	excluded := make([]domain.TranscriptSegment, 0)
	for _, segment := range aggregate.Segments {
		if segment.Decision == domain.DecisionOmit {
			excluded = append(excluded, segment)
		}
	}
	sort.Slice(excluded, func(i, j int) bool { return excluded[i].Sequence < excluded[j].Sequence })
	excludedIDs := make([]string, len(excluded))
	for i := range excluded {
		excludedIDs[i] = excluded[i].ID
	}
	return ManifestPreview{PackageID: aggregate.Package.ID, PackageVersion: aggregate.Package.Version, ConsentSummary: manifest.ConsentSummary, Segments: append([]domain.FrozenSegment(nil), manifest.Segments...), ExcludedSegmentIDs: excludedIDs, PublicCount: len(manifest.Segments), ExcludedCount: len(excludedIDs), ManifestDigest: digest, TermsDigest: manifest.TermsDigest, GeneratedAt: now.UTC(), Frozen: frozen, Consistent: manifest.Digest == digest}, nil
}

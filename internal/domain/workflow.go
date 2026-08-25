package domain

import (
	"fmt"
	"strings"
	"time"
)

func summarize(text string) string {
	runes := []rune(NormalizeText(text))
	if len(runes) > 48 {
		return string(runes[:48]) + "…"
	}
	return string(runes)
}

func (a *Aggregate) ReviseSegment(id, publicText, reason, actor string, now time.Time) error {
	if err := a.assertMutable(); err != nil {
		return err
	}
	if a.Package.Status != StatusRedactionPending && a.Package.Status != StatusRemediation {
		return ErrInvalidTransition
	}
	segment, err := a.findSegment(id)
	if err != nil {
		return err
	}
	if segment.Decision != DecisionRestricted {
		return invalid("segmentID", "not_restricted", "仅受限片段需要脱敏修订")
	}
	publicText = strings.TrimSpace(publicText)
	reason = NormalizeText(reason)
	if publicText == "" || reason == "" {
		return invalid("revision", "required", "公开文本和替换理由均不能为空")
	}
	if publicText == strings.TrimSpace(segment.SourceText) {
		return invalid("publicText", "unchanged", "受限片段的公开文本必须与原文不同")
	}
	previousText := segment.SourceText
	if segment.Revision > 0 && NormalizeText(segment.PublicText) != "" {
		previousText = segment.PublicText
	}
	segment.PublicText = publicText
	segment.Revision++
	segment.RevisionReason = reason
	segment.BeforeSummary = summarize(previousText)
	segment.AfterSummary = summarize(publicText)
	resolvedFindingIDs := make([]string, 0)
	for i := range a.Findings {
		finding := &a.Findings[i]
		if finding.SegmentID == id && finding.Verdict == VerdictReturned && finding.ResolvedByRevision == nil && finding.RevisionAtReturn < segment.Revision {
			revision := segment.Revision
			finding.ResolvedByRevision = &revision
			resolvedFindingIDs = append(resolvedFindingIDs, finding.ID)
		}
	}
	a.Revisions = append(a.Revisions, SegmentRevision{PackageID: a.Package.ID, SegmentID: id, Revision: segment.Revision, PublicText: publicText, Reason: reason, BeforeSummary: segment.BeforeSummary, AfterSummary: segment.AfterSummary, Actor: NormalizeText(actor), At: now.UTC(), ResolvedFindingIDs: resolvedFindingIDs})
	a.changed(now)
	a.record(EventRevisionSubmitted, actor, now, map[string]any{"segmentID": id, "revision": segment.Revision, "reason": reason, "before": segment.BeforeSummary, "after": segment.AfterSummary, "resolvedFindingCount": len(resolvedFindingIDs)})
	return nil
}

func (a *Aggregate) SubmitReview(actor string, now time.Time) error {
	if a.Package.Status != StatusRedactionPending && a.Package.Status != StatusRemediation {
		return ErrInvalidTransition
	}
	for _, segment := range a.Segments {
		if segment.Decision == DecisionRestricted && (segment.Revision == 0 || NormalizeText(segment.PublicText) == "") {
			return invalid("segments", "missing_revision", "所有受限片段都必须完成脱敏修订")
		}
	}
	if a.Package.Status == StatusRemediation {
		for _, finding := range a.Findings {
			if finding.Verdict == VerdictReturned && finding.ResolvedByRevision == nil {
				return invalid("findings", "unresolved", "所有退回整改项必须以新修订闭环")
			}
		}
	}
	a.Round++
	a.Package.Status = StatusReviewPending
	a.changed(now)
	a.record(EventReviewSubmitted, actor, now, map[string]any{"round": a.Round})
	return nil
}

func (a *Aggregate) ReviewSegment(segmentID string, verdict ReviewVerdict, reason, reviewer string, now time.Time) error {
	if a.Package.Status != StatusReviewPending {
		return ErrInvalidTransition
	}
	if verdict != VerdictApproved && verdict != VerdictReturned {
		return invalid("verdict", "invalid", "复核结论必须为通过或退回")
	}
	if NormalizeText(reviewer) == "" {
		return invalid("reviewer", "required", "复核员不能为空")
	}
	segment, err := a.findSegment(segmentID)
	if err != nil {
		return err
	}
	if segment.Decision == DecisionOmit {
		return invalid("segmentID", "omitted", "不公开片段无需进入公开文本复核")
	}
	if verdict == VerdictReturned && NormalizeText(reason) == "" {
		return invalid("reason", "required", "退回时必须填写明确整改原因")
	}
	for _, finding := range a.Findings {
		if finding.SegmentID == segmentID && finding.Round == a.Round {
			return invalid("segmentID", "already_reviewed", "本轮已复核该片段")
		}
	}
	finding := ReviewFinding{ID: fmt.Sprintf("%s-r%d-f%03d", a.Package.ID, a.Round, len(a.Findings)+1), PackageID: a.Package.ID, SegmentID: segmentID, Round: a.Round, Verdict: verdict, Reason: NormalizeText(reason), Reviewer: NormalizeText(reviewer), ReviewedAt: now.UTC()}
	if verdict == VerdictReturned {
		finding.RevisionAtReturn = segment.Revision
	}
	a.Findings = append(a.Findings, finding)
	a.changed(now)
	if verdict == VerdictReturned {
		a.Package.Status = StatusRemediation
		a.record(EventReviewReturned, reviewer, now, map[string]any{"segmentID": segmentID, "round": a.Round, "reason": finding.Reason})
		return nil
	}
	a.record(EventReviewApproved, reviewer, now, map[string]any{"segmentID": segmentID, "round": a.Round})
	if a.allCurrentReviewApproved() {
		a.Package.Status = StatusApprovalPending
	}
	return nil
}

func (a *Aggregate) allCurrentReviewApproved() bool {
	required := 0
	approved := 0
	for _, segment := range a.Segments {
		if segment.Decision != DecisionOmit {
			required++
		}
	}
	for _, finding := range a.Findings {
		if finding.Round == a.Round && finding.Verdict == VerdictApproved {
			approved++
		}
	}
	return required > 0 && approved == required
}

func (a *Aggregate) SetFrozen(manifest FrozenManifest, approver string, now time.Time) error {
	if a.Package.Status != StatusApprovalPending {
		return ErrInvalidTransition
	}
	if a.Consent == nil || manifest.TermsDigest != a.Consent.TermsDigest || manifest.PackageID != a.Package.ID || manifest.Digest == "" {
		return ErrIntegrity
	}
	a.Manifest = &manifest
	a.Package.Status = StatusFrozen
	a.changed(now)
	a.record(EventManifestFrozen, approver, now, map[string]any{"manifestDigest": manifest.Digest, "segmentCount": len(manifest.Segments)})
	return nil
}

func (a *Aggregate) SetCredential(credential ReleaseCredential, actor string, now time.Time) error {
	if a.Package.Status != StatusFrozen || a.Manifest == nil {
		return ErrInvalidTransition
	}
	if credential.PackageID != a.Package.ID || credential.ManifestDigest != a.Manifest.Digest || credential.TermsDigest != a.Manifest.TermsDigest || credential.Serial == 0 {
		return ErrIntegrity
	}
	a.Credential = &credential
	a.Package.Status = StatusReleased
	a.changed(now)
	a.record(EventCredentialIssued, actor, now, map[string]any{"serial": credential.Serial, "manifestDigest": credential.ManifestDigest})
	return nil
}

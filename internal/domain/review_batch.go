package domain

import (
	"fmt"
	"sort"
	"time"
)

type ReviewInput struct {
	SegmentID string        `json:"segmentID"`
	Verdict   ReviewVerdict `json:"verdict"`
	Reason    string        `json:"reason"`
}

type ReviewProgress struct {
	Round               int      `json:"round"`
	Required            int      `json:"required"`
	Decided             int      `json:"decided"`
	Remaining           int      `json:"remaining"`
	Returned            int      `json:"returned"`
	RemainingSegmentIDs []string `json:"remainingSegmentIDs"`
}

func (a *Aggregate) ReviewSegments(round int, inputs []ReviewInput, reviewer string, now time.Time) error {
	if a.Package.Status != StatusReviewPending {
		return ErrInvalidTransition
	}
	if round != a.Round {
		return invalid("reviewRound", "stale_review_round", "复核轮次已变化，请刷新后重试")
	}
	if NormalizeText(reviewer) == "" {
		return invalid("reviewer", "required", "复核员不能为空")
	}
	if len(inputs) == 0 {
		return invalid("items", "empty_batch", "批量裁决不能为空")
	}
	segmentByID := make(map[string]*TranscriptSegment, len(a.Segments))
	for i := range a.Segments {
		segmentByID[a.Segments[i].ID] = &a.Segments[i]
	}
	already := make(map[string]bool)
	for _, finding := range a.Findings {
		if finding.Round == a.Round {
			already[finding.SegmentID] = true
		}
	}
	seen := make(map[string]bool, len(inputs))
	normalized := make([]ReviewInput, len(inputs))
	for i, input := range inputs {
		line := i + 1
		input.SegmentID = NormalizeText(input.SegmentID)
		input.Reason = NormalizeText(input.Reason)
		if input.SegmentID == "" {
			return batchError(line, "", "segmentID", "required", "片段标识不能为空")
		}
		if seen[input.SegmentID] {
			return batchError(line, input.SegmentID, "segmentID", "duplicate_target", "同一批次不能重复裁决片段")
		}
		seen[input.SegmentID] = true
		segment := segmentByID[input.SegmentID]
		if segment == nil {
			return batchError(line, input.SegmentID, "segmentID", "not_found", "片段不属于当前访谈包")
		}
		if segment.Decision == DecisionOmit {
			return batchError(line, input.SegmentID, "segmentID", "omitted", "不公开片段无需进入公开文本复核")
		}
		if already[input.SegmentID] {
			return batchError(line, input.SegmentID, "segmentID", "already_reviewed", "本轮已裁决该片段")
		}
		if input.Verdict != VerdictApproved && input.Verdict != VerdictReturned {
			return batchError(line, input.SegmentID, "verdict", "invalid", "复核结论必须为通过或退回")
		}
		if input.Verdict == VerdictReturned && input.Reason == "" {
			return batchError(line, input.SegmentID, "reason", "required", "退回时必须填写独立明确原因")
		}
		normalized[i] = input
	}
	reviewer = NormalizeText(reviewer)
	returned := false
	newFindings := make([]ReviewFinding, len(normalized))
	for i, input := range normalized {
		segment := segmentByID[input.SegmentID]
		finding := ReviewFinding{ID: fmt.Sprintf("%s-r%d-f%03d", a.Package.ID, a.Round, len(a.Findings)+i+1), PackageID: a.Package.ID, SegmentID: input.SegmentID, Round: a.Round, Verdict: input.Verdict, Reason: input.Reason, Reviewer: reviewer, ReviewedAt: now.UTC()}
		if input.Verdict == VerdictReturned {
			finding.RevisionAtReturn = segment.Revision
			returned = true
		}
		newFindings[i] = finding
	}
	a.Findings = append(a.Findings, newFindings...)
	a.changed(now)
	for _, finding := range newFindings {
		details := map[string]any{"segmentID": finding.SegmentID, "round": a.Round}
		if finding.Verdict == VerdictReturned {
			details["reason"] = finding.Reason
			a.record(EventReviewReturned, reviewer, now, details)
		} else {
			a.record(EventReviewApproved, reviewer, now, details)
		}
	}
	if returned {
		a.Package.Status = StatusRemediation
	} else if a.allCurrentReviewApproved() {
		a.Package.Status = StatusApprovalPending
	}
	return nil
}

func (a *Aggregate) CurrentReviewProgress() ReviewProgress {
	progress := ReviewProgress{Round: a.Round, RemainingSegmentIDs: []string{}}
	decisions := make(map[string]ReviewVerdict)
	for _, finding := range a.Findings {
		if finding.Round == a.Round {
			decisions[finding.SegmentID] = finding.Verdict
			if finding.Verdict == VerdictReturned {
				progress.Returned++
			}
		}
	}
	ordered := append([]TranscriptSegment(nil), a.Segments...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Sequence < ordered[j].Sequence })
	for _, segment := range ordered {
		if segment.Decision == DecisionOmit {
			continue
		}
		progress.Required++
		if _, ok := decisions[segment.ID]; ok {
			progress.Decided++
		} else {
			progress.RemainingSegmentIDs = append(progress.RemainingSegmentIDs, segment.ID)
		}
	}
	progress.Remaining = progress.Required - progress.Decided
	return progress
}

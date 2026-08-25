package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type SegmentInput struct {
	ID         string `json:"id"`
	Sequence   int    `json:"sequence"`
	SourceText string `json:"sourceText"`
}

type ClassificationInput struct {
	SegmentID        string          `json:"segmentID"`
	Decision         SegmentDecision `json:"decision"`
	RiskTags         []string        `json:"riskTags"`
	RestrictionUntil *time.Time      `json:"restrictionUntil,omitempty"`
}

type ClassificationProgress struct {
	Total                    int            `json:"total"`
	Pending                  int            `json:"pending"`
	Public                   int            `json:"public"`
	Restricted               int            `json:"restricted"`
	NotPublic                int            `json:"notPublic"`
	RiskTagCounts            map[string]int `json:"riskTagCounts"`
	WithRestrictionUntil     int            `json:"withRestrictionUntil"`
	EarliestRestrictionUntil *time.Time     `json:"earliestRestrictionUntil,omitempty"`
	PendingSegmentIDs        []string       `json:"pendingSegmentIDs"`
}

func batchError(line int, segmentID, field, code, message string) error {
	label := fmt.Sprintf("第 %d 行", line)
	if segmentID != "" {
		label += "片段 " + segmentID
	}
	return &RuleError{Field: fmt.Sprintf("items[%d].%s", line-1, field), Code: code, Message: label + "：" + message, Line: line, SegmentID: segmentID}
}

func (a *Aggregate) AddSegments(inputs []SegmentInput, actor string, now time.Time) error {
	if err := a.assertMutable(); err != nil {
		return err
	}
	if a.Package.Status != StatusConsentConfirmed && a.Package.Status != StatusClassifying {
		return ErrInvalidTransition
	}
	if len(inputs) == 0 {
		return invalid("items", "empty_batch", "批量片段不能为空")
	}
	existingIDs := make(map[string]bool, len(a.Segments)+len(inputs))
	existingSequences := make(map[int]bool, len(a.Segments)+len(inputs))
	for _, segment := range a.Segments {
		existingIDs[segment.ID] = true
		existingSequences[segment.Sequence] = true
	}
	normalized := make([]SegmentInput, len(inputs))
	for i, input := range inputs {
		line := i + 1
		input.ID = NormalizeText(input.ID)
		input.SourceText = strings.TrimSpace(input.SourceText)
		if input.ID == "" {
			return batchError(line, "", "id", "required", "片段标识不能为空")
		}
		if input.Sequence <= 0 {
			return batchError(line, input.ID, "sequence", "positive_required", "顺序必须为正数")
		}
		if input.SourceText == "" {
			return batchError(line, input.ID, "sourceText", "required", "原文不能为空")
		}
		if existingIDs[input.ID] {
			return batchError(line, input.ID, "id", "duplicate", "片段标识与批次内或现有片段重复")
		}
		if existingSequences[input.Sequence] {
			return batchError(line, input.ID, "sequence", "duplicate", fmt.Sprintf("顺序 %d 与批次内或现有片段冲突", input.Sequence))
		}
		existingIDs[input.ID] = true
		existingSequences[input.Sequence] = true
		normalized[i] = input
	}
	for _, input := range normalized {
		a.Segments = append(a.Segments, TranscriptSegment{ID: input.ID, PackageID: a.Package.ID, Sequence: input.Sequence, SourceText: input.SourceText, Decision: DecisionPending})
	}
	sort.Slice(a.Segments, func(i, j int) bool { return a.Segments[i].Sequence < a.Segments[j].Sequence })
	a.Package.Status = StatusClassifying
	a.changed(now)
	a.record(EventSegmentsAdded, actor, now, map[string]any{"addedCount": len(normalized), "segmentIDs": segmentInputIDs(normalized)})
	return nil
}

func segmentInputIDs(inputs []SegmentInput) []string {
	ids := make([]string, len(inputs))
	for i := range inputs {
		ids[i] = inputs[i].ID
	}
	return ids
}

type preparedClassification struct {
	index    int
	decision SegmentDecision
	tags     []string
	until    *time.Time
}

func validateClassification(decision SegmentDecision, tags []string, until *time.Time) ([]string, *time.Time, error) {
	if !decision.ValidFinal() {
		return nil, nil, invalid("decision", "invalid", "片段判定必须为公开、受限或不公开")
	}
	normalized, err := ValidateRiskTags(tags, until)
	if err != nil {
		return nil, nil, err
	}
	if decision == DecisionPublic && len(normalized) > 0 {
		return nil, nil, invalid("riskTags", "public_with_risk", "公开片段不能保留敏感标签")
	}
	if decision == DecisionRestricted && len(normalized) == 0 {
		return nil, nil, invalid("riskTags", "restricted_without_risk", "受限片段至少需要一个敏感标签")
	}
	if until != nil {
		value := until.UTC()
		until = &value
	}
	return normalized, until, nil
}

func (a *Aggregate) ClassifySegments(inputs []ClassificationInput, actor string, now time.Time) error {
	if err := a.assertMutable(); err != nil {
		return err
	}
	if a.Package.Status != StatusClassifying {
		return ErrInvalidTransition
	}
	if len(inputs) == 0 {
		return invalid("items", "empty_batch", "批量判定不能为空")
	}
	indices := make(map[string]int, len(a.Segments))
	for i := range a.Segments {
		indices[a.Segments[i].ID] = i
	}
	seen := make(map[string]bool, len(inputs))
	prepared := make([]preparedClassification, len(inputs))
	for i, input := range inputs {
		line := i + 1
		input.SegmentID = NormalizeText(input.SegmentID)
		if input.SegmentID == "" {
			return batchError(line, "", "segmentID", "required", "片段标识不能为空")
		}
		if seen[input.SegmentID] {
			return batchError(line, input.SegmentID, "segmentID", "duplicate_target", "同一批次不能重复判定片段")
		}
		seen[input.SegmentID] = true
		index, ok := indices[input.SegmentID]
		if !ok {
			return batchError(line, input.SegmentID, "segmentID", "not_found", "片段不属于当前访谈包")
		}
		if a.Segments[index].Decision != DecisionPending {
			return batchError(line, input.SegmentID, "segmentID", "already_classified", "仅尚未判定的片段可批量提交")
		}
		tags, until, err := validateClassification(input.Decision, input.RiskTags, input.RestrictionUntil)
		if err != nil {
			if rule, ok := err.(*RuleError); ok {
				return batchError(line, input.SegmentID, rule.Field, rule.Code, rule.Message)
			}
			return err
		}
		prepared[i] = preparedClassification{index: index, decision: input.Decision, tags: tags, until: until}
	}
	segmentIDs := make([]string, len(inputs))
	for i, item := range prepared {
		segment := &a.Segments[item.index]
		segment.Decision = item.decision
		segment.RiskTags = item.tags
		segment.RestrictionUntil = item.until
		if item.decision == DecisionPublic {
			segment.PublicText = segment.SourceText
			segment.Revision = 1
		}
		if item.decision == DecisionOmit {
			segment.PublicText = ""
			segment.Revision = 1
		}
		segmentIDs[i] = segment.ID
	}
	a.changed(now)
	a.record(EventSegmentsClassified, actor, now, map[string]any{"classifiedCount": len(prepared), "segmentIDs": segmentIDs})
	return nil
}

func (a *Aggregate) ClassificationProgress() ClassificationProgress {
	progress := ClassificationProgress{Total: len(a.Segments), RiskTagCounts: map[string]int{}, PendingSegmentIDs: []string{}}
	for _, segment := range a.Segments {
		switch segment.Decision {
		case DecisionPublic:
			progress.Public++
		case DecisionRestricted:
			progress.Restricted++
		case DecisionOmit:
			progress.NotPublic++
		default:
			progress.Pending++
			progress.PendingSegmentIDs = append(progress.PendingSegmentIDs, segment.ID)
		}
		for _, tag := range segment.RiskTags {
			progress.RiskTagCounts[tag]++
		}
		if segment.RestrictionUntil != nil {
			progress.WithRestrictionUntil++
			if progress.EarliestRestrictionUntil == nil || segment.RestrictionUntil.Before(*progress.EarliestRestrictionUntil) {
				value := segment.RestrictionUntil.UTC()
				progress.EarliestRestrictionUntil = &value
			}
		}
	}
	return progress
}

package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Aggregate struct {
	Package    InterviewPackage    `json:"package"`
	Consent    *ConsentRecord      `json:"consent,omitempty"`
	Segments   []TranscriptSegment `json:"segments"`
	Revisions  []SegmentRevision   `json:"revisions"`
	Findings   []ReviewFinding     `json:"findings"`
	Manifest   *FrozenManifest     `json:"manifest,omitempty"`
	Credential *ReleaseCredential  `json:"credential,omitempty"`
	Events     []BusinessEvent     `json:"events"`
	Round      int                 `json:"round"`
}

func NewAggregate(id, topic, participantCode, owner, scope, actor string, now time.Time) (*Aggregate, error) {
	if NormalizeText(id) == "" || NormalizeText(topic) == "" || NormalizeText(participantCode) == "" || NormalizeText(owner) == "" || NormalizeText(scope) == "" {
		return nil, invalid("package", "required", "主题、参与者代号、公开范围和资料负责人均为必填项")
	}
	p := InterviewPackage{ID: id, Topic: NormalizeText(topic), ParticipantCode: NormalizeText(participantCode), OwnerName: NormalizeText(owner), IntendedScope: NormalizeText(scope), Status: StatusDraft, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
	a := &Aggregate{Package: p, Segments: []TranscriptSegment{}, Revisions: []SegmentRevision{}, Findings: []ReviewFinding{}, Events: []BusinessEvent{}}
	a.record(EventPackageCreated, actor, now, map[string]any{"topic": p.Topic, "participantCode": p.ParticipantCode})
	return a, nil
}

func (a *Aggregate) record(kind EventType, actor string, now time.Time, details map[string]any) {
	a.Events = append(a.Events, BusinessEvent{ID: fmt.Sprintf("%s-e%04d", a.Package.ID, len(a.Events)+1), PackageID: a.Package.ID, Type: kind, Actor: NormalizeText(actor), At: now.UTC(), Version: a.Package.Version, Details: details})
}

func (a *Aggregate) changed(now time.Time) {
	a.Package.Version++
	a.Package.UpdatedAt = now.UTC()
}

func (a *Aggregate) assertMutable() error {
	if a.Package.Status.IsFrozen() {
		return ErrFrozen
	}
	return nil
}

func (a *Aggregate) ConfirmConsent(c ConsentRecord, actor string, now time.Time) error {
	if err := a.assertMutable(); err != nil {
		return err
	}
	if a.Package.Status != StatusDraft {
		return ErrInvalidTransition
	}
	c.PackageID = a.Package.ID
	if err := ValidateConsent(c, now); err != nil {
		return err
	}
	if !ContainsUse(c.AllowedUses, a.Package.IntendedScope) {
		return ErrConsentScope
	}
	uses := append([]string(nil), c.AllowedUses...)
	for i := range uses {
		uses[i] = NormalizeText(uses[i])
	}
	sort.Strings(uses)
	c.Terms = NormalizeText(c.Terms)
	c.AllowedUses = uses
	c.AttributionPreference = NormalizeText(c.AttributionPreference)
	c.ConfirmedBy = NormalizeText(c.ConfirmedBy)
	c.ConfirmedAt = c.ConfirmedAt.UTC()
	deadline := ""
	if c.WithdrawalDeadline != nil {
		t := c.WithdrawalDeadline.UTC()
		c.WithdrawalDeadline = &t
		deadline = t.Format(time.RFC3339Nano)
	}
	c.TermsDigest = StableDigest(c.Terms, strings.Join(c.AllowedUses, "\n"), c.AttributionPreference, deadline, c.ConfirmedAt.Format(time.RFC3339Nano), c.ConfirmedBy)
	a.Consent = &c
	a.Package.Status = StatusConsentConfirmed
	a.changed(now)
	a.record(EventConsentConfirmed, actor, now, map[string]any{"termsDigest": c.TermsDigest, "allowedUses": c.AllowedUses})
	return nil
}

func (a *Aggregate) AddSegment(id string, sequence int, text, actor string, now time.Time) error {
	if err := a.assertMutable(); err != nil {
		return err
	}
	if a.Package.Status != StatusConsentConfirmed && a.Package.Status != StatusClassifying {
		return ErrInvalidTransition
	}
	id = NormalizeText(id)
	text = strings.TrimSpace(text)
	if id == "" || text == "" || sequence <= 0 {
		return invalid("segment", "invalid", "片段标识、顺序和原文必须有效")
	}
	for _, segment := range a.Segments {
		if segment.ID == id || segment.Sequence == sequence {
			return invalid("segment", "duplicate", "片段标识或顺序重复")
		}
	}
	a.Segments = append(a.Segments, TranscriptSegment{ID: id, PackageID: a.Package.ID, Sequence: sequence, SourceText: text, Decision: DecisionPending})
	sort.Slice(a.Segments, func(i, j int) bool { return a.Segments[i].Sequence < a.Segments[j].Sequence })
	a.Package.Status = StatusClassifying
	a.changed(now)
	a.record(EventSegmentAdded, actor, now, map[string]any{"segmentID": id, "sequence": sequence})
	return nil
}

func (a *Aggregate) findSegment(id string) (*TranscriptSegment, error) {
	for i := range a.Segments {
		if a.Segments[i].ID == id {
			return &a.Segments[i], nil
		}
	}
	return nil, ErrNotFound
}

func (a *Aggregate) ClassifySegment(id string, decision SegmentDecision, tags []string, until *time.Time, actor string, now time.Time) error {
	if err := a.assertMutable(); err != nil {
		return err
	}
	if a.Package.Status != StatusClassifying {
		return ErrInvalidTransition
	}
	normalized, normalizedUntil, err := validateClassification(decision, tags, until)
	if err != nil {
		return err
	}
	segment, err := a.findSegment(id)
	if err != nil {
		return err
	}
	segment.Decision = decision
	segment.RiskTags = normalized
	segment.RestrictionUntil = normalizedUntil
	if decision == DecisionPublic {
		segment.PublicText = segment.SourceText
		segment.Revision = 1
	}
	if decision == DecisionOmit {
		segment.PublicText = ""
		segment.Revision = 1
	}
	a.changed(now)
	a.record(EventSegmentClassified, actor, now, map[string]any{"segmentID": id, "decision": decision, "riskTags": normalized})
	return nil
}

func (a *Aggregate) CompleteClassification(actor string, now time.Time) error {
	if a.Package.Status != StatusClassifying {
		return ErrInvalidTransition
	}
	if len(a.Segments) == 0 {
		return ErrIncomplete
	}
	pending := make([]string, 0)
	for _, segment := range a.Segments {
		if !segment.Decision.ValidFinal() {
			pending = append(pending, segment.ID)
		}
	}
	if len(pending) > 0 {
		return invalid("segments", "unclassified", "仍有片段未完成敏感性判定："+strings.Join(pending, "、"))
	}
	a.Package.Status = StatusRedactionPending
	a.changed(now)
	a.record(EventClassified, actor, now, map[string]any{"segmentCount": len(a.Segments)})
	return nil
}

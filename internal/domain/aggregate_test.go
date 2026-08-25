package domain

import (
	"errors"
	"testing"
	"time"
)

func newConsentedAggregate(t *testing.T) (*Aggregate, time.Time) {
	t.Helper()
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	aggregate, err := NewAggregate("pkg-1", "老街记忆", "P-01", "整理员", "公开展示", "整理员", now)
	if err != nil {
		t.Fatal(err)
	}
	consent := ConsentRecord{Terms: "同意脱敏后公开", AllowedUses: []string{"公开展示"}, AttributionPreference: "使用代号", ConfirmedAt: now, ConfirmedBy: "参与者"}
	if err := aggregate.ConfirmConsent(consent, "整理员", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	return aggregate, now
}

func TestAggregateFullWorkflowAndFreeze(t *testing.T) {
	aggregate, now := newConsentedAggregate(t)
	if err := aggregate.AddSegment("s1", 1, "可公开内容", "整理员", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.AddSegment("s2", 2, "李某住在老街17号", "整理员", now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.ClassifySegment("s1", DecisionPublic, nil, nil, "整理员", now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.ClassifySegment("s2", DecisionRestricted, []string{"privacy", "sensitive_place"}, nil, "整理员", now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.CompleteClassification("整理员", now.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.SubmitReview("整理员", now.Add(7*time.Minute)); err == nil {
		t.Fatal("缺少脱敏修订时不应允许送审")
	}
	if err := aggregate.ReviseSegment("s2", "某人住在老街一带", "隐去姓名和门牌", "整理员", now.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.SubmitReview("整理员", now.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.ReviewSegment("s1", VerdictApproved, "", "复核员", now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.ReviewSegment("s2", VerdictApproved, "脱敏充分", "复核员", now.Add(11*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if aggregate.Package.Status != StatusApprovalPending {
		t.Fatalf("状态 = %s", aggregate.Package.Status)
	}
}

func TestReturnedFindingRequiresNewRevision(t *testing.T) {
	aggregate, now := newConsentedAggregate(t)
	_ = aggregate.AddSegment("s1", 1, "李某住在老街17号", "整理员", now.Add(time.Minute))
	_ = aggregate.ClassifySegment("s1", DecisionRestricted, []string{"privacy"}, nil, "整理员", now.Add(2*time.Minute))
	_ = aggregate.CompleteClassification("整理员", now.Add(3*time.Minute))
	_ = aggregate.ReviseSegment("s1", "李某住在老街", "隐去门牌", "整理员", now.Add(4*time.Minute))
	_ = aggregate.SubmitReview("整理员", now.Add(5*time.Minute))
	if err := aggregate.ReviewSegment("s1", VerdictReturned, "仍暴露姓名", "复核员", now.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.SubmitReview("整理员", now.Add(7*time.Minute)); err == nil {
		t.Fatal("未修订不应再次送审")
	}
	if err := aggregate.ReviseSegment("s1", "某人住在老街一带", "隐去姓名", "整理员", now.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if aggregate.Findings[0].ResolvedByRevision == nil {
		t.Fatal("整改项未关联解决修订")
	}
	if err := aggregate.SubmitReview("整理员", now.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
}

func TestConsentScopeAndFrozenProtection(t *testing.T) {
	now := time.Now().UTC()
	aggregate, _ := NewAggregate("p", "主题", "代号", "负责人", "商业广告", "整理员", now)
	err := aggregate.ConfirmConsent(ConsentRecord{Terms: "仅供学术研究", AllowedUses: []string{"学术研究"}, AttributionPreference: "匿名", ConfirmedAt: now, ConfirmedBy: "参与者"}, "整理员", now)
	if !errors.Is(err, ErrConsentScope) {
		t.Fatalf("期望同意范围错误，得到 %v", err)
	}
	aggregate.Package.Status = StatusFrozen
	if err := aggregate.AddSegment("s", 1, "内容", "整理员", now); !errors.Is(err, ErrFrozen) {
		t.Fatalf("冻结保护失效: %v", err)
	}
}

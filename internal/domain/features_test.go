package domain

import (
	"errors"
	"testing"
	"time"
)

func TestAddSegmentsIsOrderedAndAtomic(t *testing.T) {
	aggregate, now := newConsentedAggregate(t)
	if err := aggregate.AddSegment("existing", 20, "已有原文", "整理员", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	version := aggregate.Package.Version
	eventCount := len(aggregate.Events)
	err := aggregate.AddSegments([]SegmentInput{{ID: "s10", Sequence: 10, SourceText: "第一条"}, {ID: "s20", Sequence: 20, SourceText: "冲突条目"}}, "整理员", now.Add(2*time.Minute))
	var rule *RuleError
	if !errors.As(err, &rule) || rule.Line != 2 || rule.Field != "items[1].sequence" {
		t.Fatalf("期望第二行顺序冲突，得到 %#v", err)
	}
	if aggregate.Package.Version != version || len(aggregate.Segments) != 1 || len(aggregate.Events) != eventCount {
		t.Fatal("失败批次留下了部分写入")
	}
	if err := aggregate.AddSegments([]SegmentInput{{ID: " s30 ", Sequence: 30, SourceText: " 第三条 "}, {ID: "s10", Sequence: 10, SourceText: "第一条"}}, "整理员", now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if aggregate.Package.Version != version+1 || len(aggregate.Segments) != 3 {
		t.Fatal("合法批次没有只递增一次版本")
	}
	if aggregate.Segments[0].ID != "s10" || aggregate.Segments[1].ID != "existing" || aggregate.Segments[2].ID != "s30" || aggregate.Segments[2].SourceText != "第三条" {
		t.Fatalf("片段规范化或排序错误: %#v", aggregate.Segments)
	}
}

func TestClassifySegmentsProgressAndAtomicFailure(t *testing.T) {
	aggregate, now := newConsentedAggregate(t)
	if err := aggregate.AddSegments([]SegmentInput{{ID: "s1", Sequence: 1, SourceText: "公开"}, {ID: "s2", Sequence: 2, SourceText: "隐私"}, {ID: "s3", Sequence: 3, SourceText: "省略"}}, "整理员", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	version := aggregate.Package.Version
	err := aggregate.ClassifySegments([]ClassificationInput{{SegmentID: "s1", Decision: DecisionPublic}, {SegmentID: "s2", Decision: DecisionRestricted, RiskTags: []string{"privacy", "time_limited"}}}, "整理员", now.Add(2*time.Minute))
	var rule *RuleError
	if !errors.As(err, &rule) || rule.Line != 2 || rule.Field != "items[1].restrictionUntil" {
		t.Fatalf("期望第二行期限错误，得到 %#v", err)
	}
	if aggregate.Package.Version != version || aggregate.Segments[0].Decision != DecisionPending {
		t.Fatal("失败批量判定改变了聚合")
	}
	deadline := now.Add(48 * time.Hour)
	inputs := []ClassificationInput{{SegmentID: "s1", Decision: DecisionPublic}, {SegmentID: "s2", Decision: DecisionRestricted, RiskTags: []string{"privacy", "time_limited"}, RestrictionUntil: &deadline}, {SegmentID: "s3", Decision: DecisionOmit}}
	if err := aggregate.ClassifySegments(inputs, "整理员", now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	progress := aggregate.ClassificationProgress()
	if aggregate.Package.Version != version+1 || progress.Pending != 0 || progress.Public != 1 || progress.Restricted != 1 || progress.NotPublic != 1 || progress.RiskTagCounts["privacy"] != 1 || progress.WithRestrictionUntil != 1 {
		t.Fatalf("判定进度错误: %#v", progress)
	}
}

func TestRevisionHistoryKeepsResolutionRevision(t *testing.T) {
	aggregate, now := newConsentedAggregate(t)
	_ = aggregate.AddSegment("s1", 1, "张三住在十七号", "整理员", now.Add(time.Minute))
	_ = aggregate.ClassifySegment("s1", DecisionRestricted, []string{"privacy"}, nil, "整理员", now.Add(2*time.Minute))
	_ = aggregate.CompleteClassification("整理员", now.Add(3*time.Minute))
	_ = aggregate.ReviseSegment("s1", "张三住在老街", "隐去门牌", "整理员甲", now.Add(4*time.Minute))
	_ = aggregate.ReviseSegment("s1", "某人住在老街", "隐去姓名", "整理员乙", now.Add(5*time.Minute))
	_ = aggregate.SubmitReview("整理员", now.Add(6*time.Minute))
	_ = aggregate.ReviewSegment("s1", VerdictReturned, "地点仍需泛化", "复核员", now.Add(7*time.Minute))
	if err := aggregate.ReviseSegment("s1", "某人住在城区", "泛化地点", "整理员甲", now.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if aggregate.Findings[0].ResolvedByRevision == nil || *aggregate.Findings[0].ResolvedByRevision != 3 {
		t.Fatal("退回项未固定关联第三版")
	}
	if err := aggregate.ReviseSegment("s1", "受访者居住在城区", "调整措辞", "整理员乙", now.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if *aggregate.Findings[0].ResolvedByRevision != 3 {
		t.Fatal("后续修订改写了整改闭环版本")
	}
	history, err := aggregate.RevisionHistory("s1")
	if err != nil || len(history) != 4 || history[0].Revision != 1 || history[3].Revision != 4 || history[2].Actor != "整理员甲" || len(history[2].ResolvedFindingIDs) != 1 {
		t.Fatalf("修订历史错误: %#v, %v", history, err)
	}
	comparison, err := aggregate.CompareRevisions("s1", 3, 4)
	if err != nil || comparison.From.PublicText == comparison.To.PublicText {
		t.Fatalf("相邻版本对照错误: %#v, %v", comparison, err)
	}
	aggregate.Package.Status = StatusFrozen
	if err := aggregate.ReviseSegment("s1", "第五版", "不应成功", "整理员", now.Add(10*time.Minute)); !errors.Is(err, ErrFrozen) {
		t.Fatalf("冻结保护失效: %v", err)
	}
	if historyAfter, _ := aggregate.RevisionHistory("s1"); len(historyAfter) != 4 {
		t.Fatal("冻结后的失败修订改变了历史")
	}
}

func TestReviewSegmentsValidatesWholeBatch(t *testing.T) {
	aggregate, now := newConsentedAggregate(t)
	_ = aggregate.AddSegments([]SegmentInput{{ID: "s1", Sequence: 1, SourceText: "一"}, {ID: "s2", Sequence: 2, SourceText: "二"}}, "整理员", now.Add(time.Minute))
	_ = aggregate.ClassifySegments([]ClassificationInput{{SegmentID: "s1", Decision: DecisionPublic}, {SegmentID: "s2", Decision: DecisionPublic}}, "整理员", now.Add(2*time.Minute))
	_ = aggregate.CompleteClassification("整理员", now.Add(3*time.Minute))
	_ = aggregate.SubmitReview("整理员", now.Add(4*time.Minute))
	version := aggregate.Package.Version
	err := aggregate.ReviewSegments(aggregate.Round, []ReviewInput{{SegmentID: "s1", Verdict: VerdictApproved}, {SegmentID: "s2", Verdict: VerdictReturned}}, "复核员", now.Add(5*time.Minute))
	var rule *RuleError
	if !errors.As(err, &rule) || rule.Line != 2 || len(aggregate.Findings) != 0 || aggregate.Package.Version != version {
		t.Fatalf("批量裁决没有原子失败: %#v", err)
	}
	err = aggregate.ReviewSegments(aggregate.Round, []ReviewInput{{SegmentID: "s1", Verdict: VerdictApproved}, {SegmentID: "s2", Verdict: VerdictReturned, Reason: "需要继续泛化"}}, "复核员", now.Add(6*time.Minute))
	if err != nil || len(aggregate.Findings) != 2 || aggregate.Package.Version != version+1 || aggregate.Package.Status != StatusRemediation {
		t.Fatalf("批量裁决失败: %v", err)
	}
	if aggregate.Findings[0].ReviewedAt != aggregate.Findings[1].ReviewedAt || aggregate.Findings[0].Reviewer != aggregate.Findings[1].Reviewer {
		t.Fatal("同批裁决的复核员或时间不一致")
	}
}

package review_queue_cache_alias_test

import (
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func TestReviewQueueDoesNotPollutePackageList(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	base := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	seedPackage(t, repository, "review-older", domain.StatusReviewPending, base)
	seedPackage(t, repository, "draft-newer", domain.StatusDraft, base.Add(time.Minute))
	service := application.NewService(repository)

	before, err := service.ListPackages()
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 2 || before[0].ID != "draft-newer" || before[1].ID != "review-older" {
		t.Fatalf("测试前提不成立，初始总览为 %#v", before)
	}

	queue, err := service.ListReviewQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 1 || queue[0].ID != "review-older" {
		t.Fatalf("复核队列不正确：%#v", queue)
	}

	after, err := service.ListPackages()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 || after[0].ID != "draft-newer" || after[1].ID != "review-older" {
		t.Fatalf("复核队列查询污染了总览缓存：调用前=%v，调用后=%v", packageIDs(before), packageIDs(after))
	}
}

func seedPackage(t *testing.T, repository *store.Repository, id string, status domain.PackageStatus, updatedAt time.Time) {
	t.Helper()
	aggregate := &domain.Aggregate{Package: domain.InterviewPackage{
		ID:              id,
		Topic:           "缓存别名复现",
		ParticipantCode: "P-001",
		OwnerName:       "资料负责人",
		IntendedScope:   "公开展示",
		Status:          status,
		Version:         1,
		CreatedAt:       updatedAt.Add(-time.Hour),
		UpdatedAt:       updatedAt,
	}}
	_, _, err := repository.Commit(store.CommitRequest{
		PackageID:       id,
		ExpectedVersion: 0,
		IdempotencyKey:  "seed-" + id,
		RequestDigest:   "digest-" + id,
		Create:          aggregate,
	})
	if err != nil {
		t.Fatalf("写入测试访谈包 %s 失败：%v", id, err)
	}
}

func packageIDs(values []application.PackageSummary) []string {
	result := make([]string, len(values))
	for i := range values {
		result[i] = values[i].ID
	}
	return result
}

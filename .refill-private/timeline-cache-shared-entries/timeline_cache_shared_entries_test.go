package timeline_cache_shared_entries_test

import (
	"sync"
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func TestTimelineCacheDoesNotShareMutableEntries(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })

	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	service := application.NewServiceWithClock(repository, func() time.Time { return now })
	_, _, err = service.CreatePackage(application.CreatePackageCommand{
		WriteMeta: application.WriteMeta{IdempotencyKey: "create-timeline-cache", Actor: "整理员甲", Role: application.RoleOrganizer},
		ID:        "timeline-cache-package", Topic: "地方工艺记忆", ParticipantCode: "P-017", OwnerName: "资料负责人", IntendedScope: "公开展示",
	})
	if err != nil {
		t.Fatal(err)
	}

	warmed, err := service.GetPackage("timeline-cache-package")
	if err != nil {
		t.Fatal(err)
	}
	if len(warmed.Timeline) == 0 {
		t.Fatal("创建事件未进入时间线")
	}

	start := make(chan struct{})
	observed := make(chan string, 1)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		warmed.Timeline[0].Actor = "被外部调用方污染"
	}()
	go func() {
		defer workers.Done()
		<-start
		current, getErr := service.GetPackage("timeline-cache-package")
		if getErr != nil {
			observed <- "读取失败: " + getErr.Error()
			return
		}
		observed <- current.Timeline[0].Actor
	}()
	close(start)
	workers.Wait()
	_ = <-observed

	finalView, err := service.GetPackage("timeline-cache-package")
	if err != nil {
		t.Fatal(err)
	}
	if finalView.Timeline[0].Actor != "整理员甲" {
		t.Fatalf("时间线缓存被调用方污染: actor=%q", finalView.Timeline[0].Actor)
	}
}

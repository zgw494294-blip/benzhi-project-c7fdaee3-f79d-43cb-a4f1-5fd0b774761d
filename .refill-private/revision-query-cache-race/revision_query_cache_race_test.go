package revisionquerycacherace_test

import (
	"sync"
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func TestConcurrentRevisionQueriesSynchronizeCache(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })

	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	service := application.NewServiceWithClock(repository, func() time.Time { return now })
	created, _, err := service.CreatePackage(application.CreatePackageCommand{
		WriteMeta:       application.WriteMeta{IdempotencyKey: "create", Actor: "整理员", Role: application.RoleOrganizer},
		ID:              "revision-cache-package",
		Topic:           "并发修订查询",
		ParticipantCode: "P-RACE",
		OwnerName:       "资料负责人",
		IntendedScope:   "公开展示",
	})
	if err != nil {
		t.Fatal(err)
	}
	consented, _, err := service.ConfirmConsent(created.Package.ID, application.ConfirmConsentCommand{
		WriteMeta:             application.WriteMeta{ExpectedVersion: created.Package.Version, IdempotencyKey: "consent", Actor: "整理员", Role: application.RoleOrganizer},
		Terms:                 "参与者同意公开展示",
		AllowedUses:           []string{"公开展示"},
		AttributionPreference: "匿名",
		ConfirmedAt:           now,
		ConfirmedBy:           "参与者",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = service.AddSegment(created.Package.ID, application.AddSegmentCommand{
		WriteMeta:  application.WriteMeta{ExpectedVersion: consented.Package.Version, IdempotencyKey: "segment", Actor: "整理员", Role: application.RoleOrganizer},
		ID:         "shared-segment",
		Sequence:   1,
		SourceText: "用于受控并发查询的访谈片段",
	})
	if err != nil {
		t.Fatal(err)
	}

	const callers = 12
	start := make(chan struct{})
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(callers)
	done.Add(callers)
	for i := 0; i < callers; i++ {
		compare := i%2 == 0
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			if compare {
				_, _ = service.CompareRevisions(created.Package.ID, "shared-segment", 1, 2)
				return
			}
			_, _ = service.RevisionHistory(created.Package.ID, "shared-segment")
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
}

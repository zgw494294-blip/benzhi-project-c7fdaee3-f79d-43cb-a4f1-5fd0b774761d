package application

import (
	"errors"
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func TestServiceOptimisticConcurrencyAndIdempotency(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	service := NewServiceWithClock(repository, func() time.Time { return now })
	command := CreatePackageCommand{WriteMeta: WriteMeta{IdempotencyKey: "create", Actor: "整理员", Role: RoleOrganizer}, ID: "p1", Topic: "主题", ParticipantCode: "P", OwnerName: "负责人", IntendedScope: "公开展示"}
	view, replayed, err := service.CreatePackage(command)
	if err != nil || replayed {
		t.Fatalf("创建失败: %v", err)
	}
	_, replayed, err = service.CreatePackage(command)
	if err != nil || !replayed {
		t.Fatalf("创建幂等重放失败: %v", err)
	}
	consent := ConfirmConsentCommand{WriteMeta: WriteMeta{ExpectedVersion: view.Package.Version + 1, IdempotencyKey: "consent", Actor: "整理员", Role: RoleOrganizer}, Terms: "同意公开", AllowedUses: []string{"公开展示"}, AttributionPreference: "匿名", ConfirmedAt: now, ConfirmedBy: "参与者"}
	if _, _, err := service.ConfirmConsent("p1", consent); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("期望版本冲突，得到 %v", err)
	}
	consent.ExpectedVersion = view.Package.Version
	updated, _, err := service.ConfirmConsent("p1", consent)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Package.Status != domain.StatusConsentConfirmed {
		t.Fatal("同意状态未推进")
	}
}

func TestBatchSegmentsPersistAndReplayWithoutDuplication(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithClock(repository, func() time.Time { return now })
	created, _, err := service.CreatePackage(CreatePackageCommand{WriteMeta: WriteMeta{IdempotencyKey: "create-batch", Actor: "整理员", Role: RoleOrganizer}, ID: "batch-package", Topic: "主题", ParticipantCode: "P", OwnerName: "负责人", IntendedScope: "公开展示"})
	if err != nil {
		t.Fatal(err)
	}
	consented, _, err := service.ConfirmConsent("batch-package", ConfirmConsentCommand{WriteMeta: WriteMeta{ExpectedVersion: created.Package.Version, IdempotencyKey: "consent-batch", Actor: "整理员", Role: RoleOrganizer}, Terms: "同意公开", AllowedUses: []string{"公开展示"}, AttributionPreference: "匿名", ConfirmedAt: now, ConfirmedBy: "参与者"})
	if err != nil {
		t.Fatal(err)
	}
	command := AddSegmentsCommand{WriteMeta: WriteMeta{ExpectedVersion: consented.Package.Version, IdempotencyKey: "segments-batch", Actor: "整理员", Role: RoleOrganizer}, Items: []domain.SegmentInput{{ID: "s30", Sequence: 30, SourceText: "三"}, {ID: "s10", Sequence: 10, SourceText: "一"}, {ID: "s20", Sequence: 20, SourceText: "二"}}}
	view, replayed, err := service.AddSegments("batch-package", command)
	if err != nil || replayed || view.AddedCount != 3 || view.Package.Version != consented.Package.Version+1 {
		t.Fatalf("首次批量录入失败: %#v %v %v", view, replayed, err)
	}
	replayedView, replayed, err := service.AddSegments("batch-package", command)
	if err != nil || !replayed || len(replayedView.Segments) != 3 || replayedView.AddedCount != 3 {
		t.Fatalf("幂等重放失败: %#v %v %v", replayedView, replayed, err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := NewService(reopened).GetPackage("batch-package")
	if err != nil || len(loaded.Segments) != 3 || loaded.Segments[0].Sequence != 10 || loaded.Segments[2].Sequence != 30 {
		t.Fatalf("重启恢复批次失败: %#v %v", loaded.Segments, err)
	}
}

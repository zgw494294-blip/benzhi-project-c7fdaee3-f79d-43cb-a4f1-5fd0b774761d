package shared_directory_writers_test

import (
	"fmt"
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func commitCreate(repository *store.Repository, id string) error {
	aggregate, err := domain.NewAggregate(id, "主题", "P", "负责人", "公开展示", "整理员", time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC))
	if err != nil {
		return err
	}
	if _, _, err := repository.Commit(store.CommitRequest{PackageID: id, IdempotencyKey: "create-" + id, RequestDigest: "digest-" + id, Create: aggregate}); err != nil {
		return err
	}
	return nil
}

func TestSecondRepositoryCannotCorruptSharedDirectory(t *testing.T) {
	directory := t.TempDir()
	first, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	second, secondOpenErr := store.Open(directory)
	if secondOpenErr != nil {
		first.Close()
		return
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		results <- commitCreate(first, "first-package")
	}()
	go func() {
		<-start
		results <- commitCreate(second, "second-package")
	}()
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(fmt.Errorf("concurrent commit failed before recovery check: %w", err))
		}
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(directory); err != nil {
		t.Fatalf("second Repository was allowed to write the same directory and corrupted recovery: %v", err)
	}
	t.Fatal("second Repository unexpectedly shared the directory without exclusive ownership")
}

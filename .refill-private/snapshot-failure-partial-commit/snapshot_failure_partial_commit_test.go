package snapshot_failure_partial_commit_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func TestCommitErrorDoesNotExposePartiallyCommittedState(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "snapshot.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	aggregate, err := domain.NewAggregate("package-1", "主题", "P", "负责人", "公开展示", "整理员", time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	_, _, commitErr := repository.Commit(store.CommitRequest{PackageID: "package-1", IdempotencyKey: "create-1", RequestDigest: "digest-1", Create: aggregate})
	if commitErr == nil {
		if _, err := repository.Get("package-1"); err != nil {
			t.Fatalf("successful commit did not expose its state: %v", err)
		}
		if err := repository.Close(); err != nil {
			t.Fatal(err)
		}
		reopened, err := store.Open(directory)
		if err != nil {
			t.Fatalf("successful commit was not recoverable after restart: %v", err)
		}
		defer reopened.Close()
		if _, err := reopened.Get("package-1"); err != nil {
			t.Fatalf("successful commit disappeared after restart: %v", err)
		}
		return
	}
	if _, err := repository.Get("package-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Commit returned %v but the failed mutation remains visible in memory", commitErr)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(directory, "snapshot.json")); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.Get("package-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Commit returned an error but the mutation was replayed after restart: %v", err)
	}
}

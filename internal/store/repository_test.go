package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

func TestRepositoryRestartIdempotencyAndConflict(t *testing.T) {
	directory := t.TempDir()
	repository, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	aggregate, _ := domain.NewAggregate("p1", "主题", "P", "负责人", "公开展示", "整理员", time.Now())
	request := CommitRequest{PackageID: "p1", IdempotencyKey: "key-1", RequestDigest: "digest-1", Create: aggregate, Mutate: func(value *domain.Aggregate, _ func() uint64) (json.RawMessage, error) {
		return json.Marshal(value.Package)
	}}
	first, replayed, err := repository.Commit(request)
	if err != nil || replayed {
		t.Fatalf("首次提交: replayed=%v err=%v", replayed, err)
	}
	second, replayed, err := repository.Commit(request)
	if err != nil || !replayed || string(first) != string(second) {
		t.Fatalf("幂等重放失败: %v %v", replayed, err)
	}
	request.RequestDigest = "changed"
	if _, _, err := repository.Commit(request); !errors.Is(err, domain.ErrIdempotencyKey) {
		t.Fatalf("期望幂等冲突，得到 %v", err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.Get("p1")
	if err != nil || loaded.Package.Version != 1 {
		t.Fatalf("重启恢复失败: %v", err)
	}
}

func TestRepositoryDetectsTruncatedLog(t *testing.T) {
	directory := t.TempDir()
	repository, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	aggregate, _ := domain.NewAggregate("p1", "主题", "P", "负责人", "公开展示", "整理员", time.Now())
	_, _, err = repository.Commit(CommitRequest{PackageID: "p1", IdempotencyKey: "key", RequestDigest: "digest", Create: aggregate, Mutate: func(value *domain.Aggregate, _ func() uint64) (json.RawMessage, error) {
		return json.RawMessage(`{}`), nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "events.bin")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, info.Size()-3); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(directory); !errors.Is(err, domain.ErrIntegrity) {
		t.Fatalf("期望完整性错误，得到 %v", err)
	}
}

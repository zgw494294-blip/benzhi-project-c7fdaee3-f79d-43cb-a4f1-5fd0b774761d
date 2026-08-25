package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

func readSnapshot(path string) (*snapshot, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 64<<20))
	decoder.DisallowUnknownFields()
	var value snapshot
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("%w: 快照解析失败: %v", domain.ErrIntegrity, err)
	}
	if value.SchemaVersion != schemaVersion || value.Packages == nil || value.Idempotency == nil {
		return nil, fmt.Errorf("%w: 快照版本或结构无效", domain.ErrIntegrity)
	}
	return &value, nil
}

func snapshotIdempotency(packages map[string]*domain.Aggregate, records map[string]IdempotencyRecord) map[string]IdempotencyRecord {
	result := make(map[string]IdempotencyRecord, len(records))
	for key, record := range records {
		aggregate := packages[record.PackageID]
		if aggregate != nil && aggregate.Package.Status.IsFrozen() {
			continue
		}
		record.Response = append(json.RawMessage(nil), record.Response...)
		result[key] = record
	}
	return result
}

func writeSnapshot(path string, value snapshot) error {
	directory := filepath.Dir(path)
	file, err := os.CreateTemp(directory, ".snapshot-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	keep := false
	defer func() {
		file.Close()
		if !keep {
			os.Remove(temporary)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	keep = true
	dir, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

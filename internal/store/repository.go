package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

type Repository struct {
	mu           sync.Mutex
	directory    string
	logPath      string
	snapshotPath string
	logFile      *os.File
	state        snapshot
}

func Open(directory string) (*Repository, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, errors.New("存储目录不能为空")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	r := &Repository{directory: directory, logPath: filepath.Join(directory, "events.bin"), snapshotPath: filepath.Join(directory, "snapshot.json")}
	file, err := os.OpenFile(r.logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	r.logFile = file
	if err := r.restore(); err != nil {
		file.Close()
		return nil, err
	}
	return r, nil
}

func (r *Repository) restore() error {
	frames, err := readFrames(r.logFile)
	if err != nil {
		return err
	}
	base, err := readSnapshot(r.snapshotPath)
	if err != nil {
		return err
	}
	if base == nil {
		r.state = snapshot{SchemaVersion: schemaVersion, NextSerial: 1, Packages: map[string]*domain.Aggregate{}, Idempotency: map[string]IdempotencyRecord{}}
	} else {
		r.state = *base
	}
	if r.state.NextSerial == 0 {
		r.state.NextSerial = 1
	}
	if r.state.LastSequence > uint64(len(frames)) {
		return fmt.Errorf("%w: 快照超前于事件日志", domain.ErrIntegrity)
	}
	if r.state.LastSequence > 0 && frames[r.state.LastSequence-1].Checksum != r.state.LastHash {
		return fmt.Errorf("%w: 快照与事件链不一致", domain.ErrIntegrity)
	}
	for _, frame := range frames {
		if frame.Sequence <= r.state.LastSequence {
			continue
		}
		if frame.Aggregate != nil {
			r.state.Packages[frame.PackageID] = frame.Aggregate
		}
		if frame.Idempotency != nil {
			r.state.Idempotency[idempotencyIndex(frame.PackageID, frame.Idempotency.Key)] = *frame.Idempotency
		}
		r.state.NextSerial = frame.NextSerial
		r.state.LastSequence = frame.Sequence
		r.state.LastHash = frame.Checksum
	}
	return nil
}

func (r *Repository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.logFile == nil {
		return nil
	}
	err := r.logFile.Close()
	r.logFile = nil
	return err
}

func (r *Repository) Get(id string) (*domain.Aggregate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value := r.state.Packages[id]
	if value == nil {
		return nil, domain.ErrNotFound
	}
	return cloneAggregate(value)
}

func (r *Repository) List() ([]*domain.Aggregate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]*domain.Aggregate, 0, len(r.state.Packages))
	for _, value := range r.state.Packages {
		copied, err := cloneAggregate(value)
		if err != nil {
			return nil, err
		}
		result = append(result, copied)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Package.UpdatedAt.After(result[j].Package.UpdatedAt) })
	return result, nil
}

func (r *Repository) Commit(request CommitRequest) (json.RawMessage, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if request.PackageID == "" || request.IdempotencyKey == "" || request.RequestDigest == "" {
		return nil, false, domain.ErrInvalidInput
	}
	index := idempotencyIndex(request.PackageID, request.IdempotencyKey)
	if previous, ok := r.state.Idempotency[index]; ok {
		if previous.RequestDigest != request.RequestDigest {
			return nil, false, domain.ErrIdempotencyKey
		}
		return append(json.RawMessage(nil), previous.Response...), true, nil
	}
	current := r.state.Packages[request.PackageID]
	if request.Create != nil {
		if current != nil || request.ExpectedVersion != 0 {
			return nil, false, domain.ErrVersionConflict
		}
		var err error
		current, err = cloneAggregate(request.Create)
		if err != nil {
			return nil, false, err
		}
	} else {
		if current == nil {
			return nil, false, domain.ErrNotFound
		}
		if current.Package.Version != request.ExpectedVersion {
			return nil, false, domain.ErrVersionConflict
		}
		var err error
		current, err = cloneAggregate(current)
		if err != nil {
			return nil, false, err
		}
	}
	nextSerial := r.state.NextSerial
	allocate := func() uint64 { value := nextSerial; nextSerial++; return value }
	var response json.RawMessage
	var err error
	if request.Mutate != nil {
		response, err = request.Mutate(current, allocate)
		if err != nil {
			return nil, false, err
		}
	}
	if len(response) == 0 {
		response = json.RawMessage(`{}`)
	}
	record := IdempotencyRecord{PackageID: request.PackageID, Key: request.IdempotencyKey, RequestDigest: request.RequestDigest, Response: append(json.RawMessage(nil), response...), CreatedAt: time.Now().UTC()}
	frame := EventFrame{SchemaVersion: schemaVersion, Sequence: r.state.LastSequence + 1, PreviousHash: r.state.LastHash, Kind: "aggregate.commit", PackageID: request.PackageID, Aggregate: current, Idempotency: &record, NextSerial: nextSerial, WrittenAt: time.Now().UTC()}
	checksum, err := frameChecksum(frame)
	if err != nil {
		return nil, false, err
	}
	frame.Checksum = checksum
	offset, err := r.logFile.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, false, err
	}
	if err := appendFrame(r.logFile, frame); err != nil {
		return nil, false, err
	}
	packages, err := cloneMap(r.state.Packages)
	if err != nil {
		r.rollbackLog(offset)
		return nil, false, err
	}
	packages[request.PackageID] = current
	idempotency := cloneIdempotency(r.state.Idempotency)
	idempotency[index] = record
	copyState := snapshot{SchemaVersion: schemaVersion, LastSequence: frame.Sequence, LastHash: frame.Checksum, NextSerial: nextSerial, Packages: packages, Idempotency: idempotency}
	if err := writeSnapshot(r.snapshotPath, copyState); err != nil {
		r.rollbackLog(offset)
		return nil, false, err
	}
	r.state.Packages[request.PackageID] = current
	r.state.Idempotency[index] = record
	r.state.NextSerial = nextSerial
	r.state.LastSequence = frame.Sequence
	r.state.LastHash = frame.Checksum
	return append(json.RawMessage(nil), response...), false, nil
}

func (r *Repository) rollbackLog(offset int64) {
	if truncErr := r.logFile.Truncate(offset); truncErr == nil {
		_ = r.logFile.Sync()
	}
}

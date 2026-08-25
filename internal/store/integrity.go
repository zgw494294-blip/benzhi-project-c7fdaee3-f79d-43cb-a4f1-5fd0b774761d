package store

import (
	"fmt"
	"os"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

type IntegrityReport struct {
	Valid            bool      `json:"valid"`
	EventFrameCount  int       `json:"eventFrameCount"`
	LastSequence     uint64    `json:"lastSequence"`
	LastHash         string    `json:"lastHash"`
	SnapshotSequence uint64    `json:"snapshotSequence"`
	PackageCount     int       `json:"packageCount"`
	IdempotencyCount int       `json:"idempotencyCount"`
	CheckedAt        time.Time `json:"checkedAt"`
}

func (r *Repository) InspectIntegrity() (IntegrityReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	report := IntegrityReport{CheckedAt: time.Now().UTC()}
	inspectionFile, err := os.Open(r.logPath)
	if err != nil {
		return report, err
	}
	frames, readErr := readFrames(inspectionFile)
	closeErr := inspectionFile.Close()
	if readErr != nil {
		return report, readErr
	}
	if closeErr != nil {
		return report, closeErr
	}
	currentSnapshot, err := readSnapshot(r.snapshotPath)
	if err != nil {
		return report, err
	}
	if currentSnapshot == nil && len(frames) > 0 {
		return report, fmt.Errorf("%w: 事件日志存在但投影快照缺失", domain.ErrIntegrity)
	}
	if len(frames) > 0 {
		last := frames[len(frames)-1]
		report.LastSequence = last.Sequence
		report.LastHash = last.Checksum
		if last.Sequence != r.state.LastSequence || last.Checksum != r.state.LastHash {
			return report, fmt.Errorf("%w: 内存投影与磁盘事件链不一致", domain.ErrIntegrity)
		}
	}
	if currentSnapshot != nil {
		report.SnapshotSequence = currentSnapshot.LastSequence
		if currentSnapshot.LastSequence != report.LastSequence || currentSnapshot.LastHash != report.LastHash {
			return report, fmt.Errorf("%w: 快照未追平事件链", domain.ErrIntegrity)
		}
		if currentSnapshot.NextSerial != r.state.NextSerial {
			return report, fmt.Errorf("%w: 凭据序号投影不一致", domain.ErrIntegrity)
		}
	}
	report.EventFrameCount = len(frames)
	report.PackageCount = len(r.state.Packages)
	report.IdempotencyCount = len(r.state.Idempotency)
	report.Valid = true
	return report, nil
}

package store

import (
	"encoding/json"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

const schemaVersion = 1

type IdempotencyRecord struct {
	PackageID     string          `json:"packageID"`
	Key           string          `json:"key"`
	RequestDigest string          `json:"requestDigest"`
	Response      json.RawMessage `json:"response"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type EventFrame struct {
	SchemaVersion int                `json:"schemaVersion"`
	Sequence      uint64             `json:"sequence"`
	PreviousHash  string             `json:"previousHash"`
	Checksum      string             `json:"checksum"`
	Kind          string             `json:"kind"`
	PackageID     string             `json:"packageID"`
	Aggregate     *domain.Aggregate  `json:"aggregate,omitempty"`
	Idempotency   *IdempotencyRecord `json:"idempotency,omitempty"`
	NextSerial    uint64             `json:"nextSerial"`
	WrittenAt     time.Time          `json:"writtenAt"`
}

type snapshot struct {
	SchemaVersion int                          `json:"schemaVersion"`
	LastSequence  uint64                       `json:"lastSequence"`
	LastHash      string                       `json:"lastHash"`
	NextSerial    uint64                       `json:"nextSerial"`
	Packages      map[string]*domain.Aggregate `json:"packages"`
	Idempotency   map[string]IdempotencyRecord `json:"idempotency"`
}

type Mutator func(aggregate *domain.Aggregate, allocateSerial func() uint64) (json.RawMessage, error)

type CommitRequest struct {
	PackageID       string
	ExpectedVersion uint64
	IdempotencyKey  string
	RequestDigest   string
	Create          *domain.Aggregate
	Mutate          Mutator
}

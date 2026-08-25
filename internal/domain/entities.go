package domain

import "time"

type InterviewPackage struct {
	ID              string        `json:"id"`
	Topic           string        `json:"topic"`
	ParticipantCode string        `json:"participantCode"`
	OwnerName       string        `json:"ownerName"`
	IntendedScope   string        `json:"intendedScope"`
	Status          PackageStatus `json:"status"`
	Version         uint64        `json:"version"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

type ConsentRecord struct {
	PackageID             string     `json:"packageID"`
	Terms                 string     `json:"terms"`
	AllowedUses           []string   `json:"allowedUses"`
	AttributionPreference string     `json:"attributionPreference"`
	WithdrawalDeadline    *time.Time `json:"withdrawalDeadline,omitempty"`
	ConfirmedAt           time.Time  `json:"confirmedAt"`
	ConfirmedBy           string     `json:"confirmedBy"`
	TermsDigest           string     `json:"termsDigest"`
}

type TranscriptSegment struct {
	ID               string          `json:"id"`
	PackageID        string          `json:"packageID"`
	Sequence         int             `json:"sequence"`
	SourceText       string          `json:"sourceText"`
	RiskTags         []string        `json:"riskTags"`
	RestrictionUntil *time.Time      `json:"restrictionUntil,omitempty"`
	Decision         SegmentDecision `json:"decision"`
	PublicText       string          `json:"publicText"`
	Revision         uint64          `json:"revision"`
	RevisionReason   string          `json:"revisionReason,omitempty"`
	BeforeSummary    string          `json:"beforeSummary,omitempty"`
	AfterSummary     string          `json:"afterSummary,omitempty"`
}

// SegmentRevision 是受限片段一次不可覆盖的脱敏修订。
type SegmentRevision struct {
	PackageID          string    `json:"packageID"`
	SegmentID          string    `json:"segmentID"`
	Revision           uint64    `json:"revision"`
	PublicText         string    `json:"publicText"`
	Reason             string    `json:"reason"`
	BeforeSummary      string    `json:"beforeSummary"`
	AfterSummary       string    `json:"afterSummary"`
	Actor              string    `json:"actor"`
	At                 time.Time `json:"at"`
	ResolvedFindingIDs []string  `json:"resolvedFindingIDs"`
}

type ReviewFinding struct {
	ID                 string        `json:"id"`
	PackageID          string        `json:"packageID"`
	SegmentID          string        `json:"segmentID"`
	Round              int           `json:"round"`
	Verdict            ReviewVerdict `json:"verdict"`
	Reason             string        `json:"reason"`
	ResolvedByRevision *uint64       `json:"resolvedByRevision,omitempty"`
	RevisionAtReturn   uint64        `json:"revisionAtReturn,omitempty"`
	Reviewer           string        `json:"reviewer"`
	ReviewedAt         time.Time     `json:"reviewedAt"`
}

type FrozenSegment struct {
	ID         string   `json:"id"`
	Sequence   int      `json:"sequence"`
	PublicText string   `json:"publicText"`
	RiskTags   []string `json:"riskTags,omitempty"`
}

type FrozenManifest struct {
	PackageID       string          `json:"packageID"`
	Topic           string          `json:"topic"`
	ParticipantCode string          `json:"participantCode"`
	IntendedScope   string          `json:"intendedScope"`
	TermsDigest     string          `json:"termsDigest"`
	ConsentSummary  string          `json:"consentSummary"`
	Segments        []FrozenSegment `json:"segments"`
	FrozenBy        string          `json:"frozenBy"`
	FrozenAt        time.Time       `json:"frozenAt"`
	Digest          string          `json:"digest"`
}

type ReleaseCredential struct {
	Serial         uint64    `json:"serial"`
	PackageID      string    `json:"packageID"`
	ManifestDigest string    `json:"manifestDigest"`
	TermsDigest    string    `json:"termsDigest"`
	IssuedBy       string    `json:"issuedBy"`
	IssuedAt       time.Time `json:"issuedAt"`
	SchemaVersion  int       `json:"schemaVersion"`
}

type BusinessEvent struct {
	ID        string         `json:"id"`
	PackageID string         `json:"packageID"`
	Type      EventType      `json:"type"`
	Actor     string         `json:"actor"`
	At        time.Time      `json:"at"`
	Version   uint64         `json:"version"`
	Details   map[string]any `json:"details,omitempty"`
}

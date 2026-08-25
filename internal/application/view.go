package application

import (
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/audit"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

type PackageView struct {
	Package                domain.InterviewPackage       `json:"package"`
	Consent                *domain.ConsentRecord         `json:"consent,omitempty"`
	Segments               []domain.TranscriptSegment    `json:"segments"`
	Findings               []domain.ReviewFinding        `json:"findings"`
	Manifest               *domain.FrozenManifest        `json:"manifest,omitempty"`
	Credential             *domain.ReleaseCredential     `json:"credential,omitempty"`
	Timeline               []audit.TimelineItem          `json:"timeline"`
	Verification           audit.Verification            `json:"verification"`
	Evidence               *audit.PublicEvidence         `json:"evidence,omitempty"`
	Readiness              domain.WorkflowReadiness      `json:"readiness"`
	ReviewRound            int                           `json:"reviewRound"`
	ClassificationProgress domain.ClassificationProgress `json:"classificationProgress"`
	ReviewProgress         domain.ReviewProgress         `json:"reviewProgress"`
	AddedCount             int                           `json:"addedCount,omitempty"`
}

func makeView(aggregate *domain.Aggregate) PackageView {
	return PackageView{Package: aggregate.Package, Consent: aggregate.Consent, Segments: aggregate.Segments, Findings: aggregate.Findings, Manifest: aggregate.Manifest, Credential: aggregate.Credential, Timeline: audit.Timeline(aggregate.Events), Verification: audit.VerifyCredential(aggregate.Manifest, aggregate.Credential), Evidence: audit.BuildEvidence(aggregate.Manifest, aggregate.Credential), Readiness: aggregate.Readiness(), ReviewRound: aggregate.Round, ClassificationProgress: aggregate.ClassificationProgress(), ReviewProgress: aggregate.CurrentReviewProgress()}
}

type PackageSummary struct {
	ID               string                `json:"id"`
	Topic            string                `json:"topic"`
	ParticipantCode  string                `json:"participantCode"`
	OwnerName        string                `json:"ownerName"`
	Status           domain.PackageStatus  `json:"status"`
	Version          uint64                `json:"version"`
	SegmentCount     int                   `json:"segmentCount"`
	CredentialSerial uint64                `json:"credentialSerial,omitempty"`
	UpdatedAt        time.Time             `json:"updatedAt"`
	ReviewProgress   domain.ReviewProgress `json:"reviewProgress"`
}

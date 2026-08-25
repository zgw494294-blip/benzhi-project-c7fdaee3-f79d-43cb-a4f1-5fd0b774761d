package application

import (
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

const (
	RoleOrganizer      = "organizer"
	RoleReviewer       = "reviewer"
	RoleReleaseManager = "release_manager"
)

type WriteMeta struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"actor"`
	Role            string `json:"role"`
}

type CreatePackageCommand struct {
	WriteMeta
	ID              string `json:"id"`
	Topic           string `json:"topic"`
	ParticipantCode string `json:"participantCode"`
	OwnerName       string `json:"ownerName"`
	IntendedScope   string `json:"intendedScope"`
}

type ConfirmConsentCommand struct {
	WriteMeta
	Terms                 string     `json:"terms"`
	AllowedUses           []string   `json:"allowedUses"`
	AttributionPreference string     `json:"attributionPreference"`
	WithdrawalDeadline    *time.Time `json:"withdrawalDeadline,omitempty"`
	ConfirmedAt           time.Time  `json:"confirmedAt"`
	ConfirmedBy           string     `json:"confirmedBy"`
}

type AddSegmentCommand struct {
	WriteMeta
	ID         string `json:"id"`
	Sequence   int    `json:"sequence"`
	SourceText string `json:"sourceText"`
}

type AddSegmentsCommand struct {
	WriteMeta
	Items []domain.SegmentInput `json:"items"`
}

type ClassifySegmentCommand struct {
	WriteMeta
	SegmentID        string                 `json:"segmentID"`
	Decision         domain.SegmentDecision `json:"decision"`
	RiskTags         []string               `json:"riskTags"`
	RestrictionUntil *time.Time             `json:"restrictionUntil,omitempty"`
}

type ClassifySegmentsCommand struct {
	WriteMeta
	Items []domain.ClassificationInput `json:"items"`
}

type CompleteClassificationCommand struct{ WriteMeta }

type ReviseSegmentCommand struct {
	WriteMeta
	SegmentID  string `json:"segmentID"`
	PublicText string `json:"publicText"`
	Reason     string `json:"reason"`
}

type SubmitReviewCommand struct{ WriteMeta }

type ReviewSegmentCommand struct {
	WriteMeta
	SegmentID string               `json:"segmentID"`
	Verdict   domain.ReviewVerdict `json:"verdict"`
	Reason    string               `json:"reason"`
}

type ReviewSegmentsCommand struct {
	WriteMeta
	ReviewRound int                  `json:"reviewRound"`
	Items       []domain.ReviewInput `json:"items"`
}

type ApproveReleaseCommand struct {
	WriteMeta
	PreviewManifestDigest string `json:"previewManifestDigest"`
}

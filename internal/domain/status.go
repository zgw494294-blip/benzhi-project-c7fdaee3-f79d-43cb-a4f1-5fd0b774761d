package domain

type PackageStatus string

const (
	StatusDraft            PackageStatus = "draft"
	StatusConsentConfirmed PackageStatus = "consent_confirmed"
	StatusClassifying      PackageStatus = "classifying"
	StatusRedactionPending PackageStatus = "redaction_pending"
	StatusReviewPending    PackageStatus = "review_pending"
	StatusRemediation      PackageStatus = "remediation"
	StatusApprovalPending  PackageStatus = "approval_pending"
	StatusFrozen           PackageStatus = "frozen"
	StatusReleased         PackageStatus = "released"
)

func (s PackageStatus) IsTerminal() bool { return s == StatusReleased }

func (s PackageStatus) IsFrozen() bool {
	return s == StatusFrozen || s == StatusReleased
}

type SegmentDecision string

const (
	DecisionPending    SegmentDecision = "pending"
	DecisionPublic     SegmentDecision = "public"
	DecisionRestricted SegmentDecision = "restricted"
	DecisionOmit       SegmentDecision = "omit"
)

func (d SegmentDecision) ValidFinal() bool {
	return d == DecisionPublic || d == DecisionRestricted || d == DecisionOmit
}

type ReviewVerdict string

const (
	VerdictApproved ReviewVerdict = "approved"
	VerdictReturned ReviewVerdict = "returned"
)

type EventType string

const (
	EventPackageCreated     EventType = "package.created"
	EventConsentConfirmed   EventType = "consent.confirmed"
	EventSegmentAdded       EventType = "segment.added"
	EventSegmentsAdded      EventType = "segments.batch_added"
	EventSegmentClassified  EventType = "segment.classified"
	EventSegmentsClassified EventType = "segments.batch_classified"
	EventClassified         EventType = "classification.completed"
	EventRevisionSubmitted  EventType = "revision.submitted"
	EventReviewSubmitted    EventType = "review.submitted"
	EventReviewReturned     EventType = "review.returned"
	EventReviewApproved     EventType = "review.approved"
	EventManifestFrozen     EventType = "manifest.frozen"
	EventCredentialIssued   EventType = "credential.issued"
)

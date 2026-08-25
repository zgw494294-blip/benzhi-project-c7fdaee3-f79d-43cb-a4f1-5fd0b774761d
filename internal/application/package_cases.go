package application

import (
	"encoding/json"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func (s *Service) CreatePackage(command CreatePackageCommand) (PackageView, bool, error) {
	if err := validateMeta(command.WriteMeta, RoleOrganizer); err != nil {
		return PackageView{}, false, err
	}
	if command.ExpectedVersion != 0 {
		return PackageView{}, false, domain.ErrVersionConflict
	}
	now := s.now().UTC()
	aggregate, err := domain.NewAggregate(command.ID, command.Topic, command.ParticipantCode, command.OwnerName, command.IntendedScope, command.Actor, now)
	if err != nil {
		return PackageView{}, false, err
	}
	digest, err := requestDigest("create_package", command)
	if err != nil {
		return PackageView{}, false, err
	}
	response, replayed, err := s.repository.Commit(store.CommitRequest{PackageID: aggregate.Package.ID, ExpectedVersion: 0, IdempotencyKey: command.IdempotencyKey, RequestDigest: digest, Create: aggregate, Mutate: func(value *domain.Aggregate, _ func() uint64) (json.RawMessage, error) { return encodeView(value) }})
	if err != nil {
		return PackageView{}, false, err
	}
	s.invalidateListCache()
	var view PackageView
	if err := json.Unmarshal(response, &view); err != nil {
		return PackageView{}, false, err
	}
	return view, replayed, nil
}

func (s *Service) ConfirmConsent(packageID string, command ConfirmConsentCommand) (PackageView, bool, error) {
	return s.mutate(packageID, "confirm_consent", command.WriteMeta, command, RoleOrganizer, func(aggregate *domain.Aggregate, _ func() uint64) error {
		consent := domain.ConsentRecord{Terms: command.Terms, AllowedUses: command.AllowedUses, AttributionPreference: command.AttributionPreference, WithdrawalDeadline: command.WithdrawalDeadline, ConfirmedAt: command.ConfirmedAt, ConfirmedBy: command.ConfirmedBy}
		return aggregate.ConfirmConsent(consent, command.Actor, s.now().UTC())
	})
}

func (s *Service) AddSegment(packageID string, command AddSegmentCommand) (PackageView, bool, error) {
	return s.mutate(packageID, "add_segment", command.WriteMeta, command, RoleOrganizer, func(aggregate *domain.Aggregate, _ func() uint64) error {
		return aggregate.AddSegment(command.ID, command.Sequence, command.SourceText, command.Actor, s.now().UTC())
	})
}

func (s *Service) AddSegments(packageID string, command AddSegmentsCommand) (PackageView, bool, error) {
	view, replayed, err := s.mutate(packageID, "add_segments", command.WriteMeta, command, RoleOrganizer, func(aggregate *domain.Aggregate, _ func() uint64) error {
		return aggregate.AddSegments(command.Items, command.Actor, s.now().UTC())
	})
	if err == nil {
		view.AddedCount = len(command.Items)
	}
	return view, replayed, err
}

func (s *Service) ClassifySegment(packageID string, command ClassifySegmentCommand) (PackageView, bool, error) {
	return s.mutate(packageID, "classify_segment", command.WriteMeta, command, RoleOrganizer, func(aggregate *domain.Aggregate, _ func() uint64) error {
		return aggregate.ClassifySegment(command.SegmentID, command.Decision, command.RiskTags, command.RestrictionUntil, command.Actor, s.now().UTC())
	})
}

func (s *Service) ClassifySegments(packageID string, command ClassifySegmentsCommand) (PackageView, bool, error) {
	return s.mutate(packageID, "classify_segments", command.WriteMeta, command, RoleOrganizer, func(aggregate *domain.Aggregate, _ func() uint64) error {
		return aggregate.ClassifySegments(command.Items, command.Actor, s.now().UTC())
	})
}

func (s *Service) CompleteClassification(packageID string, command CompleteClassificationCommand) (PackageView, bool, error) {
	return s.mutate(packageID, "complete_classification", command.WriteMeta, command, RoleOrganizer, func(aggregate *domain.Aggregate, _ func() uint64) error {
		return aggregate.CompleteClassification(command.Actor, s.now().UTC())
	})
}

func (s *Service) ReviseSegment(packageID string, command ReviseSegmentCommand) (PackageView, bool, error) {
	return s.mutate(packageID, "revise_segment", command.WriteMeta, command, RoleOrganizer, func(aggregate *domain.Aggregate, _ func() uint64) error {
		return aggregate.ReviseSegment(command.SegmentID, command.PublicText, command.Reason, command.Actor, s.now().UTC())
	})
}

func (s *Service) SubmitReview(packageID string, command SubmitReviewCommand) (PackageView, bool, error) {
	return s.mutate(packageID, "submit_review", command.WriteMeta, command, RoleOrganizer, func(aggregate *domain.Aggregate, _ func() uint64) error {
		return aggregate.SubmitReview(command.Actor, s.now().UTC())
	})
}

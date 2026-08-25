package application

import (
	"context"
	"crypto/subtle"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/audit"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

func (s *Service) ReviewSegment(packageID string, command ReviewSegmentCommand) (PackageView, bool, error) {
	return s.mutate(packageID, "review_segment", command.WriteMeta, command, RoleReviewer, func(aggregate *domain.Aggregate, _ func() uint64) error {
		return aggregate.ReviewSegment(command.SegmentID, command.Verdict, command.Reason, command.Actor, s.now().UTC())
	})
}

func (s *Service) ReviewSegments(packageID string, command ReviewSegmentsCommand) (PackageView, bool, error) {
	return s.mutate(packageID, "review_segments", command.WriteMeta, command, RoleReviewer, func(aggregate *domain.Aggregate, _ func() uint64) error {
		return aggregate.ReviewSegments(command.ReviewRound, command.Items, command.Actor, s.now().UTC())
	})
}

func (s *Service) PreviewRelease(packageID string) (audit.ManifestPreview, error) {
	aggregate, err := s.repository.Get(packageID)
	if err != nil {
		return audit.ManifestPreview{}, err
	}
	return audit.BuildManifestPreview(aggregate, s.now().UTC())
}

func (s *Service) ApproveRelease(packageID string, command ApproveReleaseCommand) (PackageView, bool, error) {
	return s.ApproveReleaseContext(context.Background(), packageID, command)
}

func (s *Service) ApproveReleaseContext(_ context.Context, packageID string, command ApproveReleaseCommand) (PackageView, bool, error) {
	return s.mutate(packageID, "approve_release", command.WriteMeta, command, RoleReleaseManager, func(aggregate *domain.Aggregate, allocate func() uint64) error {
		now := s.now().UTC()
		manifest, err := audit.BuildManifest(aggregate, command.Actor, now)
		if err != nil {
			return err
		}
		if command.PreviewManifestDigest == "" {
			return &domain.RuleError{Field: "previewManifestDigest", Code: "required", Message: "批准前必须确认冻结清单预览"}
		}
		if subtle.ConstantTimeCompare([]byte(command.PreviewManifestDigest), []byte(manifest.Digest)) != 1 {
			return &domain.RuleError{Field: "previewManifestDigest", Code: "preview_expired", Message: "冻结清单预览已过期，请重新预览并确认"}
		}
		if err := aggregate.SetFrozen(manifest, command.Actor, now); err != nil {
			return err
		}
		credential, err := audit.IssueCredential(manifest, allocate(), command.Actor, now)
		if err != nil {
			return err
		}
		return aggregate.SetCredential(credential, command.Actor, now)
	})
}

func (s *Service) RevisionHistory(packageID, segmentID string) ([]domain.SegmentRevision, error) {
	aggregate, err := s.repository.Get(packageID)
	if err != nil {
		return nil, err
	}
	return aggregate.RevisionHistory(segmentID)
}

func (s *Service) CompareRevisions(packageID, segmentID string, from, to uint64) (domain.RevisionComparison, error) {
	aggregate, err := s.repository.Get(packageID)
	if err != nil {
		return domain.RevisionComparison{}, err
	}
	return aggregate.CompareRevisions(segmentID, from, to)
}

func (s *Service) VerifyCredential(packageID string) (audit.Verification, error) {
	aggregate, err := s.repository.Get(packageID)
	if err != nil {
		return audit.Verification{}, err
	}
	return audit.VerifyCredential(aggregate.Manifest, aggregate.Credential), nil
}

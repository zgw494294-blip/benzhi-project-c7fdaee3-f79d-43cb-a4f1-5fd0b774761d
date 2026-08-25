package httpui

import (
	"net/http"
	"strconv"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

func (s *Server) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := assets.ReadFile("assets/index.html")
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) ListPackagesHandler(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	var values []application.PackageSummary
	var err error
	if status == "review_pending" {
		values, err = s.service.ListReviewQueue()
	} else if status == "" {
		values, err = s.service.ListPackages()
	} else {
		writeError(w, &domain.RuleError{Field: "status", Code: "invalid", Message: "仅支持 review_pending 待复核筛选"})
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"packages": values})
}

func (s *Server) ReviewQueueHandler(w http.ResponseWriter, _ *http.Request) {
	values, err := s.service.ListReviewQueue()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"packages": values})
}

func (s *Server) CreatePackageHandler(w http.ResponseWriter, r *http.Request) {
	var command application.CreatePackageCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	view, replayed, err := s.service.CreatePackage(command)
	if err != nil {
		writeError(w, err)
		return
	}
	if replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) GetPackageHandler(w http.ResponseWriter, r *http.Request) {
	view, err := s.service.GetPackage(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) ConfirmConsentHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ConfirmConsentCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	view, replayed, err := s.service.ConfirmConsent(r.PathValue("id"), command)
	s.writeMutation(w, view, replayed, err)
}

func (s *Server) AddSegmentHandler(w http.ResponseWriter, r *http.Request) {
	var command application.AddSegmentCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	view, replayed, err := s.service.AddSegment(r.PathValue("id"), command)
	s.writeMutation(w, view, replayed, err)
}

func (s *Server) AddSegmentsHandler(w http.ResponseWriter, r *http.Request) {
	var command application.AddSegmentsCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	view, replayed, err := s.service.AddSegments(r.PathValue("id"), command)
	s.writeMutation(w, view, replayed, err)
}

func (s *Server) ClassifySegmentHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ClassifySegmentCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SegmentID = r.PathValue("segmentID")
	view, replayed, err := s.service.ClassifySegment(r.PathValue("id"), command)
	s.writeMutation(w, view, replayed, err)
}

func (s *Server) ClassifySegmentsHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ClassifySegmentsCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	view, replayed, err := s.service.ClassifySegments(r.PathValue("id"), command)
	s.writeMutation(w, view, replayed, err)
}

func (s *Server) CompleteClassificationHandler(w http.ResponseWriter, r *http.Request) {
	var command application.CompleteClassificationCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	view, replayed, err := s.service.CompleteClassification(r.PathValue("id"), command)
	s.writeMutation(w, view, replayed, err)
}

func (s *Server) ReviseSegmentHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ReviseSegmentCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SegmentID = r.PathValue("segmentID")
	view, replayed, err := s.service.ReviseSegment(r.PathValue("id"), command)
	s.writeMutation(w, view, replayed, err)
}

func (s *Server) RevisionHistoryHandler(w http.ResponseWriter, r *http.Request) {
	values, err := s.service.RevisionHistory(r.PathValue("id"), r.PathValue("segmentID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revisions": values})
}

func (s *Server) CompareRevisionsHandler(w http.ResponseWriter, r *http.Request) {
	from, fromErr := strconv.ParseUint(r.URL.Query().Get("from"), 10, 64)
	to, toErr := strconv.ParseUint(r.URL.Query().Get("to"), 10, 64)
	if fromErr != nil || toErr != nil {
		writeError(w, &domain.RuleError{Field: "revision", Code: "invalid", Message: "from 和 to 必须是有效修订号"})
		return
	}
	value, err := s.service.CompareRevisions(r.PathValue("id"), r.PathValue("segmentID"), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) SubmitReviewHandler(w http.ResponseWriter, r *http.Request) {
	var command application.SubmitReviewCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	view, replayed, err := s.service.SubmitReview(r.PathValue("id"), command)
	s.writeMutation(w, view, replayed, err)
}

func (s *Server) ReviewSegmentHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ReviewSegmentCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SegmentID = r.PathValue("segmentID")
	view, replayed, err := s.service.ReviewSegment(r.PathValue("id"), command)
	s.writeMutation(w, view, replayed, err)
}

func (s *Server) ReviewSegmentsHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ReviewSegmentsCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	view, replayed, err := s.service.ReviewSegments(r.PathValue("id"), command)
	s.writeMutation(w, view, replayed, err)
}

func (s *Server) PreviewReleaseHandler(w http.ResponseWriter, r *http.Request) {
	preview, err := s.service.PreviewRelease(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) ApproveReleaseHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ApproveReleaseCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	view, replayed, err := s.service.ApproveReleaseContext(r.Context(), r.PathValue("id"), command)
	s.writeMutation(w, view, replayed, err)
}

func (s *Server) TimelineHandler(w http.ResponseWriter, r *http.Request) {
	view, err := s.service.GetPackage(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"timeline": view.Timeline})
}

func (s *Server) VerifyCredentialHandler(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.VerifyCredential(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) writeMutation(w http.ResponseWriter, view application.PackageView, replayed bool, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	if replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	writeJSON(w, http.StatusOK, view)
}

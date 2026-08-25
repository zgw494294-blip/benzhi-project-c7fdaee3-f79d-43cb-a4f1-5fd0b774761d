package httpui

import (
	"encoding/json"
	"errors"
	"net/http"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Field     string `json:"field,omitempty"`
	Line      int    `json:"line,omitempty"`
	SegmentID string `json:"segmentID,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusUnprocessableEntity
	body := apiError{Code: "business_rule", Message: err.Error()}
	var rule *domain.RuleError
	switch {
	case errors.As(err, &rule):
		body.Code = rule.Code
		body.Message = rule.Message
		body.Field = rule.Field
		body.Line = rule.Line
		body.SegmentID = rule.SegmentID
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		body.Code = "not_found"
	case errors.Is(err, domain.ErrVersionConflict):
		status = http.StatusConflict
		body.Code = "version_conflict"
	case errors.Is(err, domain.ErrIdempotencyKey):
		status = http.StatusConflict
		body.Code = "idempotency_conflict"
	case errors.Is(err, application.ErrForbidden):
		status = http.StatusForbidden
		body.Code = "forbidden"
	case errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
		body.Code = "invalid_input"
	case errors.Is(err, domain.ErrIntegrity):
		status = http.StatusInternalServerError
		body.Code = "integrity_error"
	}
	writeJSON(w, status, errorBody{Error: body})
}

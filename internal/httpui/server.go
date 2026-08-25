package httpui

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
)

//go:embed assets/*
var assets embed.FS

type Server struct {
	service *application.Service
	logger  *slog.Logger
	handler http.Handler
}

func New(service *application.Service, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.IndexHandler)
	assetFS, _ := fs.Sub(assets, "assets")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetFS))))
	mux.HandleFunc("GET /api/packages", s.ListPackagesHandler)
	mux.HandleFunc("GET /api/review-queue", s.ReviewQueueHandler)
	mux.HandleFunc("POST /api/packages", s.CreatePackageHandler)
	mux.HandleFunc("GET /api/packages/{id}", s.GetPackageHandler)
	mux.HandleFunc("POST /api/packages/{id}/consent", s.ConfirmConsentHandler)
	mux.HandleFunc("POST /api/packages/{id}/segments", s.AddSegmentHandler)
	mux.HandleFunc("POST /api/packages/{id}/segments/batch", s.AddSegmentsHandler)
	mux.HandleFunc("POST /api/packages/{id}/segments/{segmentID}/classify", s.ClassifySegmentHandler)
	mux.HandleFunc("POST /api/packages/{id}/classification/batch", s.ClassifySegmentsHandler)
	mux.HandleFunc("POST /api/packages/{id}/classification/complete", s.CompleteClassificationHandler)
	mux.HandleFunc("POST /api/packages/{id}/segments/{segmentID}/revision", s.ReviseSegmentHandler)
	mux.HandleFunc("GET /api/packages/{id}/segments/{segmentID}/revisions", s.RevisionHistoryHandler)
	mux.HandleFunc("GET /api/packages/{id}/segments/{segmentID}/revisions/compare", s.CompareRevisionsHandler)
	mux.HandleFunc("POST /api/packages/{id}/review/submit", s.SubmitReviewHandler)
	mux.HandleFunc("POST /api/packages/{id}/segments/{segmentID}/review", s.ReviewSegmentHandler)
	mux.HandleFunc("POST /api/packages/{id}/review/batch", s.ReviewSegmentsHandler)
	mux.HandleFunc("GET /api/packages/{id}/release/preview", s.PreviewReleaseHandler)
	mux.HandleFunc("POST /api/packages/{id}/release/approve", s.ApproveReleaseHandler)
	mux.HandleFunc("GET /api/packages/{id}/timeline", s.TimelineHandler)
	mux.HandleFunc("GET /api/packages/{id}/credential/verify", s.VerifyCredentialHandler)
	s.handler = s.security(s.recoverer(mux))
	return s
}

func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("HTTP 处理发生异常", "error", recovered)
				writeJSON(w, http.StatusInternalServerError, errorBody{Error: apiError{Code: "internal_error", Message: "服务内部错误"}})
			}
		}()
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Header.Get("Content-Type") != "" && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") && r.Method != http.MethodGet {
			writeJSON(w, http.StatusUnsupportedMediaType, errorBody{Error: apiError{Code: "content_type", Message: "请求必须使用 application/json"}})
			return
		}
		next.ServeHTTP(w, r)
	})
}

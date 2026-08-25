package httpui

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/application"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repository.Close() })
	return New(application.NewService(repository), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestIndexAndSecurityHeaders(t *testing.T) {
	server := testServer(t)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "<body>") {
		t.Fatalf("首页无效: %d", recorder.Code)
	}
	if recorder.Header().Get("Content-Security-Policy") == "" || recorder.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("安全响应头缺失")
	}
}

func TestCreateEndpointAndBodyLimit(t *testing.T) {
	server := testServer(t)
	body := `{"expectedVersion":0,"idempotencyKey":"k","actor":"整理员","role":"organizer","id":"p","topic":"主题","participantCode":"P","ownerName":"负责人","intendedScope":"公开展示"}`
	request := httptest.NewRequest(http.MethodPost, "/api/packages", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("创建返回 %d: %s", recorder.Code, recorder.Body.String())
	}
	large := bytes.Repeat([]byte("x"), maximumBodyBytes+1)
	request = httptest.NewRequest(http.MethodPost, "/api/packages", bytes.NewReader(large))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity && recorder.Code != http.StatusBadRequest {
		t.Fatalf("超大请求体返回 %d", recorder.Code)
	}
}

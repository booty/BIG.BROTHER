package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"bigbrother/internal/media"
)

func TestHealthzReturnsJSONStatus(t *testing.T) {
	handler := NewHandler(t.TempDir(), nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if response["status"] != "ok" {
		t.Fatalf("status body = %#v, want status ok", response)
	}
}

func TestHandlerServesLocalAsset(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("local intercom shell"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(staticDir, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "local intercom shell" {
		t.Fatalf("body = %q, want local asset contents", got)
	}
}

func TestMediaAvailabilityIsSeparateFromApplicationLiveness(t *testing.T) {
	manager := media.New(media.Options{})
	proxy, err := media.NewProxy(manager, "http://127.0.0.1:8889")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(t.TempDir(), nil, proxy, manager.StatusHandler())

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want application liveness 200", health.Code)
	}

	status := httptest.NewRecorder()
	handler.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/media/status", nil))
	var mediaStatus media.Status
	if err := json.Unmarshal(status.Body.Bytes(), &mediaStatus); err != nil {
		t.Fatal(err)
	}
	if status.Code != http.StatusOK || mediaStatus.Available || mediaStatus.Message == "" {
		t.Fatalf("media status = %d %#v, want separate unavailable status", status.Code, mediaStatus)
	}
}

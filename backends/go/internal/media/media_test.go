package media

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestAssignStreamsAreDistinctAndReleaseCleansOwnedSessions(t *testing.T) {
	var mu sync.Mutex
	var deleted []string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		if request.Method != http.MethodDelete {
			t.Fatalf("cleanup method = %q, want DELETE", request.Method)
		}
		deleted = append(deleted, request.URL.RequestURI())
		return newHTTPResponse(request, http.StatusOK, "", nil), nil
	})}
	manager := New(Options{HTTPURL: "http://media.test", HTTPClient: client})
	first, err := manager.AssignStream("session-one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.AssignStream("session-two")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !validStreamID(first) || !validStreamID(second) {
		t.Fatalf("stream IDs = %q and %q, want distinct 24-character hex IDs", first, second)
	}

	manager.recordSession(first, "/"+first+"/whip/publisher-one")
	manager.recordSession(first, "/"+second+"/whep/reader-one")
	manager.recordSession(second, "/"+second+"/whip/publisher-two")
	manager.ReleaseStream("session-one")

	mu.Lock()
	gotDeleted := append([]string(nil), deleted...)
	mu.Unlock()
	if len(gotDeleted) != 2 {
		t.Fatalf("deleted resources = %v, want two resources owned by first stream", gotDeleted)
	}
	gotSet := map[string]bool{}
	for _, resource := range gotDeleted {
		gotSet[resource] = true
	}
	for _, want := range []string{"/" + first + "/whip/publisher-one", "/" + second + "/whep/reader-one"} {
		if !gotSet[want] {
			t.Fatalf("cleanup resources = %v, missing %q", gotDeleted, want)
		}
	}
	if _, exists := manager.streams["session-one"]; exists {
		t.Fatal("released participant still has a stream association")
	}
	if len(manager.sessions[second]) != 1 {
		t.Fatalf("other participant session count = %d, want 1", len(manager.sessions[second]))
	}
}

func TestProxyPreservesNegotiationAndTracksSessionCleanup(t *testing.T) {
	const streamID = "0123456789abcdef01234567"
	const ownerID = "fedcba9876543210fedcba98"
	var requests []*http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body []byte
		if request.Body != nil {
			var err error
			body, err = io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
		}
		request.Body = io.NopCloser(strings.NewReader(string(body)))
		requests = append(requests, request.Clone(context.Background()))
		switch request.Method {
		case http.MethodOptions:
			response := newHTTPResponse(request, http.StatusNoContent, "", nil)
			response.Header.Set("Access-Control-Allow-Origin", "*")
			response.Header.Set("Link", "<https://stun.invalid>; rel=ice-server")
			return response, nil
		case http.MethodPost:
			response := newHTTPResponse(request, http.StatusCreated, "application/sdp", []byte("v=0\\r\\n"))
			response.Header.Set("Location", "http://127.0.0.1:8889/"+streamID+"/whip/session-one")
			response.Header.Set("ETag", "session-tag")
			return response, nil
		case http.MethodPatch:
			return newHTTPResponse(request, http.StatusNoContent, "", nil), nil
		case http.MethodDelete:
			return newHTTPResponse(request, http.StatusNotFound, "", nil), nil
		default:
			return nil, fmt.Errorf("unexpected method %s", request.Method)
		}
	})
	manager := New(Options{HTTPClient: &http.Client{Transport: transport}})
	manager.process.running = true
	manager.process.done = make(chan struct{})
	proxy, err := NewProxy(manager, "http://127.0.0.1:8889")
	if err != nil {
		t.Fatal(err)
	}

	options := httptest.NewRequest(http.MethodOptions, "/media/"+streamID+"/whip", nil)
	options.Header.Set("Access-Control-Request-Method", "POST")
	optionsRecorder := httptest.NewRecorder()
	proxy.ServeHTTP(optionsRecorder, options)
	if optionsRecorder.Code != http.StatusNoContent || optionsRecorder.Header().Get("Link") == "" {
		t.Fatalf("OPTIONS response = %d, headers %v", optionsRecorder.Code, optionsRecorder.Header())
	}

	post := httptest.NewRequest(http.MethodPost, "/media/"+streamID+"/whip", strings.NewReader("offer-sdp"))
	post.Header.Set("Content-Type", "application/sdp")
	post.Header.Set(ownerHeader, ownerID)
	postRecorder := httptest.NewRecorder()
	proxy.ServeHTTP(postRecorder, post)
	if postRecorder.Code != http.StatusCreated || postRecorder.Header().Get("Content-Type") != "application/sdp" || postRecorder.Header().Get("ETag") != "session-tag" {
		t.Fatalf("POST response = %d, headers %v", postRecorder.Code, postRecorder.Header())
	}
	if got, want := postRecorder.Header().Get("Location"), "/media/"+streamID+"/whip/session-one"; got != want {
		t.Fatalf("rewritten Location = %q, want %q", got, want)
	}
	if len(manager.sessions[ownerID]) != 1 {
		t.Fatalf("tracked sessions = %d, want 1", len(manager.sessions[ownerID]))
	}

	patchRequest := httptest.NewRequest(http.MethodPatch, "/media/"+streamID+"/whip/session-one", strings.NewReader("candidate-fragment"))
	patchRequest.Header.Set("Content-Type", "application/trickle-ice-sdpfrag")
	patchRequest.Header.Set(ownerHeader, ownerID)
	patchRecorder := httptest.NewRecorder()
	proxy.ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusNoContent {
		t.Fatalf("PATCH status = %d, want 204", patchRecorder.Code)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/media/"+streamID+"/whip/session-one", nil)
	deleteRequest.Header.Set(ownerHeader, ownerID)
	deleteRecorder := httptest.NewRecorder()
	proxy.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusNotFound {
		t.Fatalf("DELETE status = %d, want the upstream 404 preserved", deleteRecorder.Code)
	}
	if len(manager.sessions[ownerID]) != 0 {
		t.Fatalf("tracked sessions after DELETE = %d, want 0", len(manager.sessions[ownerID]))
	}
	if len(requests) != 4 {
		t.Fatalf("upstream requests = %d, want OPTIONS, POST, PATCH, DELETE", len(requests))
	}
	if got := requests[1].URL.Path; got != "/"+streamID+"/whip" {
		t.Fatalf("upstream POST path = %q", got)
	}
	if got := requests[1].Header.Get("Content-Type"); got != "application/sdp" {
		t.Fatalf("forwarded POST Content-Type = %q", got)
	}
	postBody, _ := io.ReadAll(requests[1].Body)
	if string(postBody) != "offer-sdp" {
		t.Fatalf("forwarded POST body = %q", postBody)
	}
	patchBody, _ := io.ReadAll(requests[2].Body)
	if string(patchBody) != "candidate-fragment" || requests[2].Header.Get("Content-Type") != "application/trickle-ice-sdpfrag" {
		t.Fatalf("forwarded PATCH body/content type = %q / %q", patchBody, requests[2].Header.Get("Content-Type"))
	}
}

func TestProxyReportsUnavailableMediaAndRejectsOtherRoutes(t *testing.T) {
	manager := New(Options{})
	proxy, err := NewProxy(manager, "http://127.0.0.1:8889")
	if err != nil {
		t.Fatal(err)
	}
	const streamID = "0123456789abcdef01234567"
	request := httptest.NewRequest(http.MethodPost, "/media/"+streamID+"/whip", strings.NewReader("offer"))
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "MediaMTX is not running") {
		t.Fatalf("unavailable response = %d %q", response.Code, response.Body.String())
	}

	manager.process.running = true
	manager.process.done = make(chan struct{})
	badRequest := httptest.NewRequest(http.MethodGet, "/media/"+streamID+"/whep", nil)
	badResponse := httptest.NewRecorder()
	proxy.ServeHTTP(badResponse, badRequest)
	if badResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unsupported route status = %d, want 405", badResponse.Code)
	}
}

func TestMediaProcessReadinessUnexpectedExitAndShutdown(t *testing.T) {
	config := filepath.Join(t.TempDir(), "mediamtx.yml")
	if err := os.WriteFile(config, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(filepath.Dir(config), "mediamtx")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexec sleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := New(Options{
		BinaryPath: binary,
		ConfigPath: config,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return newHTTPResponse(request, http.StatusOK, "application/json", []byte(`{"items":[]}`)), nil
		})},
		ReadyTimeout:    time.Second,
		ShutdownTimeout: time.Second,
		PollInterval:    time.Millisecond,
		Stdout:          io.Discard,
		Stderr:          io.Discard,
	})
	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !manager.Status().Available {
		t.Fatal("media should be available after the readiness response")
	}
	if err := manager.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if status := manager.Status(); status.Available {
		t.Fatalf("status after Stop = %#v, want unavailable", status)
	}
}

func TestMediaProcessFailsStartupWhenChildExitsBeforeReadiness(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "mediamtx")
	config := filepath.Join(directory, "mediamtx.yml")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 7\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Options{
		BinaryPath: binary,
		ConfigPath: config,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return newHTTPResponse(request, http.StatusServiceUnavailable, "", nil), nil
		})},
		ReadyTimeout: time.Second,
		PollInterval: time.Millisecond,
		Stdout:       io.Discard,
		Stderr:       io.Discard,
	})
	if err := manager.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "exited before becoming ready") {
		t.Fatalf("Start error = %v, want early child exit", err)
	}
}

func TestUnexpectedMediaExitIsReportedWithoutRestart(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "mediamtx")
	config := filepath.Join(directory, "mediamtx.yml")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nsleep 0.1\nexit 9\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Options{
		BinaryPath: binary,
		ConfigPath: config,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return newHTTPResponse(request, http.StatusOK, "application/json", []byte(`{"items":[]}`)), nil
		})},
		ReadyTimeout: time.Second,
		PollInterval: time.Millisecond,
		Stdout:       io.Discard,
		Stderr:       io.Discard,
	})
	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for manager.Status().Available && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	status := manager.Status()
	if status.Available || !strings.Contains(status.Message, "MediaMTX exited") {
		t.Fatalf("status after unexpected child exit = %#v", status)
	}
}

func TestResolvePathsFindsPackagedMediaFiles(t *testing.T) {
	directory := t.TempDir()
	packageDir := filepath.Join(directory, "bundle")
	mediaDir := filepath.Join(packageDir, "media")
	if err := os.MkdirAll(mediaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(mediaDir, "mediamtx")
	config := filepath.Join(mediaDir, "mediamtx.yml")
	if err := os.WriteFile(binary, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	gotBinary, gotConfig, err := ResolvePaths(filepath.Join(packageDir, "bigbrother"), directory)
	if err != nil {
		t.Fatal(err)
	}
	if gotBinary != binary || gotConfig != config {
		t.Fatalf("ResolvePaths = %q, %q; want %q, %q", gotBinary, gotConfig, binary, config)
	}
}

func newHTTPResponse(request *http.Request, status int, contentType string, body []byte) *http.Response {
	response := &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Request:    request,
	}
	if contentType != "" {
		response.Header.Set("Content-Type", contentType)
	}
	return response
}

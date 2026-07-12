package uat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestCreateSessionEndpointValidatesAndCreatesSession(t *testing.T) {
	store := &fakeStore{
		createdSession: Session{
			ID:          42,
			TestVersion: "22 เมษายน 2569",
			TesterName:  "manuay",
			UID:         "14180",
			PwaCode:     "1020",
			Area:        "3",
			TestDate:    "2026-06-30",
		},
	}
	handler := NewHandler(store, fstest.MapFS{"index.html": {Data: []byte("ok")}})

	req := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader(`{
		"test_version": " 22 เมษายน 2569 ",
		"tester_name": " manuay ",
		"uid": " 14180 ",
		"pwa_code": " 1020 ",
		"area": " 3 ",
		"job_name": " งานแผนที่ ",
		"division": " division ",
		"institution": " institution ",
		"position": " position ",
		"test_date": "2026-06-30"
	}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.createdInput.TestVersion != "22 เมษายน 2569" {
		t.Fatalf("expected test version to be trimmed, got %q", store.createdInput.TestVersion)
	}
	if store.createdInput.TesterName != "manuay" {
		t.Fatalf("expected tester name to be trimmed, got %q", store.createdInput.TesterName)
	}
	if store.createdInput.UID != "14180" || store.createdInput.PwaCode != "1020" || store.createdInput.Area != "3" {
		t.Fatalf("expected session metadata to be trimmed, got %#v", store.createdInput)
	}

	var payload struct {
		Session Session `json:"session"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Session.ID != 42 {
		t.Fatalf("expected created session in response, got id %d", payload.Session.ID)
	}
}

func TestCreateSessionEndpointRejectsBadDate(t *testing.T) {
	store := &fakeStore{}
	handler := NewHandler(store, fstest.MapFS{"index.html": {Data: []byte("ok")}})

	req := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader(`{
		"test_version": "22 เมษายน 2569",
		"tester_name": "manuay",
		"test_date": "30/06/2026"
	}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if store.createCalled {
		t.Fatal("store should not be called for invalid input")
	}
}

func TestSessionInfoProxyForwardsCookieAndReturnsInfo(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "session_id=abc" {
			t.Fatalf("expected forwarded cookie, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","uid":"14180","uname":"Tester One","pwa_code":"1020","area":"3","job_name":"GIS","division":"IT","institution":"PWA","position":"Engineer"}`))
	}))
	defer upstream.Close()

	store := &fakeStore{}
	handler := NewHandlerWithConfig(store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, HandlerConfig{SessionInfoURL: upstream.URL})

	req := httptest.NewRequest(http.MethodGet, "/api/session-info", nil)
	req.Header.Set("Cookie", "session_id=abc")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		SessionInfo SessionInfo `json:"session_info"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.SessionInfo.UName != "Tester One" || payload.SessionInfo.Area != "3" {
		t.Fatalf("unexpected session info: %#v", payload.SessionInfo)
	}
}

func TestPatchResultEndpointRejectsMutuallyExclusiveOutcome(t *testing.T) {
	store := &fakeStore{}
	handler := NewHandler(store, fstest.MapFS{"index.html": {Data: []byte("ok")}})

	req := httptest.NewRequest(http.MethodPatch, "/api/results/7", strings.NewReader(`{
		"is_passed": true,
		"is_failed": true,
		"comment": "bad state"
	}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if store.updateCalled {
		t.Fatal("store should not be called for mutually exclusive result")
	}
}

func TestGetSessionResultsReturnsNotFound(t *testing.T) {
	store := &fakeStore{getResultsErr: ErrNotFound}
	handler := NewHandler(store, fstest.MapFS{"index.html": {Data: []byte("ok")}})

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/99/results", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

type fakeStore struct {
	createdSession Session
	createdInput   CreateSessionInput
	createCalled   bool
	updateCalled   bool
	getResultsErr  error
}

func (f *fakeStore) Health(ctx context.Context) error {
	return nil
}

func (f *fakeStore) ListReferences(ctx context.Context) (ReferenceData, error) {
	return ReferenceData{}, nil
}

func (f *fakeStore) ListTestCases(ctx context.Context, testSuite string) ([]TestCase, error) {
	return nil, nil
}

func (f *fakeStore) ListSessions(ctx context.Context, filters SessionFilters) ([]Session, error) {
	return nil, nil
}

func (f *fakeStore) CreateSession(ctx context.Context, input CreateSessionInput) (Session, error) {
	f.createCalled = true
	f.createdInput = input
	return f.createdSession, nil
}

func (f *fakeStore) DeleteSession(ctx context.Context, sessionID int64, uid string) error { return nil }

func (f *fakeStore) GetSessionResults(ctx context.Context, sessionID int64) (SessionResults, error) {
	if f.getResultsErr != nil {
		return SessionResults{}, f.getResultsErr
	}
	return SessionResults{}, nil
}

func (f *fakeStore) UpdateResult(ctx context.Context, resultID int64, input UpdateResultInput) (Result, error) {
	f.updateCalled = true
	if resultID <= 0 {
		return Result{}, errors.New("bad id")
	}
	return Result{ID: resultID, IsPassed: input.IsPassed, IsFailed: input.IsFailed, Comment: input.Comment}, nil
}

func (f *fakeStore) Report(ctx context.Context, filters ReportFilters) ([]ReportRow, error) {
	return nil, nil
}

func (f *fakeStore) Close() {}

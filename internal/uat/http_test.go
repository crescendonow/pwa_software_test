package uat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// TestRequireAuthenticationNeverLocksOutReturningUser guards against the
// regression where requireAuthentication checked an in-memory "expired(uid)"
// timestamp *before* refreshing it via touchActivity. Once a user crossed the
// inactivityLimit, the stale timestamp was never updated (the request was
// rejected before reaching touchActivity), permanently locking the user out
// until the process restarted -- even though the upstream pwa_gis_tracking
// session was still valid. The fix removes that in-memory state entirely and
// defers fully to the upstream session; a valid upstream session must always
// be accepted, however many requests (and however much real time) has passed.
func TestRequireAuthenticationNeverLocksOutReturningUser(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","uid":"14180","uname":"Tester One","pwa_code":"1020","area":"3"}`))
	}))
	defer upstream.Close()

	store := &fakeStore{}
	handler := NewHandlerWithConfig(store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, HandlerConfig{SessionInfoURL: upstream.URL})

	// Simulate the user returning many times "later" (in the old code this
	// would eventually fail once the in-memory activity timestamp aged past
	// inactivityLimit); every call must succeed since there is no longer any
	// server-side inactivity timer to expire.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/references", nil)
		req.Header.Set("Cookie", "session_id=abc")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("call %d: expected 200 for a user with a valid upstream session, got %d: %s", i, rec.Code, rec.Body.String())
		}
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

func TestListSessionsScopesVisibilityByAuthenticatedUID(t *testing.T) {
	tests := []struct {
		name        string
		uid         string
		expectedUID string
	}{
		{name: "regular user sees own sessions", uid: "14180", expectedUID: "14180"},
		{name: "UID 14744 sees every session", uid: "14744", expectedUID: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"uid":%q,"uname":"Tester"}`, tt.uid)
			}))
			defer upstream.Close()

			store := &fakeStore{}
			handler := NewHandlerWithConfig(store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, HandlerConfig{
				SessionInfoURL: upstream.URL,
			})

			req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
			req.Header.Set("Cookie", "session_id=abc")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			if store.listSessionsFilters.UID != tt.expectedUID {
				t.Fatalf("expected UID filter %q, got %q", tt.expectedUID, store.listSessionsFilters.UID)
			}
		})
	}
}

func TestListSessionsRejectsAuthenticatedSessionWithoutUID(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uname":"Tester"}`))
	}))
	defer upstream.Close()

	store := &fakeStore{}
	handler := NewHandlerWithConfig(store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, HandlerConfig{
		SessionInfoURL: upstream.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.listSessionsCalls != 0 {
		t.Fatal("store should not be called when authenticated UID is missing")
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

func TestDashboardWordcloudEndpointReturnsNLPWords(t *testing.T) {
	store := &fakeStore{comments: []string{`comment one`, `comment one`}}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf(`expected POST, got %s`, r.Method)
		}
		if r.URL.Path != `/wordcloud` {
			t.Fatalf(`expected /wordcloud, got %s`, r.URL.Path)
		}

		var payload struct {
			Comments []string
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Comments) != 2 || payload.Comments[0] != `comment one` {
			t.Fatalf(`expected comments to be forwarded, got %#v`, payload.Comments)
		}

		w.Header().Set(`Content-Type`, `application/json`)
		if err := json.NewEncoder(w).Encode([]WordCloudItem{{Word: `system`, Weight: 2}}); err != nil {
			t.Fatal(err)
		}
	}))
	defer upstream.Close()

	handler := NewHandlerWithConfig(store, fstest.MapFS{`index.html`: {Data: []byte(`ok`)}}, HandlerConfig{
		NLPURL: upstream.URL + `/wordcloud`,
	})

	req := httptest.NewRequest(http.MethodGet, `/api/dashboard/wordcloud`, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(`expected 200, got %d: %s`, rec.Code, rec.Body.String())
	}
	var payload struct {
		Words []WordCloudItem
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Words) != 1 || payload.Words[0] != (WordCloudItem{Word: `system`, Weight: 2}) {
		t.Fatalf(`expected NLP words in response, got %#v`, payload.Words)
	}
}

func TestDashboardWordcloudEndpointReturnsBadGatewayWhenNLPIsUnreachable(t *testing.T) {
	store := &fakeStore{comments: []string{`comment one`}}
	handler := NewHandlerWithConfig(store, fstest.MapFS{`index.html`: {Data: []byte(`ok`)}}, HandlerConfig{
		NLPURL:     `http://nlp.internal/wordcloud`,
		HTTPClient: &http.Client{Transport: unreachableRoundTripper{}},
	})

	req := httptest.NewRequest(http.MethodGet, `/api/dashboard/wordcloud`, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf(`expected 502, got %d: %s`, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `could not reach nlp service`) {
		t.Fatalf(`expected unreachable NLP error, got %s`, rec.Body.String())
	}
}

func TestFreeTextQueryEndpointForwardsAuthenticatedActorAndHidesSQL(t *testing.T) {
	session := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session_id=abc" {
			t.Fatalf("expected session cookie, got %q", r.Header.Get("Cookie"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uid":"14180","uname":"Tester One"}`))
	}))
	defer session.Close()

	query := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Prompt string `json:"prompt"`
			Actor  struct {
				UID   string `json:"uid"`
				UName string `json:"uname"`
			} `json:"actor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Prompt != "สรุปผล" || payload.Actor.UID != "14180" || payload.Actor.UName != "Tester One" {
			t.Fatalf("unexpected free-text payload: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","answer":"พบ 1 รายการ","columns":["total"],"rows":[{"total":1}],"row_count":1,"truncated":false}`))
	}))
	defer query.Close()

	handler := NewHandlerWithConfig(&fakeStore{}, fstest.MapFS{"index.html": {Data: []byte("ok")}}, HandlerConfig{
		SessionInfoURL:   session.URL,
		FreeTextQueryURL: query.URL + "/query",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/freetext-query", strings.NewReader(`{"prompt":"สรุปผล"}`))
	req.Header.Set("Cookie", "session_id=abc")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SELECT") {
		t.Fatalf("generated SQL must not reach browser: %s", rec.Body.String())
	}
}

func TestFreeTextQueryEndpointRejectsUnauthenticatedRequest(t *testing.T) {
	session := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer session.Close()

	called := false
	query := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer query.Close()

	handler := NewHandlerWithConfig(&fakeStore{}, fstest.MapFS{"index.html": {Data: []byte("ok")}}, HandlerConfig{
		SessionInfoURL:   session.URL,
		FreeTextQueryURL: query.URL,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/freetext-query", strings.NewReader(`{"prompt":"สรุปผล"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("expected unauthenticated request to stop at Go auth, got %d (called=%v)", rec.Code, called)
	}
}

func TestFreeTextQueryEndpointRejectsSessionWithoutUID(t *testing.T) {
	session := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uname":"Tester One"}`))
	}))
	defer session.Close()

	called := false
	query := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer query.Close()

	handler := NewHandlerWithConfig(&fakeStore{}, fstest.MapFS{"index.html": {Data: []byte("ok")}}, HandlerConfig{
		SessionInfoURL:   session.URL,
		FreeTextQueryURL: query.URL,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/freetext-query", strings.NewReader(`{"prompt":"เธชเธฃเธธเธเธเธฅ"}`))
	req.Header.Set("Cookie", "session_id=abc")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("expected missing UID to stop at Go auth, got %d (called=%v)", rec.Code, called)
	}
}

type unreachableRoundTripper struct{}

func (unreachableRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New(`connection refused`)
}

type fakeStore struct {
	createdSession      Session
	createdInput        CreateSessionInput
	createCalled        bool
	updateCalled        bool
	getResultsErr       error
	comments            []string
	listCommentsErr     error
	listSessionsFilters SessionFilters
	listSessionsCalls   int
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
	f.listSessionsCalls++
	f.listSessionsFilters = filters
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

func (f *fakeStore) DashboardSummary(ctx context.Context, filters DashboardFilters) (DashboardSummary, error) {
	return DashboardSummary{}, nil
}

func (f *fakeStore) ListComments(ctx context.Context, filters DashboardFilters) ([]string, error) {
	return f.comments, f.listCommentsErr
}

func (f *fakeStore) Close() {}

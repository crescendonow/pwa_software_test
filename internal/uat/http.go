package uat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type HandlerConfig struct {
	SessionInfoURL   string
	LoginURL         string
	LogoutURL        string
	OfficesURL       string
	NLPURL           string
	FreeTextQueryURL string
	HTTPClient       *http.Client
}

type Server struct {
	store            Store
	staticFS         fs.FS
	sessionInfoURL   string
	loginURL         string
	logoutURL        string
	officesURL       string
	nlpURL           string
	freeTextQueryURL string
	httpClient       *http.Client
}

const allSessionsUID = "14744"

func NewHandler(store Store, staticFS fs.FS) http.Handler {
	return NewHandlerWithConfig(store, staticFS, HandlerConfig{
		SessionInfoURL:   os.Getenv("SESSION_INFO_URL"),
		LoginURL:         os.Getenv("LOGIN_URL"),
		LogoutURL:        os.Getenv("LOGOUT_URL"),
		OfficesURL:       os.Getenv("OFFICES_URL"),
		NLPURL:           os.Getenv("NLP_URL"),
		FreeTextQueryURL: os.Getenv("FREETEXT_QUERY_URL"),
	})
}

func NewHandlerWithConfig(store Store, staticFS fs.FS, cfg HandlerConfig) http.Handler {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	server := &Server{
		store:            store,
		staticFS:         staticFS,
		sessionInfoURL:   strings.TrimSpace(cfg.SessionInfoURL),
		loginURL:         strings.TrimSpace(cfg.LoginURL),
		logoutURL:        strings.TrimSpace(cfg.LogoutURL),
		officesURL:       strings.TrimSpace(cfg.OfficesURL),
		nlpURL:           strings.TrimSpace(cfg.NLPURL),
		freeTextQueryURL: strings.TrimSpace(cfg.FreeTextQueryURL),
		httpClient:       client,
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /login", server.handleLoginPage)
	mux.HandleFunc("POST /login", server.handleLogin)
	mux.HandleFunc("GET /logout", server.handleLogout)
	mux.HandleFunc("GET /api/health", server.handleHealth)
	mux.HandleFunc("GET /api/references", server.handleReferences)
	mux.HandleFunc("GET /api/session-info", server.handleSessionInfo)
	mux.HandleFunc("GET /api/offices", server.handleOffices)
	mux.HandleFunc("GET /api/test-cases", server.handleTestCases)
	mux.HandleFunc("GET /api/sessions", server.handleSessions)
	mux.HandleFunc("POST /api/sessions", server.handleSessions)
	mux.HandleFunc("/api/sessions/", server.handleSessionDetail)
	mux.HandleFunc("/api/results/", server.handleResultDetail)
	mux.HandleFunc("GET /api/report", server.handleReport)
	mux.HandleFunc("GET /api/dashboard/summary", server.handleDashboardSummary)
	mux.HandleFunc("GET /api/dashboard/wordcloud", server.handleDashboardWordcloud)
	mux.HandleFunc("POST /api/freetext-query", server.handleFreeTextQuery)
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	return server.requireAuthentication(mux)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	contents, err := fs.ReadFile(s.staticFS, "login.html")
	if err != nil {
		writeError(w, http.StatusNotFound, "login page not found")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(contents)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.loginURL == "" {
		writeError(w, http.StatusServiceUnavailable, "LOGIN_URL is not configured")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid login body")
		return
	}
	upstreamURL, err := resolveConfiguredURL(s.loginURL, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid LOGIN_URL")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create login request")
		return
	}
	copyForwardHeaders(req, r)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not login")
		return
	}
	defer resp.Body.Close()

	copyRewrittenCookies(w, resp)
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "invalid login response")
		return
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var data map[string]any
		if err := json.Unmarshal(payload, &data); err == nil {
			data["redirect"] = joinBasePath(requestBasePath(r), "pwagis_uat.html")
			writeJSON(w, resp.StatusCode, data)
			return
		}
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(payload)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.logoutURL != "" {
		upstreamURL, err := resolveConfiguredURL(s.logoutURL, r)
		if err == nil {
			req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
			if reqErr == nil {
				copyForwardHeaders(req, r)
				if resp, doErr := s.httpClient.Do(req); doErr == nil {
					copyRewrittenCookies(w, resp)
					_ = resp.Body.Close()
				}
			}
		}
	}
	http.Redirect(w, r, joinBasePath(requestBasePath(r), "login"), http.StatusFound)
}
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Health(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database is not reachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleReferences(w http.ResponseWriter, r *http.Request) {
	references, err := s.store.ListReferences(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load references")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"references": references})
}

func (s *Server) handleSessionInfo(w http.ResponseWriter, r *http.Request) {
	if s.sessionInfoURL == "" {
		writeError(w, http.StatusServiceUnavailable, "SESSION_INFO_URL is not configured")
		return
	}

	upstreamURL, err := s.resolveSessionInfoURL(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid SESSION_INFO_URL")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session-info request")
		return
	}
	copyForwardHeaders(req, r)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not load session info")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		writeError(w, resp.StatusCode, "login required")
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("session info upstream returned %d", resp.StatusCode))
		return
	}

	var info SessionInfo
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&info); err != nil {
		writeError(w, http.StatusBadGateway, "invalid session info response")
		return
	}
	normalizeSessionInfo(&info)
	if info.UName == "" {
		writeError(w, http.StatusBadGateway, "session info missing uname")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session_info": info})
}

func (s *Server) resolveSessionInfoURL(r *http.Request) (string, error) {
	return resolveConfiguredURL(s.sessionInfoURL, r)
}

// handleOffices proxies the branch office list from pwa_gis_tracking
// (GET /pwa_gis_tracking/api/offices) so the create-session form can offer a
// multi-branch picker without this service talking to the office database
// directly. Reuses the same proxy pattern as handleSessionInfo.
func (s *Server) handleOffices(w http.ResponseWriter, r *http.Request) {
	if s.officesURL == "" {
		writeError(w, http.StatusServiceUnavailable, "OFFICES_URL is not configured")
		return
	}

	upstreamURL, err := resolveConfiguredURL(s.officesURL, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid OFFICES_URL")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create offices request")
		return
	}
	copyForwardHeaders(req, r)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not load offices")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		writeError(w, resp.StatusCode, "login required")
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("offices upstream returned %d", resp.StatusCode))
		return
	}

	var upstream struct {
		Status string       `json:"status"`
		Data   []OfficeInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		writeError(w, http.StatusBadGateway, "invalid offices response")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"offices": upstream.Data})
}

func resolveConfiguredURL(rawURL string, r *http.Request) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() {
		return parsed.String(), nil
	}
	if !strings.HasPrefix(rawURL, "/") {
		return "", errors.New("relative upstream URL must start with /")
	}

	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host + rawURL, nil
}
func copyForwardHeaders(dst *http.Request, src *http.Request) {
	if cookie := src.Header.Get("Cookie"); cookie != "" {
		dst.Header.Set("Cookie", cookie)
	}
	if auth := src.Header.Get("Authorization"); auth != "" {
		dst.Header.Set("Authorization", auth)
	}
	dst.Header.Set("Accept", "application/json")
}

func (s *Server) handleTestCases(w http.ResponseWriter, r *http.Request) {
	cases, err := s.store.ListTestCases(r.Context(), strings.TrimSpace(r.URL.Query().Get("test_suite")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load test cases")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"test_cases": cases})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		info, err := s.loadSessionInfo(r)
		if err != nil || info.UID == "" {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		filters := SessionFilters{
			Area:        strings.TrimSpace(r.URL.Query().Get("area")),
			TesterName:  strings.TrimSpace(r.URL.Query().Get("tester_name")),
			TestVersion: strings.TrimSpace(r.URL.Query().Get("test_version")),
			DateFrom:    strings.TrimSpace(r.URL.Query().Get("date_from")),
			DateTo:      strings.TrimSpace(r.URL.Query().Get("date_to")),
		}
		if info.UID != allSessionsUID {
			filters.UID = info.UID
		}
		sessions, err := s.store.ListSessions(r.Context(), filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load sessions")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
	case http.MethodPost:
		var input CreateSessionInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		normalizeCreateSessionInput(&input)
		if err := validateCreateSessionInput(input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// A tester may run UAT against several branches in one go: create one
		// session per selected branch (pwa_code), each with its own results.
		branches := input.Branches
		if len(branches) == 0 {
			branches = []BranchInput{{PwaCode: input.PwaCode, Name: input.BranchName, Zone: input.Area}}
		}

		sessions := make([]Session, 0, len(branches))
		for _, branch := range branches {
			branchInput := input
			branchInput.Branches = nil
			branchInput.PwaCode = strings.TrimSpace(branch.PwaCode)
			branchInput.BranchName = strings.TrimSpace(branch.Name)
			if zone := strings.TrimSpace(branch.Zone); zone != "" {
				branchInput.Area = zone
			}

			session, err := s.store.CreateSession(r.Context(), branchInput)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not create session")
				return
			}
			sessions = append(sessions, session)
		}

		writeJSON(w, http.StatusCreated, map[string]any{"session": sessions[0], "sessions": sessions})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	sessionID, suffix, ok := parseIDPath(r.URL.Path, "/api/sessions/")
	if suffix == "" && r.Method == http.MethodDelete {
		info, err := s.loadSessionInfo(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		err = s.store.DeleteSession(r.Context(), sessionID, info.UID)
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			writeError(w, http.StatusForbidden, "only the session owner can delete it")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete session")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !ok || suffix != "results" || r.Method != http.MethodGet {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}

	results, err := s.store.GetSessionResults(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load session results")
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleResultDetail(w http.ResponseWriter, r *http.Request) {
	resultID, suffix, ok := parseIDPath(r.URL.Path, "/api/results/")
	if !ok || suffix != "" || r.Method != http.MethodPatch {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}

	var input UpdateResultInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if input.IsPassed && input.IsFailed {
		writeError(w, http.StatusBadRequest, "result cannot be both passed and failed")
		return
	}

	result, err := s.store.UpdateResult(r.Context(), resultID, input)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "result not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not update result")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	filters := ReportFilters{
		Area:        strings.TrimSpace(r.URL.Query().Get("area")),
		TesterName:  strings.TrimSpace(r.URL.Query().Get("tester_name")),
		TestVersion: strings.TrimSpace(r.URL.Query().Get("test_version")),
		TestSuite:   strings.TrimSpace(r.URL.Query().Get("test_suite")),
		DateFrom:    strings.TrimSpace(r.URL.Query().Get("date_from")),
		DateTo:      strings.TrimSpace(r.URL.Query().Get("date_to")),
	}
	if rawSessionID := strings.TrimSpace(r.URL.Query().Get("session_id")); rawSessionID != "" {
		sessionID, err := strconv.ParseInt(rawSessionID, 10, 64)
		if err != nil || sessionID <= 0 {
			writeError(w, http.StatusBadRequest, "session_id must be a positive integer")
			return
		}
		filters.SessionID = sessionID
	}

	rows, err := s.store.Report(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load report")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

func dashboardFiltersFromQuery(r *http.Request) DashboardFilters {
	query := r.URL.Query()
	return DashboardFilters{
		Area:        strings.TrimSpace(query.Get("area")),
		TestVersion: strings.TrimSpace(query.Get("test_version")),
		TestSuite:   strings.TrimSpace(query.Get("test_suite")),
		DateFrom:    strings.TrimSpace(query.Get("date_from")),
		DateTo:      strings.TrimSpace(query.Get("date_to")),
	}
}

func (s *Server) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.store.DashboardSummary(r.Context(), dashboardFiltersFromQuery(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load dashboard summary")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// handleDashboardWordcloud pulls comments for the given filter from the
// database and forwards them to the internal Python NLP microservice, which
// tokenizes Thai text and returns [{word, weight}]. The Python service is
// never exposed directly to the browser.
func (s *Server) handleDashboardWordcloud(w http.ResponseWriter, r *http.Request) {
	if s.nlpURL == "" {
		writeError(w, http.StatusServiceUnavailable, "NLP_URL is not configured")
		return
	}

	comments, err := s.store.ListComments(r.Context(), dashboardFiltersFromQuery(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load comments")
		return
	}

	body, err := json.Marshal(map[string]any{"comments": comments})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not encode comments")
		return
	}

	upstreamURL, err := resolveConfiguredURL(s.nlpURL, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid NLP_URL")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create wordcloud request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not reach nlp service")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("nlp service returned %d", resp.StatusCode))
		return
	}

	var words []WordCloudItem
	if err := json.NewDecoder(resp.Body).Decode(&words); err != nil {
		writeError(w, http.StatusBadGateway, "invalid wordcloud response")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"words": words})
}

// handleFreeTextQuery forwards a prompt and the authenticated actor identity
// to the loopback-only Python service. The browser never receives generated
// SQL; the Python response is already limited to an answer and result table.
func (s *Server) handleFreeTextQuery(w http.ResponseWriter, r *http.Request) {
	if s.freeTextQueryURL == "" {
		writeError(w, http.StatusServiceUnavailable, "FREETEXT_QUERY_URL is not configured")
		return
	}

	var input FreeTextQueryInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	input.Prompt = strings.TrimSpace(input.Prompt)
	if input.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}
	if len([]rune(input.Prompt)) > 500 {
		writeError(w, http.StatusBadRequest, "prompt must be at most 500 characters")
		return
	}

	info, err := s.loadSessionInfo(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "login required")
		return
	}
	if info.UID == "" {
		writeError(w, http.StatusUnauthorized, "authenticated user id is missing")
		return
	}
	body, err := json.Marshal(map[string]any{
		"prompt": input.Prompt,
		"actor":  map[string]string{"uid": info.UID, "uname": info.UName},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not encode free-text query")
		return
	}

	upstreamURL, err := resolveConfiguredURL(s.freeTextQueryURL, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid FREETEXT_QUERY_URL")
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create free-text query request")
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not reach free-text query service")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("free-text query service returned %d", resp.StatusCode))
		return
	}

	var result FreeTextQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		writeError(w, http.StatusBadGateway, "invalid free-text query response")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func normalizeSessionInfo(info *SessionInfo) {
	info.UID = strings.TrimSpace(info.UID)
	info.UName = strings.TrimSpace(info.UName)
	info.PwaCode = strings.TrimSpace(info.PwaCode)
	info.Permission = strings.TrimSpace(info.Permission)
	info.PermissionLeak = strings.TrimSpace(info.PermissionLeak)
	info.Area = strings.TrimSpace(info.Area)
	info.JobName = strings.TrimSpace(info.JobName)
	info.Division = strings.TrimSpace(info.Division)
	info.Institution = strings.TrimSpace(info.Institution)
	info.Position = strings.TrimSpace(info.Position)
}

func normalizeCreateSessionInput(input *CreateSessionInput) {
	input.TestVersion = strings.TrimSpace(input.TestVersion)
	input.TesterName = strings.TrimSpace(input.TesterName)
	input.UID = strings.TrimSpace(input.UID)
	input.PwaCode = strings.TrimSpace(input.PwaCode)
	input.BranchName = strings.TrimSpace(input.BranchName)
	input.Area = strings.TrimSpace(input.Area)
	input.JobName = strings.TrimSpace(input.JobName)
	input.Division = strings.TrimSpace(input.Division)
	input.Institution = strings.TrimSpace(input.Institution)
	input.Position = strings.TrimSpace(input.Position)
	input.TestDate = strings.TrimSpace(input.TestDate)
	for i := range input.Branches {
		input.Branches[i].PwaCode = strings.TrimSpace(input.Branches[i].PwaCode)
		input.Branches[i].Name = strings.TrimSpace(input.Branches[i].Name)
		input.Branches[i].Zone = strings.TrimSpace(input.Branches[i].Zone)
	}
}

func validateCreateSessionInput(input CreateSessionInput) error {
	if input.TestVersion == "" {
		return errors.New("test_version is required")
	}
	if input.TesterName == "" {
		return errors.New("tester_name is required")
	}
	if _, err := time.Parse("2006-01-02", input.TestDate); err != nil {
		return errors.New("test_date must use YYYY-MM-DD")
	}
	return nil
}

func parseIDPath(path string, prefix string) (int64, string, bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" || strings.HasPrefix(trimmed, prefix) {
		return 0, "", false
	}

	parts := strings.Split(trimmed, "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, "", false
	}
	if len(parts) == 1 {
		return id, "", true
	}
	if len(parts) == 2 {
		return id, parts[1], true
	}
	return 0, "", false
}

func copyRewrittenCookies(w http.ResponseWriter, resp *http.Response) {
	for _, cookie := range resp.Header.Values("Set-Cookie") {
		w.Header().Add("Set-Cookie", rewriteCookiePath(cookie))
	}
}

func rewriteCookiePath(cookie string) string {
	parts := strings.Split(cookie, ";")
	pathFound := false
	for i, part := range parts {
		if strings.EqualFold(strings.TrimSpace(strings.SplitN(part, "=", 2)[0]), "Path") {
			parts[i] = " Path=/"
			pathFound = true
		}
	}
	if !pathFound {
		parts = append(parts, " Path=/")
	}
	return strings.Join(parts, ";")
}

func requestBasePath(r *http.Request) string {
	prefix := strings.TrimRight(strings.TrimSpace(r.Header.Get("X-Forwarded-Prefix")), "/")
	if prefix != "" {
		if strings.HasPrefix(prefix, "/") {
			return prefix
		}
		return "/" + prefix
	}
	path := strings.TrimRight(r.URL.Path, "/")
	for _, suffix := range []string{"/login", "/logout"} {
		if strings.HasSuffix(path, suffix) {
			base := strings.TrimSuffix(path, suffix)
			if base == "" {
				return ""
			}
			return base
		}
	}
	return ""
}

func joinBasePath(basePath, target string) string {
	basePath = strings.TrimRight(basePath, "/")
	if basePath == "" {
		return "/" + strings.TrimLeft(target, "/")
	}
	return basePath + "/" + strings.TrimLeft(target, "/")
}
func readJSON(r *http.Request, dest any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dest)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if s.sessionInfoURL == "" || path == "/" || path == "/login" || path == "/logout" || path == "/login.html" || path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}
		_, err := s.loadSessionInfo(r)
		if err != nil {
			if strings.HasPrefix(path, "/api/") {
				writeError(w, http.StatusUnauthorized, "login required")
			} else {
				http.Redirect(w, r, joinBasePath(requestBasePath(r), "login"), http.StatusFound)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) loadSessionInfo(r *http.Request) (SessionInfo, error) {
	if s.sessionInfoURL == "" {
		return SessionInfo{}, errors.New("session info is not configured")
	}
	u, err := s.resolveSessionInfoURL(r)
	if err != nil {
		return SessionInfo{}, err
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		return SessionInfo{}, err
	}
	copyForwardHeaders(req, r)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return SessionInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SessionInfo{}, errors.New("login required")
	}
	var info SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return SessionInfo{}, err
	}
	normalizeSessionInfo(&info)
	if info.UName == "" {
		return SessionInfo{}, errors.New("session info missing uname")
	}
	return info, nil
}

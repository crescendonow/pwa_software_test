package uat

import "time"

type Session struct {
	ID          int64     `json:"id"`
	TestVersion string    `json:"test_version"`
	TesterName  string    `json:"tester_name"`
	UID         string    `json:"uid"`
	PwaCode     string    `json:"pwa_code"`
	BranchName  string    `json:"branch_name"`
	Area        string    `json:"area"`
	JobName     string    `json:"job_name"`
	Division    string    `json:"division"`
	Institution string    `json:"institution"`
	Position    string    `json:"position"`
	TestDate    string    `json:"test_date"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type TestCase struct {
	ID             int64  `json:"id"`
	TestVersion    string `json:"test_version"`
	TestSuite      string `json:"test_suite"`
	LayerName      string `json:"layer_name"`
	FeatureChanges string `json:"feature_changes"`
	CaseGroup      string `json:"case_group"`
	TestAction     string `json:"test_action"`
	SortOrder      int    `json:"sort_order"`
	IsActive       bool   `json:"is_active"`
}

type Result struct {
	ID         int64     `json:"id"`
	SessionID  int64     `json:"session_id"`
	TestCaseID int64     `json:"test_case_id"`
	IsPassed   bool      `json:"is_passed"`
	IsFailed   bool      `json:"is_failed"`
	Comment    string    `json:"comment"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
	TestCase   TestCase  `json:"test_case,omitempty"`
}

type Summary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Pending int `json:"pending"`
}

type SessionResults struct {
	Session Session  `json:"session"`
	Results []Result `json:"results"`
	Summary Summary  `json:"summary"`
}

type ReferenceData struct {
	TestVersions   []string `json:"test_versions"`
	TestSuites     []string `json:"test_suites"`
	LayerNames     []string `json:"layer_names"`
	FeatureChanges []string `json:"feature_changes"`
	TestActions    []string `json:"test_actions"`
}

type SessionInfo struct {
	Status         string `json:"status,omitempty"`
	UID            string `json:"uid"`
	UName          string `json:"uname"`
	PwaCode        string `json:"pwa_code"`
	Permission     string `json:"permission,omitempty"`
	PermissionLeak string `json:"permission_leak,omitempty"`
	Area           string `json:"area"`
	JobName        string `json:"job_name"`
	Division       string `json:"division"`
	Institution    string `json:"institution"`
	Position       string `json:"position"`
}

type SessionFilters struct {
	Area        string
	TestVersion string
	TestSuite   string
	DateFrom    string
	DateTo      string
	TesterName  string
}

type ReportFilters struct {
	SessionID   int64
	Area        string
	TestVersion string
	TestSuite   string
	DateFrom    string
	DateTo      string
	TesterName  string
}

type CreateSessionInput struct {
	TestVersion string        `json:"test_version"`
	TesterName  string        `json:"tester_name"`
	UID         string        `json:"uid"`
	PwaCode     string        `json:"pwa_code"`
	BranchName  string        `json:"branch_name"`
	Area        string        `json:"area"`
	JobName     string        `json:"job_name"`
	Division    string        `json:"division"`
	Institution string        `json:"institution"`
	Position    string        `json:"position"`
	TestDate    string        `json:"test_date"`
	Branches    []BranchInput `json:"branches,omitempty"`
}

// BranchInput represents one selected pwa_code/branch to create a session for
// when a tester chooses to run UAT against multiple branches at once.
type BranchInput struct {
	PwaCode string `json:"pwa_code"`
	Name    string `json:"name"`
	Zone    string `json:"zone"`
}

// OfficeInfo mirrors the pwa_gis_tracking office proxy shape used to populate
// the multi-branch picker on the create-session form.
type OfficeInfo struct {
	PwaCode string `json:"pwa_code"`
	Name    string `json:"name"`
	Zone    string `json:"zone"`
}

type UpdateResultInput struct {
	IsPassed  bool   `json:"is_passed"`
	IsFailed  bool   `json:"is_failed"`
	Comment   string `json:"comment"`
	TestSuite string `json:"test_suite"`
}

type ReportRow struct {
	SessionID      int64  `json:"session_id"`
	ResultID       int64  `json:"result_id"`
	TestCaseID     int64  `json:"test_case_id"`
	SortOrder      int    `json:"sort_order"`
	TestDate       string `json:"test_date"`
	TestVersion    string `json:"test_version"`
	TestSuite      string `json:"test_suite"`
	TesterName     string `json:"tester_name"`
	UID            string `json:"uid"`
	PwaCode        string `json:"pwa_code"`
	Area           string `json:"area"`
	JobName        string `json:"job_name"`
	Division       string `json:"division"`
	Institution    string `json:"institution"`
	Position       string `json:"position"`
	LayerName      string `json:"layer_name"`
	FeatureChanges string `json:"feature_changes"`
	CaseGroup      string `json:"case_group"`
	TestAction     string `json:"test_action"`
	IsPassed       bool   `json:"is_passed"`
	IsFailed       bool   `json:"is_failed"`
	Comment        string `json:"comment"`
}

// DashboardFilters narrows the aggregate dashboard summary / word cloud data.
type DashboardFilters struct {
	Area        string
	TestVersion string
	TestSuite   string
	DateFrom    string
	DateTo      string
}

// DashboardSummary aggregates pass/fail/pending counts overall and grouped by
// layer_name / test_suite for the realtime dashboard.
type DashboardSummary struct {
	Total         int                  `json:"total"`
	Passed        int                  `json:"passed"`
	Failed        int                  `json:"failed"`
	Pending       int                  `json:"pending"`
	PercentPassed float64              `json:"percent_passed"`
	ByLayer       []DashboardBreakdown `json:"by_layer"`
	BySuite       []DashboardBreakdown `json:"by_suite"`
	GeneratedAt   time.Time            `json:"generated_at"`
}

// DashboardBreakdown is one row of a group-by aggregate (layer or suite).
type DashboardBreakdown struct {
	Key           string  `json:"key"`
	Total         int     `json:"total"`
	Passed        int     `json:"passed"`
	Failed        int     `json:"failed"`
	Pending       int     `json:"pending"`
	PercentPassed float64 `json:"percent_passed"`
}

// WordCloudItem is one token/weight pair returned by the NLP microservice.
type WordCloudItem struct {
	Word   string `json:"word"`
	Weight int    `json:"weight"`
}

type FreeTextQueryInput struct {
	Prompt string `json:"prompt"`
}

type FreeTextQueryResponse struct {
	Status    string           `json:"status"`
	Answer    string           `json:"answer"`
	Columns   []string         `json:"columns"`
	Rows      []map[string]any `json:"rows"`
	RowCount  int              `json:"row_count"`
	Truncated bool             `json:"truncated"`
}

func summarize(results []Result) Summary {
	summary := Summary{Total: len(results)}
	for _, result := range results {
		switch {
		case result.IsPassed:
			summary.Passed++
		case result.IsFailed:
			summary.Failed++
		default:
			summary.Pending++
		}
	}
	return summary
}

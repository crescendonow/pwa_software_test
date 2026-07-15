package uat

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &PostgresStore{db: pool}, nil
}

func (p *PostgresStore) Close() {
	p.db.Close()
}

func (p *PostgresStore) Health(ctx context.Context) error {
	return p.db.Ping(ctx)
}

func (p *PostgresStore) ListReferences(ctx context.Context) (ReferenceData, error) {
	var refs ReferenceData
	var err error
	refs.TestVersions, err = p.listText(ctx, `SELECT test_version_name FROM ut_logs.ref_test_version ORDER BY tv_id`)
	if err != nil {
		return ReferenceData{}, err
	}
	refs.TestSuites, err = p.listText(ctx, `
		SELECT test_suite FROM ut_logs.test_cases
		WHERE test_suite <> ''
		GROUP BY test_suite
		ORDER BY MIN(sort_order)
	`)
	if err != nil {
		return ReferenceData{}, err
	}
	refs.LayerNames, err = p.listText(ctx, `SELECT layer_name FROM ut_logs.ref_layer_name ORDER BY ly_id`)
	if err != nil {
		return ReferenceData{}, err
	}
	refs.FeatureChanges, err = p.listText(ctx, `SELECT detail_changes FROM ut_logs.ref_feature_changes ORDER BY fc_id`)
	if err != nil {
		return ReferenceData{}, err
	}
	refs.TestActions, err = p.listText(ctx, `SELECT detail_action FROM ut_logs.ref_test_action ORDER BY ta_id`)
	if err != nil {
		return ReferenceData{}, err
	}
	return refs, nil
}

func (p *PostgresStore) listText(ctx context.Context, sql string) ([]string, error) {
	rows, err := p.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (p *PostgresStore) ListTestCases(ctx context.Context, testSuite string) ([]TestCase, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, test_version, test_suite, layer_name, feature_changes, case_group, test_action, sort_order, is_active
		FROM ut_logs.test_cases
		WHERE is_active = TRUE
		  AND ($1 = '' OR test_suite = $1)
		ORDER BY test_version, sort_order, id
	`, testSuite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cases := []TestCase{}
	for rows.Next() {
		var testCase TestCase
		if err := rows.Scan(
			&testCase.ID,
			&testCase.TestVersion,
			&testCase.TestSuite,
			&testCase.LayerName,
			&testCase.FeatureChanges,
			&testCase.CaseGroup,
			&testCase.TestAction,
			&testCase.SortOrder,
			&testCase.IsActive,
		); err != nil {
			return nil, err
		}
		cases = append(cases, testCase)
	}
	return cases, rows.Err()
}

func (p *PostgresStore) ListSessions(ctx context.Context, filters SessionFilters) ([]Session, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, test_version, tester_name, uid, pwa_code, branch_name, area, job_name, division, institution, position,
			test_date::text, created_at, updated_at
		FROM ut_logs.test_sessions
		WHERE ($1 = '' OR area = $1)
		  AND ($2 = '' OR tester_name = $2)
		  AND ($3 = '' OR test_version = $3)
		  AND ($4::date IS NULL OR test_date >= $4::date)
		  AND ($5::date IS NULL OR test_date <= $5::date)
		  AND ($6 = '' OR uid = $6)
		ORDER BY test_date DESC, id DESC
	`, filters.Area, filters.TesterName, filters.TestVersion, nullableDate(filters.DateFrom), nullableDate(filters.DateTo), filters.UID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []Session{}
	for rows.Next() {
		var session Session
		if err := scanSession(rows, &session); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (p *PostgresStore) CreateSession(ctx context.Context, input CreateSessionInput) (Session, error) {
	tx, err := p.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)

	var session Session
	if err := tx.QueryRow(ctx, `
		INSERT INTO ut_logs.test_sessions (
			test_version, tester_name, uid, pwa_code, branch_name, area, job_name, division, institution, position, test_date
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::date)
		ON CONFLICT (test_version, tester_name, test_date, pwa_code) DO UPDATE SET
			uid = EXCLUDED.uid,
			branch_name = EXCLUDED.branch_name,
			area = EXCLUDED.area,
			job_name = EXCLUDED.job_name,
			division = EXCLUDED.division,
			institution = EXCLUDED.institution,
			position = EXCLUDED.position,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, test_version, tester_name, uid, pwa_code, branch_name, area, job_name, division, institution, position,
			test_date::text, created_at, updated_at
	`, input.TestVersion, input.TesterName, input.UID, input.PwaCode, input.BranchName, input.Area, input.JobName, input.Division, input.Institution, input.Position, input.TestDate).Scan(
		&session.ID,
		&session.TestVersion,
		&session.TesterName,
		&session.UID,
		&session.PwaCode,
		&session.BranchName,
		&session.Area,
		&session.JobName,
		&session.Division,
		&session.Institution,
		&session.Position,
		&session.TestDate,
		&session.CreatedAt,
		&session.UpdatedAt,
	); err != nil {
		return Session{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO ut_logs.test_results (session_id, test_case_id)
		SELECT $1, id
		FROM ut_logs.test_cases
		WHERE is_active = TRUE
		  AND test_version = $2
		ON CONFLICT (session_id, test_case_id) DO NOTHING
	`, session.ID, session.TestVersion); err != nil {
		return Session{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (p *PostgresStore) GetSessionResults(ctx context.Context, sessionID int64, viewerUID string) (SessionResults, error) {
	session, err := p.getSession(ctx, sessionID, viewerUID)
	if err != nil {
		return SessionResults{}, err
	}

	if err := p.ensureResultsForActiveCases(ctx, session.ID, session.TestVersion); err != nil {
		return SessionResults{}, err
	}

	rows, err := p.db.Query(ctx, `
		SELECT
			r.id, r.session_id, r.test_case_id, r.is_passed, r.is_failed, r.comment, r.created_at, r.updated_at,
			tc.id, tc.test_version, tc.test_suite, tc.layer_name, tc.feature_changes, tc.case_group, tc.test_action, tc.sort_order, tc.is_active
		FROM ut_logs.test_results r
		JOIN ut_logs.test_cases tc ON tc.id = r.test_case_id
		WHERE r.session_id = $1
		ORDER BY tc.sort_order, tc.id
	`, sessionID)
	if err != nil {
		return SessionResults{}, err
	}
	defer rows.Close()

	results := []Result{}
	for rows.Next() {
		var result Result
		if err := rows.Scan(
			&result.ID,
			&result.SessionID,
			&result.TestCaseID,
			&result.IsPassed,
			&result.IsFailed,
			&result.Comment,
			&result.CreatedAt,
			&result.UpdatedAt,
			&result.TestCase.ID,
			&result.TestCase.TestVersion,
			&result.TestCase.TestSuite,
			&result.TestCase.LayerName,
			&result.TestCase.FeatureChanges,
			&result.TestCase.CaseGroup,
			&result.TestCase.TestAction,
			&result.TestCase.SortOrder,
			&result.TestCase.IsActive,
		); err != nil {
			return SessionResults{}, err
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return SessionResults{}, err
	}

	return SessionResults{
		Session: session,
		Results: results,
		Summary: summarize(results),
	}, nil
}

func (p *PostgresStore) UpdateResult(ctx context.Context, resultID int64, input UpdateResultInput, viewerUID string) (Result, error) {
	var result Result
	err := p.db.QueryRow(ctx, `
		UPDATE ut_logs.test_results AS result
		SET is_passed = $2,
			is_failed = $3,
			comment = COALESCE($4, '')
		FROM ut_logs.test_sessions AS session
		WHERE result.id = $1
		  AND session.id = result.session_id
		  AND ($5 = '' OR session.uid = $5)
		RETURNING result.id, result.session_id, result.test_case_id, result.is_passed, result.is_failed,
			result.comment, result.created_at, result.updated_at
	`, resultID, input.IsPassed, input.IsFailed, input.Comment, viewerUID).Scan(
		&result.ID,
		&result.SessionID,
		&result.TestCaseID,
		&result.IsPassed,
		&result.IsFailed,
		&result.Comment,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrNotFound
		}
		return Result{}, err
	}
	return result, nil
}

func (p *PostgresStore) Report(ctx context.Context, filters ReportFilters) ([]ReportRow, error) {
	rows, err := p.db.Query(ctx, `
		SELECT
			session_id, result_id, test_case_id, sort_order, test_date::text, test_version, test_suite, tester_name,
			uid, pwa_code, area, job_name, division, institution, position,
			layer_name, feature_changes, case_group, test_action, is_passed, is_failed, comment
		FROM ut_logs.v_uat_report
		WHERE ($1::bigint = 0 OR session_id = $1)
		  AND ($2 = '' OR area = $2)
		  AND ($3 = '' OR tester_name = $3)
		  AND ($4 = '' OR test_version = $4)
		  AND ($5 = '' OR test_suite = $5)
		  AND ($6::date IS NULL OR test_date >= $6::date)
		  AND ($7::date IS NULL OR test_date <= $7::date)
		  AND ($8 = '' OR uid = $8)
		ORDER BY test_date DESC, session_id DESC, sort_order, test_case_id
	`, filters.SessionID, filters.Area, filters.TesterName, filters.TestVersion, filters.TestSuite,
		nullableDate(filters.DateFrom), nullableDate(filters.DateTo), filters.UID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	report := []ReportRow{}
	for rows.Next() {
		var row ReportRow
		if err := rows.Scan(
			&row.SessionID,
			&row.ResultID,
			&row.TestCaseID,
			&row.SortOrder,
			&row.TestDate,
			&row.TestVersion,
			&row.TestSuite,
			&row.TesterName,
			&row.UID,
			&row.PwaCode,
			&row.Area,
			&row.JobName,
			&row.Division,
			&row.Institution,
			&row.Position,
			&row.LayerName,
			&row.FeatureChanges,
			&row.CaseGroup,
			&row.TestAction,
			&row.IsPassed,
			&row.IsFailed,
			&row.Comment,
		); err != nil {
			return nil, err
		}
		report = append(report, row)
	}
	return report, rows.Err()
}

// nullableDate converts an empty date filter into SQL NULL so that
// "$n::date IS NULL OR ..." filter clauses treat "no filter" correctly;
// a non-empty value is passed through and cast to date by the query itself.
func nullableDate(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (p *PostgresStore) getSession(ctx context.Context, sessionID int64, viewerUID string) (Session, error) {
	var session Session
	err := p.db.QueryRow(ctx, `
		SELECT id, test_version, tester_name, uid, pwa_code, branch_name, area, job_name, division, institution, position,
			test_date::text, created_at, updated_at
		FROM ut_logs.test_sessions
		WHERE id = $1
		  AND ($2 = '' OR uid = $2)
	`, sessionID, viewerUID).Scan(
		&session.ID,
		&session.TestVersion,
		&session.TesterName,
		&session.UID,
		&session.PwaCode,
		&session.BranchName,
		&session.Area,
		&session.JobName,
		&session.Division,
		&session.Institution,
		&session.Position,
		&session.TestDate,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}
	return session, nil
}

func (p *PostgresStore) ensureResultsForActiveCases(ctx context.Context, sessionID int64, testVersion string) error {
	_, err := p.db.Exec(ctx, `
		INSERT INTO ut_logs.test_results (session_id, test_case_id)
		SELECT $1, id
		FROM ut_logs.test_cases
		WHERE is_active = TRUE
		  AND test_version = $2
		ON CONFLICT (session_id, test_case_id) DO NOTHING
	`, sessionID, testVersion)
	return err
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(scanner sessionScanner, session *Session) error {
	return scanner.Scan(
		&session.ID,
		&session.TestVersion,
		&session.TesterName,
		&session.UID,
		&session.PwaCode,
		&session.BranchName,
		&session.Area,
		&session.JobName,
		&session.Division,
		&session.Institution,
		&session.Position,
		&session.TestDate,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
}

func (p *PostgresStore) DeleteSession(ctx context.Context, sessionID int64, uid string) error {
	var owner string
	err := p.db.QueryRow(ctx, `SELECT uid FROM ut_logs.test_sessions WHERE id=$1`, sessionID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != uid {
		return ErrForbidden
	}
	_, err = p.db.Exec(ctx, `DELETE FROM ut_logs.test_sessions WHERE id=$1`, sessionID)
	return err
}

func (p *PostgresStore) DashboardSummary(ctx context.Context, filters DashboardFilters) (DashboardSummary, error) {
	summary := DashboardSummary{GeneratedAt: time.Now()}

	err := p.db.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN is_passed THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_failed THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN NOT is_passed AND NOT is_failed THEN 1 ELSE 0 END), 0)
		FROM ut_logs.v_uat_report
		WHERE ($1 = '' OR area = $1)
		  AND ($2 = '' OR test_version = $2)
		  AND ($3 = '' OR test_suite = $3)
		  AND ($4::date IS NULL OR test_date >= $4::date)
		  AND ($5::date IS NULL OR test_date <= $5::date)
	`, filters.Area, filters.TestVersion, filters.TestSuite, nullableDate(filters.DateFrom), nullableDate(filters.DateTo)).
		Scan(&summary.Total, &summary.Passed, &summary.Failed, &summary.Pending)
	if err != nil {
		return DashboardSummary{}, err
	}
	summary.PercentPassed = percentOf(summary.Passed, summary.Total)

	summary.ByLayer, err = p.dashboardBreakdown(ctx, "layer_name", filters)
	if err != nil {
		return DashboardSummary{}, err
	}
	summary.BySuite, err = p.dashboardBreakdown(ctx, "test_suite", filters)
	if err != nil {
		return DashboardSummary{}, err
	}
	return summary, nil
}

// dashboardBreakdown groups v_uat_report by the given column name (must be a
// trusted, hard-coded identifier -- never derived from user input).
func (p *PostgresStore) dashboardBreakdown(ctx context.Context, column string, filters DashboardFilters) ([]DashboardBreakdown, error) {
	if column != "layer_name" && column != "test_suite" {
		return nil, errors.New("unsupported breakdown column")
	}
	rows, err := p.db.Query(ctx, `
		SELECT
			`+column+` AS key,
			COUNT(*),
			COALESCE(SUM(CASE WHEN is_passed THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_failed THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN NOT is_passed AND NOT is_failed THEN 1 ELSE 0 END), 0)
		FROM ut_logs.v_uat_report
		WHERE ($1 = '' OR area = $1)
		  AND ($2 = '' OR test_version = $2)
		  AND ($3 = '' OR test_suite = $3)
		  AND ($4::date IS NULL OR test_date >= $4::date)
		  AND ($5::date IS NULL OR test_date <= $5::date)
		  AND `+column+` <> ''
		GROUP BY `+column+`
		ORDER BY `+column+`
	`, filters.Area, filters.TestVersion, filters.TestSuite, nullableDate(filters.DateFrom), nullableDate(filters.DateTo))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	breakdown := []DashboardBreakdown{}
	for rows.Next() {
		var row DashboardBreakdown
		if err := rows.Scan(&row.Key, &row.Total, &row.Passed, &row.Failed, &row.Pending); err != nil {
			return nil, err
		}
		row.PercentPassed = percentOf(row.Passed, row.Total)
		breakdown = append(breakdown, row)
	}
	return breakdown, rows.Err()
}

func (p *PostgresStore) ListComments(ctx context.Context, filters DashboardFilters) ([]string, error) {
	rows, err := p.db.Query(ctx, `
		SELECT comment
		FROM ut_logs.v_uat_report
		WHERE comment <> ''
		  AND ($1 = '' OR area = $1)
		  AND ($2 = '' OR test_version = $2)
		  AND ($3 = '' OR test_suite = $3)
		  AND ($4::date IS NULL OR test_date >= $4::date)
		  AND ($5::date IS NULL OR test_date <= $5::date)
		ORDER BY session_id DESC, sort_order
	`, filters.Area, filters.TestVersion, filters.TestSuite, nullableDate(filters.DateFrom), nullableDate(filters.DateTo))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := []string{}
	for rows.Next() {
		var comment string
		if err := rows.Scan(&comment); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func percentOf(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

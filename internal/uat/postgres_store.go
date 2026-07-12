package uat

import (
	"context"
	"errors"

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
	_ = testSuite
	rows, err := p.db.Query(ctx, `
		SELECT id, test_version, layer_name, feature_changes, case_group, test_action, sort_order, is_active
		FROM ut_logs.test_cases
		WHERE is_active = TRUE
		ORDER BY test_version, sort_order, id
	`)
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
		SELECT id, test_version, tester_name, uid, pwa_code, area, job_name, division, institution, position,
			test_date::text, created_at, updated_at
		FROM ut_logs.test_sessions
		WHERE ($1 = '' OR area = $1)
		  AND ($2 = '' OR tester_name = $2)
		ORDER BY test_date DESC, id DESC
	`, filters.Area, filters.TesterName)
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
			test_version, tester_name, uid, pwa_code, area, job_name, division, institution, position, test_date
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::date)
		ON CONFLICT (test_version, tester_name, test_date) DO UPDATE SET
			uid = EXCLUDED.uid,
			pwa_code = EXCLUDED.pwa_code,
			area = EXCLUDED.area,
			job_name = EXCLUDED.job_name,
			division = EXCLUDED.division,
			institution = EXCLUDED.institution,
			position = EXCLUDED.position,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, test_version, tester_name, uid, pwa_code, area, job_name, division, institution, position,
			test_date::text, created_at, updated_at
	`, input.TestVersion, input.TesterName, input.UID, input.PwaCode, input.Area, input.JobName, input.Division, input.Institution, input.Position, input.TestDate).Scan(
		&session.ID,
		&session.TestVersion,
		&session.TesterName,
		&session.UID,
		&session.PwaCode,
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

func (p *PostgresStore) GetSessionResults(ctx context.Context, sessionID int64) (SessionResults, error) {
	session, err := p.getSession(ctx, sessionID)
	if err != nil {
		return SessionResults{}, err
	}

	if err := p.ensureResultsForActiveCases(ctx, session.ID, session.TestVersion); err != nil {
		return SessionResults{}, err
	}

	rows, err := p.db.Query(ctx, `
		SELECT
			r.id, r.session_id, r.test_case_id, r.is_passed, r.is_failed, r.comment, r.created_at, r.updated_at,
			tc.id, tc.test_version, tc.layer_name, tc.feature_changes, tc.case_group, tc.test_action, tc.sort_order, tc.is_active
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

func (p *PostgresStore) UpdateResult(ctx context.Context, resultID int64, input UpdateResultInput) (Result, error) {
	var result Result
	err := p.db.QueryRow(ctx, `
		UPDATE ut_logs.test_results
		SET is_passed = $2,
			is_failed = $3,
			comment = COALESCE($4, '')
		WHERE id = $1
		RETURNING id, session_id, test_case_id, is_passed, is_failed, comment, created_at, updated_at
	`, resultID, input.IsPassed, input.IsFailed, input.Comment).Scan(
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
			session_id, result_id, test_case_id, sort_order, test_date::text, test_version, tester_name,
			uid, pwa_code, area, job_name, division, institution, position,
			layer_name, feature_changes, case_group, test_action, is_passed, is_failed, comment
		FROM ut_logs.v_uat_report
		WHERE ($1::bigint = 0 OR session_id = $1)
		  AND ($2 = '' OR area = $2)
		  AND ($3 = '' OR tester_name = $3)
		ORDER BY test_date DESC, session_id DESC, sort_order, test_case_id
	`, filters.SessionID, filters.Area, filters.TesterName)
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

func (p *PostgresStore) getSession(ctx context.Context, sessionID int64) (Session, error) {
	var session Session
	err := p.db.QueryRow(ctx, `
		SELECT id, test_version, tester_name, uid, pwa_code, area, job_name, division, institution, position,
			test_date::text, created_at, updated_at
		FROM ut_logs.test_sessions
		WHERE id = $1
	`, sessionID).Scan(
		&session.ID,
		&session.TestVersion,
		&session.TesterName,
		&session.UID,
		&session.PwaCode,
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

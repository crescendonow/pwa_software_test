-- Add a free-text UAT row for requirements that are not covered by the
-- predefined test cases. The user's text is stored in test_results.comment.

INSERT INTO ut_logs.ref_layer_name (layer_name)
VALUES ('additional_requirement')
ON CONFLICT (layer_name) DO NOTHING;

INSERT INTO ut_logs.ref_feature_changes (detail_changes)
VALUES ('ความต้องการเพิ่มเติมจากผู้ทดสอบ')
ON CONFLICT (detail_changes) DO NOTHING;

INSERT INTO ut_logs.ref_test_action (detail_action)
VALUES ('กรอกรายละเอียดความต้องการเพิ่มเติมในช่องหมายเหตุ')
ON CONFLICT (detail_action) DO NOTHING;

-- One row per version in ref_test_version, rather than hard-coded version
-- names: test_version is FK'd to ref_test_version ON UPDATE CASCADE, so a
-- version rename silently rewrites test_cases and test_sessions. Hard-coded
-- names go stale the moment that happens ('5 พฤษภาคม 2569' was renamed to
-- '6 กรกฎาคม 2569' and no longer exists). Driving off the ref table keeps this
-- correct across renames and picks up future versions automatically.
--
-- sort_order 1130 + tv_id * 10 stays above the seeded range (max 1130), so the
-- row lands last in every session, and is stable per version across re-runs.
INSERT INTO ut_logs.test_cases (
    sort_order,
    test_version,
    test_suite,
    layer_name,
    feature_changes,
    case_group,
    test_action,
    is_active
)
SELECT
    1130 + version.tv_id * 10,
    version.test_version_name,
    'ความต้องการเพิ่มเติม',
    'additional_requirement',
    'ความต้องการเพิ่มเติมจากผู้ทดสอบ',
    '',
    'กรอกรายละเอียดความต้องการเพิ่มเติมในช่องหมายเหตุ',
    TRUE
FROM ut_logs.ref_test_version AS version
ON CONFLICT (sort_order) DO UPDATE SET
    test_version = EXCLUDED.test_version,
    test_suite = EXCLUDED.test_suite,
    layer_name = EXCLUDED.layer_name,
    feature_changes = EXCLUDED.feature_changes,
    case_group = EXCLUDED.case_group,
    test_action = EXCLUDED.test_action,
    is_active = EXCLUDED.is_active;

-- Existing sessions also need a result row; new sessions are covered by the
-- normal CreateSession/ensureResultsForActiveCases flow.
INSERT INTO ut_logs.test_results (session_id, test_case_id)
SELECT session.id, test_case.id
FROM ut_logs.test_sessions AS session
JOIN ut_logs.test_cases AS test_case
    ON test_case.test_version = session.test_version
   AND test_case.layer_name = 'additional_requirement'
   AND test_case.is_active = TRUE
ON CONFLICT (session_id, test_case_id) DO NOTHING;

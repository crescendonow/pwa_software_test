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
VALUES
    (
        1140,
        '22 เมษายน 2569',
        '',
        'additional_requirement',
        'ความต้องการเพิ่มเติมจากผู้ทดสอบ',
        '',
        'กรอกรายละเอียดความต้องการเพิ่มเติมในช่องหมายเหตุ',
        TRUE
    ),
    (
        1150,
        '5 พฤษภาคม 2569',
        '5 พ.ค. 69',
        'additional_requirement',
        'ความต้องการเพิ่มเติมจากผู้ทดสอบ',
        '',
        'กรอกรายละเอียดความต้องการเพิ่มเติมในช่องหมายเหตุ',
        TRUE
    )
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

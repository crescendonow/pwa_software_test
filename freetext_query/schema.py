"""The UAT schema exposed to the language model.

The audit table is deliberately omitted: it contains prompts and generated
SQL and must not become queryable through the user-facing assistant.
"""

SCHEMA_DESCRIPTION = """
Database: PostgreSQL schema ut_logs. The service may read only these UAT objects:

- ref_test_version(tv_id, test_version_name)
- ref_layer_name(ly_id, layer_name)
- ref_feature_changes(fc_id, detail_changes)
- ref_test_action(ta_id, detail_action)
- test_sessions(id, test_version, tester_name, uid, pwa_code, area, job_name,
  division, institution, position, branch_name, test_date, created_at, updated_at)
- test_cases(id, test_version, test_suite, layer_name, feature_changes,
  case_group, test_action, sort_order, is_active, created_at)
- test_results(id, session_id, test_case_id, is_passed, is_failed, comment,
  created_at, updated_at)
- v_uat_report(session_id, result_id, test_case_id, sort_order, test_date,
  test_version, test_suite, tester_name, uid, pwa_code, area, job_name,
  division, institution, position, layer_name, feature_changes, case_group,
  test_action, is_passed, is_failed, comment)

Relationships: test_results.session_id joins test_sessions.id and
test_results.test_case_id joins test_cases.id. Use v_uat_report for most
reporting questions. Dates are PostgreSQL DATE values. is_passed and
is_failed are booleans; rows with both false are pending.
""".strip()

ALLOWED_RELATIONS = {
    "ut_logs.ref_test_version",
    "ut_logs.ref_layer_name",
    "ut_logs.ref_feature_changes",
    "ut_logs.ref_test_action",
    "ut_logs.test_sessions",
    "ut_logs.test_cases",
    "ut_logs.test_results",
    "ut_logs.v_uat_report",
}

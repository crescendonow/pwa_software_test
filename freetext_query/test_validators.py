import pytest

from .validators import UnsafeQuery, validate_sql


def test_valid_query_is_limited_to_100_rows():
    sql = validate_sql("SELECT test_version, COUNT(*) AS total FROM ut_logs.v_uat_report GROUP BY test_version")
    assert sql.endswith("LIMIT 100")


def test_existing_large_limit_is_reduced():
    sql = validate_sql("SELECT * FROM ut_logs.v_uat_report LIMIT 1000")
    assert "LIMIT 100" in sql
    assert "LIMIT 1000" not in sql


def test_cte_aliases_can_be_used_but_only_allowed_source_relations_are_read():
    sql = validate_sql("WITH report AS (SELECT test_version FROM ut_logs.v_uat_report) SELECT * FROM report")
    assert sql.endswith("LIMIT 100")


def test_limit_is_inserted_before_trailing_sql_comment():
    sql = validate_sql("SELECT * FROM ut_logs.v_uat_report -- note")
    assert "LIMIT 100 -- note" in sql


@pytest.mark.parametrize("sql", [
    "DELETE FROM ut_logs.test_results",
    "UPDATE ut_logs.test_results SET comment = 'x'",
    "INSERT INTO ut_logs.test_results DEFAULT VALUES",
    "DROP TABLE ut_logs.test_results",
    "SELECT * FROM ut_logs.test_results; DELETE FROM ut_logs.test_results",
    "SELECT * FROM public.users",
    "SELECT pg_read_file('/etc/passwd')",
    "SELECT * INTO ut_logs.copy_results FROM ut_logs.test_results",
    "SELECT * FROM ut_logs.test_results FOR UPDATE",
    "SELECT nextval('ut_logs.sequence_id')",
    "SELECT 1",
])
def test_mutating_multiple_or_outside_schema_queries_are_rejected(sql):
    with pytest.raises(UnsafeQuery):
        validate_sql(sql)

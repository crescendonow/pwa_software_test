import asyncio

import pytest

from .gemini_client import GeminiError
from .service import QueryService


class FakeLLM:
    def __init__(self, response=None, error=None):
        self.response = response
        self.error = error

    async def generate_query(self, prompt):
        if self.error:
            raise self.error
        return self.response


class FakeDatabase:
    def __init__(self, result=(['total'], [{'total': 1}], False)):
        self.result = result
        self.executed = []
        self.audits = []

    def execute_read_only(self, sql):
        self.executed.append(sql)
        return self.result

    def insert_audit(self, **kwargs):
        self.audits.append(kwargs)


def test_query_returns_result_and_audits_without_result_rows():
    database = FakeDatabase()
    service = QueryService(
        llm=FakeLLM({'sql': 'SELECT COUNT(*) AS total FROM ut_logs.v_uat_report', 'answer': 'พบข้อมูล'}),
        database=database,
    )

    result = asyncio.run(service.query('มีกี่รายการ', '14180', 'Tester One'))

    assert result.status == 'success'
    assert result.rows == [{'total': 1}]
    assert database.executed[0].endswith('LIMIT 100')
    assert database.audits[0]['uid'] == '14180'
    assert database.audits[0]['status'] == 'success'
    assert 'rows' not in database.audits[0]


def test_unsafe_generated_sql_is_rejected_before_database_execution():
    database = FakeDatabase()
    service = QueryService(
        llm=FakeLLM({'sql': 'DELETE FROM ut_logs.test_results', 'answer': ''}),
        database=database,
    )

    result = asyncio.run(service.query('ลบข้อมูลทั้งหมด', '14180', 'Tester One'))

    assert result.status == 'rejected'
    assert database.executed == []
    assert database.audits[0]['status'] == 'rejected'
    assert database.audits[0]['generated_sql'].startswith('DELETE')


def test_query_response_is_capped_at_100_rows_even_if_database_returns_more():
    database = FakeDatabase(result=(['id'], [{'id': value} for value in range(101)], False))
    service = QueryService(
        llm=FakeLLM({'sql': 'SELECT id FROM ut_logs.test_results', 'answer': 'พบข้อมูล'}),
        database=database,
    )

    result = asyncio.run(service.query('แสดงรายการ', '14180', 'Tester One'))

    assert len(result.rows) == 100
    assert result.row_count == 100
    assert result.truncated is True


def test_llm_failure_is_audited_and_returned_to_api_layer():
    database = FakeDatabase()
    service = QueryService(
        llm=FakeLLM(error=GeminiError('timeout')),
        database=database,
    )

    with pytest.raises(GeminiError):
        asyncio.run(service.query('สรุปผล', '14180', 'Tester One'))

    assert database.executed == []
    assert database.audits[0]['status'] == 'error'

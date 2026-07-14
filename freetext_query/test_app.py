import asyncio

import httpx

from . import app as app_module
from .models import QueryResponse


class FakeService:
    async def query(self, prompt, uid, uname):
        assert prompt == "สรุปผล"
        assert uid == "14180"
        assert uname == "Tester One"
        return QueryResponse(status="success", answer="พบข้อมูล", columns=["total"], rows=[{"total": 1}], row_count=1)


async def _post_query():
    transport = httpx.ASGITransport(app=app_module.app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        return await client.post("/query", json={"prompt": "สรุปผล", "actor": {"uid": "14180", "uname": "Tester One"}})


def test_query_endpoint_returns_public_result_contract_without_sql():
    original = app_module._service
    app_module._service = FakeService()
    try:
        response = asyncio.run(_post_query())
    finally:
        app_module._service = original

    assert response.status_code == 200
    payload = response.json()
    assert payload["answer"] == "พบข้อมูล"
    assert payload["rows"] == [{"total": 1}]
    assert "sql" not in payload


def test_query_endpoint_requires_actor_uid():
    response = asyncio.run(_post_query_without_actor())
    assert response.status_code == 400


async def _post_query_without_actor():
    transport = httpx.ASGITransport(app=app_module.app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
        return await client.post("/query", json={"prompt": "เธชเธฃเธธเธเธเธฅ"})

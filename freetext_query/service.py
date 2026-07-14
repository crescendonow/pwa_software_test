from __future__ import annotations

import time
from typing import Any

from .config import MAX_ROWS
from .database import Database, DatabaseError
from .gemini_client import GeminiClient, GeminiError
from .models import QueryResponse
from .validators import UnsafeQuery, validate_sql


class QueryService:
    def __init__(self, *, llm: GeminiClient, database: Database):
        self.llm = llm
        self.database = database

    async def query(self, prompt: str, uid: str, uname: str) -> QueryResponse:
        started = time.monotonic()
        generated_sql: str | None = None
        try:
            generated = await self.llm.generate_query(prompt)
            generated_sql = str(generated.get("sql", ""))
            safe_sql = validate_sql(generated_sql)
            columns, rows, truncated = self.database.execute_read_only(safe_sql)
            truncated = truncated or len(rows) > MAX_ROWS
            rows = rows[:MAX_ROWS]
            answer = str(generated.get("answer", "")).strip() or "พบข้อมูลตามคำถาม"
            self._audit(uid, uname, prompt, generated_sql, "success", "", started)
            return QueryResponse(
                status="success",
                answer=answer,
                columns=columns,
                rows=rows,
                row_count=len(rows),
                truncated=truncated,
            )
        except UnsafeQuery as exc:
            self._audit(uid, uname, prompt, generated_sql, "rejected", str(exc), started)
            return QueryResponse(status="rejected", answer="คำถามนี้สร้างคำสั่งอ่านข้อมูลที่ไม่ปลอดภัย จึงไม่สามารถดำเนินการได้")
        except (GeminiError, DatabaseError) as exc:
            self._audit(uid, uname, prompt, generated_sql, "error", str(exc), started)
            raise
        except Exception as exc:
            self._audit(uid, uname, prompt, generated_sql, "error", str(exc), started)
            raise

    def _audit(self, uid: str, uname: str, prompt: str, sql: str | None, status: str, error: str, started: float) -> None:
        duration_ms = int((time.monotonic() - started) * 1000)
        try:
            self.database.insert_audit(
                uid=uid,
                uname=uname,
                prompt=prompt,
                generated_sql=sql,
                status=status,
                error_message=error,
                duration_ms=duration_ms,
            )
        except Exception:
            # Query outcome must not be replaced by an audit outage. The
            # service logger can still capture the failure at the API layer.
            pass

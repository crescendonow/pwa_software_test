from __future__ import annotations

from datetime import date, datetime
from decimal import Decimal
from typing import Any

from sqlalchemy import create_engine, text

from .config import DATABASE_TIMEOUT_SECONDS, DATABASE_URL, MAX_ROWS


class DatabaseError(RuntimeError):
    pass


def _safe_value(value: Any) -> Any:
    if isinstance(value, (datetime, date)):
        return value.isoformat()
    if isinstance(value, Decimal):
        return float(value)
    if isinstance(value, (bytes, bytearray, memoryview)):
        return bytes(value).hex()
    return value


class Database:
    def __init__(self, url: str = DATABASE_URL):
        if not url:
            raise DatabaseError("DATABASE_URL is not configured")
        self.engine = create_engine(url, pool_pre_ping=True, pool_size=3, max_overflow=1)

    def execute_read_only(self, sql: str) -> tuple[list[str], list[dict[str, Any]], bool]:
        try:
            with self.engine.connect() as connection:
                transaction = connection.begin()
                try:
                    connection.execute(text("SET TRANSACTION READ ONLY"))
                    connection.execute(text(f"SET LOCAL statement_timeout = {DATABASE_TIMEOUT_SECONDS * 1000}"))
                    result = connection.execute(text(sql))
                    columns = list(result.keys())
                    raw_rows = result.fetchmany(MAX_ROWS + 1)
                    transaction.commit()
                except Exception:
                    transaction.rollback()
                    raise
        except Exception as exc:
            raise DatabaseError(str(exc)) from exc

        truncated = len(raw_rows) > MAX_ROWS
        rows = [
            {column: _safe_value(value) for column, value in zip(columns, row)}
            for row in raw_rows[:MAX_ROWS]
        ]
        return columns, rows, truncated

    def insert_audit(self, *, uid: str, uname: str, prompt: str, generated_sql: str | None, status: str, error_message: str, duration_ms: int) -> None:
        with self.engine.begin() as connection:
            connection.execute(
                text(
                    """
                    INSERT INTO ut_logs.freetext_query_audit
                        (uid, uname, prompt, generated_sql, status, error_message, duration_ms)
                    VALUES (:uid, :uname, :prompt, :generated_sql, :status, :error_message, :duration_ms)
                    """
                ),
                {
                    "uid": uid,
                    "uname": uname,
                    "prompt": prompt,
                    "generated_sql": generated_sql,
                    "status": status,
                    "error_message": error_message,
                    "duration_ms": duration_ms,
                },
            )

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field


class Actor(BaseModel):
    uid: str = ""
    uname: str = ""


class QueryRequest(BaseModel):
    prompt: str = Field(..., min_length=1, max_length=500)
    actor: Actor = Field(default_factory=Actor)


class QueryResponse(BaseModel):
    status: str
    answer: str = ""
    columns: list[str] = Field(default_factory=list)
    rows: list[dict[str, Any]] = Field(default_factory=list)
    row_count: int = 0
    truncated: bool = False

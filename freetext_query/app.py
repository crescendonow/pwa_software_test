from __future__ import annotations

import logging

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse

from .config import DATABASE_URL, GEMINI_API_KEY, GEMINI_MODEL, MAX_PROMPT_LENGTH
from .database import Database, DatabaseError
from .gemini_client import GeminiClient, GeminiError
from .models import QueryRequest, QueryResponse
from .service import QueryService


logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(name)s: %(message)s")
log = logging.getLogger("freetext_query")

app = FastAPI(title="GIS Online UAT Free-text Query", version="1.0.0")

_service: QueryService | None = None


def get_service() -> QueryService:
    global _service
    if _service is None:
        _service = QueryService(llm=GeminiClient(), database=Database())
    return _service


@app.get("/healthz")
def healthz():
    database_configured = bool(DATABASE_URL)
    gemini_configured = bool(GEMINI_API_KEY)
    ready = database_configured and gemini_configured
    body = {
        "ok": ready,
        "service": "freetext_query",
        "database_configured": database_configured,
        "gemini_configured": gemini_configured,
        "model": GEMINI_MODEL,
    }
    return JSONResponse(status_code=200 if ready else 503, content=body)


@app.post("/query", response_model=QueryResponse)
async def query(payload: QueryRequest):
    prompt = payload.prompt.strip()
    if not prompt:
        raise HTTPException(status_code=400, detail="prompt is required")
    if len(prompt) > MAX_PROMPT_LENGTH:
        raise HTTPException(status_code=400, detail=f"prompt must be at most {MAX_PROMPT_LENGTH} characters")
    uid = payload.actor.uid.strip()
    if not uid:
        raise HTTPException(status_code=400, detail="actor uid is required")
    try:
        return await get_service().query(prompt, uid, payload.actor.uname.strip())
    except GeminiError as exc:
        log.error("Gemini query failed: %s", exc)
        raise HTTPException(status_code=502, detail="ไม่สามารถประมวลผลคำถามด้วย Gemini ได้") from exc
    except DatabaseError as exc:
        log.error("database query failed: %s", exc)
        raise HTTPException(status_code=502, detail="ไม่สามารถอ่านข้อมูล UAT ได้") from exc


if __name__ == "__main__":
    import uvicorn

    from .config import HOST, PORT

    uvicorn.run("freetext_query.app:app", host=HOST, port=PORT)

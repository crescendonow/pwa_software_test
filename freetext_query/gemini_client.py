from __future__ import annotations

import json
from typing import Any

import httpx

from .config import GEMINI_API_KEY, GEMINI_MODEL, GEMINI_TIMEOUT_SECONDS
from .schema import SCHEMA_DESCRIPTION


class GeminiError(RuntimeError):
    pass


class GeminiClient:
    def __init__(self, *, api_key: str = GEMINI_API_KEY, model: str = GEMINI_MODEL, timeout: float = GEMINI_TIMEOUT_SECONDS):
        self.api_key = api_key
        self.model = model
        self.timeout = timeout

    async def generate_query(self, prompt: str) -> dict[str, Any]:
        if not self.api_key:
            raise GeminiError("GEMINI_API_KEY is not configured")

        instruction = (
            "You convert Thai natural-language questions into one safe PostgreSQL query "
            "for the allowed UAT schema. Return JSON only with keys sql and answer. "
            "The sql must be a single read-only SELECT/WITH statement. Do not invent "
            "tables or columns. Do not include markdown fences.\n\n"
            f"{SCHEMA_DESCRIPTION}\n\nQuestion: {prompt}"
        )
        body = {
            "contents": [{"role": "user", "parts": [{"text": instruction}]}],
            "generationConfig": {
                "responseMimeType": "application/json",
                "responseSchema": {
                    "type": "OBJECT",
                    "properties": {
                        "sql": {"type": "STRING"},
                        "answer": {"type": "STRING"},
                    },
                    "required": ["sql", "answer"],
                },
            },
        }
        url = f"https://generativelanguage.googleapis.com/v1beta/models/{self.model}:generateContent"
        try:
            async with httpx.AsyncClient(timeout=self.timeout) as client:
                response = await client.post(url, headers={"x-goog-api-key": self.api_key}, json=body)
                response.raise_for_status()
        except (httpx.HTTPError, httpx.TimeoutException) as exc:
            raise GeminiError(f"Gemini request failed: {exc}") from exc

        try:
            payload = response.json()
            text = payload["candidates"][0]["content"]["parts"][0]["text"]
            parsed = json.loads(text)
            if not isinstance(parsed, dict):
                raise ValueError("response is not an object")
            return parsed
        except (KeyError, IndexError, TypeError, ValueError, json.JSONDecodeError) as exc:
            raise GeminiError("Gemini returned an invalid structured response") from exc

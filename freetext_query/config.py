from __future__ import annotations

import os
from pathlib import Path

from dotenv import load_dotenv


load_dotenv(Path(__file__).with_name(".env"))

HOST = os.getenv("FREETEXT_QUERY_HOST", "127.0.0.1")
PORT = int(os.getenv("FREETEXT_QUERY_PORT", "5024"))
DATABASE_URL = os.getenv("DATABASE_URL", "")
if DATABASE_URL.startswith("postgres://"):
    DATABASE_URL = "postgresql://" + DATABASE_URL[len("postgres://"):]
GEMINI_API_KEY = os.getenv("GEMINI_API_KEY", "")
GEMINI_MODEL = os.getenv("GEMINI_MODEL", "gemini-3.5-flash")
GEMINI_TIMEOUT_SECONDS = float(os.getenv("GEMINI_TIMEOUT_SECONDS", "30"))
DATABASE_TIMEOUT_SECONDS = int(os.getenv("DATABASE_TIMEOUT_SECONDS", "10"))
MAX_PROMPT_LENGTH = int(os.getenv("FREETEXT_MAX_PROMPT_LENGTH", "500"))
MAX_ROWS = int(os.getenv("FREETEXT_MAX_ROWS", "100"))

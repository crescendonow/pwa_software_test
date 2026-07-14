"""PWA GIS UAT - Thai NLP microservice.

Small internal FastAPI service used only by the Go backend
(internal/uat/http.go handleDashboardWordcloud). It never talks to the
database directly and is never exposed to the browser: the Go side pulls
`comment` values from ut_logs.v_uat_report per the dashboard filters, POSTs
them here, and forwards our JSON response back to the frontend.

Endpoints:
  POST /wordcloud       -> [{"word": str, "weight": int}, ...] (top N tokens)
  POST /wordcloud.png   -> PNG image rendered with the `wordcloud` package
  GET  /healthz         -> {"ok": true}
"""

from __future__ import annotations

import io
import os
import re
from pathlib import Path
from collections import Counter
from typing import List

from fastapi import FastAPI, Response
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

try:
    from dotenv import load_dotenv
    load_dotenv(Path(__file__).with_name(".env"))
except Exception:  # pragma: no cover - optional for the lightweight fallback
    pass

try:
    from pythainlp.corpus import thai_stopwords
    from pythainlp.tokenize import word_tokenize
except Exception:  # pragma: no cover - pythainlp not installed in some envs
    thai_stopwords = None
    word_tokenize = None

try:
    from wordcloud import WordCloud
except Exception:  # pragma: no cover - wordcloud/pillow not installed
    WordCloud = None

app = FastAPI(title="pwa-gis-uat-nlp", version="1.0.0")

TOP_N_WORDS = int(os.environ.get("TOP_N_WORDS", "80"))
THAI_FONT_PATH = os.environ.get("THAI_FONT_PATH", "")

# Extra stopwords / noise tokens that show up a lot in UAT comments but carry
# no useful signal for a word cloud.
EXTRA_STOPWORDS = {
    "", " ", "\n", "\t", "ๆ", "ๆๆ", "ครับ", "ค่ะ", "คะ", "นะ", "จ้า", "จ้ะ",
}

_TOKEN_RE = re.compile(r"[ก-๙a-zA-Z0-9]+")


class CommentsRequest(BaseModel):
    comments: List[str] = Field(default_factory=list)
    top_n: int | None = None


def _stopwords() -> set[str]:
    if thai_stopwords is None:
        return set(EXTRA_STOPWORDS)
    return set(thai_stopwords()) | EXTRA_STOPWORDS


def _tokenize(text: str) -> List[str]:
    if not text:
        return []
    if word_tokenize is not None:
        tokens = word_tokenize(text, engine="newmm")
    else:
        # Fallback: naive whitespace/punctuation split so the endpoint still
        # returns something useful if pythainlp isn't installed.
        tokens = _TOKEN_RE.findall(text)
    return [t.strip() for t in tokens if t.strip()]


def count_words(comments: List[str], top_n: int) -> List[tuple[str, int]]:
    stopwords = _stopwords()
    counter: Counter[str] = Counter()
    for comment in comments:
        for token in _tokenize(comment):
            if len(token) < 2:
                continue
            if token in stopwords:
                continue
            counter[token] += 1
    return counter.most_common(top_n)


@app.get("/healthz")
def healthz():
    return {"ok": True}


@app.post("/wordcloud")
def wordcloud(payload: CommentsRequest):
    top_n = payload.top_n or TOP_N_WORDS
    counted = count_words(payload.comments, top_n)
    return [{"word": word, "weight": weight} for word, weight in counted]


@app.post("/wordcloud.png")
def wordcloud_png(payload: CommentsRequest):
    top_n = payload.top_n or TOP_N_WORDS
    counted = count_words(payload.comments, top_n)

    if WordCloud is None or not counted:
        return Response(status_code=204)

    frequencies = dict(counted)
    wc_kwargs = dict(
        width=1200,
        height=630,
        background_color="white",
        prefer_horizontal=0.9,
    )
    if THAI_FONT_PATH:
        wc_kwargs["font_path"] = THAI_FONT_PATH

    image = WordCloud(**wc_kwargs).generate_from_frequencies(frequencies).to_image()
    buffer = io.BytesIO()
    image.save(buffer, format="PNG")
    buffer.seek(0)
    return StreamingResponse(buffer, media_type="image/png")

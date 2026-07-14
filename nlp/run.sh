#!/usr/bin/env sh
set -eu

cd -- "$(dirname -- "$0")"
python=.venv/bin/python

if [ ! -x "$python" ]; then
  printf '%s\n' 'NLP virtual environment is missing. Create nlp/.venv with Python 3.11 and install nlp/requirements.txt first.' >&2
  exit 1
fi

export TOP_N_WORDS=${TOP_N_WORDS:-80}
exec "$python" -m uvicorn app:app --host "${NLP_HOST:-127.0.0.1}" --port "${NLP_PORT:-5023}"

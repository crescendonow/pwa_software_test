$ErrorActionPreference = 'Stop'

$python = Join-Path $PSScriptRoot '.venv\Scripts\python.exe'
if (-not (Test-Path $python)) {
    throw 'Free-text query virtual environment is missing. Create freetext_query\.venv and install requirements.txt first.'
}

Set-Location (Split-Path $PSScriptRoot -Parent)
& $python -m uvicorn freetext_query.app:app --host 127.0.0.1 --port 5024
exit $LASTEXITCODE

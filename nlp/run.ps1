$ErrorActionPreference = 'Stop'

$python = Join-Path $PSScriptRoot '.venv\Scripts\python.exe'
if (-not (Test-Path $python)) {
    throw 'NLP virtual environment is missing. Create nlp\.venv with Python 3.11 and install nlp\requirements.txt first.'
}

if (-not $env:TOP_N_WORDS) {
    $env:TOP_N_WORDS = '80'
}

$hostAddress = if ($env:NLP_HOST) { $env:NLP_HOST } else { '127.0.0.1' }
$port = if ($env:NLP_PORT) { $env:NLP_PORT } else { '5023' }

Set-Location $PSScriptRoot
& $python -m uvicorn app:app --host $hostAddress --port $port
exit $LASTEXITCODE

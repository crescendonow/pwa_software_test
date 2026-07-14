param(
    [string]$NssmPath = "C:\nssm-2.24\nssm-2.24\win64\nssm.exe",
    [string]$AppRoot = $PSScriptRoot,
    [string]$ServiceUser = "",
    [switch]$Apply
)

$ErrorActionPreference = "Stop"

function Invoke-Nssm {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
    & $NssmPath @Args
}

if (-not (Test-Path -LiteralPath $NssmPath)) {
    throw "NSSM was not found at $NssmPath"
}

if ($Apply -and [string]::IsNullOrWhiteSpace($ServiceUser)) {
    throw "-ServiceUser is required with -Apply. Use a dedicated Windows service account."
}

$envFiles = @(
    (Join-Path $AppRoot "nlp\.env"),
    (Join-Path $AppRoot "freetext_query\.env")
)

function Protect-EnvFile {
    param([string]$Path, [string]$Account)
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "Production environment file is missing: $Path"
    }
    $grant = "{0}:(R)" -f $Account
    & icacls.exe $Path /inheritance:r /grant:r "SYSTEM:(F)" "Administrators:(F)" $grant | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "Could not restrict ACL on $Path"
    }
}

$services = @(
    @{
        Name = "GISOnlineUATWordCloud"
        DisplayName = "GIS Online UAT Word Cloud"
        Description = "Thai word cloud service for GIS Online UAT"
        Python = Join-Path $AppRoot "nlp\.venv\Scripts\python.exe"
        Parameters = "-m uvicorn nlp.app:app --host 127.0.0.1 --port 5023"
        Directory = $AppRoot
        LogDirectory = Join-Path $AppRoot "nlp\logs"
    },
    @{
        Name = "GISOnlineUATFreeTextQuery"
        DisplayName = "GIS Online UAT Free-text Query"
        Description = "Thai free-text to read-only UAT query service"
        Python = Join-Path $AppRoot "freetext_query\.venv\Scripts\python.exe"
        Parameters = "-m uvicorn freetext_query.app:app --host 127.0.0.1 --port 5024"
        Directory = $AppRoot
        LogDirectory = Join-Path $AppRoot "freetext_query\logs"
    }
)

foreach ($service in $services) {
    $stdout = Join-Path $service.LogDirectory "service_stdout.log"
    $stderr = Join-Path $service.LogDirectory "service_stderr.log"
    Write-Host "$($service.Name): $($service.Python) $($service.Parameters)"
    if (-not (Test-Path -LiteralPath $service.Python)) {
        throw "Python virtual environment is missing: $($service.Python)"
    }

    if (-not $Apply) { continue }

    New-Item -ItemType Directory -Force -Path $service.LogDirectory | Out-Null
    Invoke-Nssm stop $service.Name 2>$null
    Invoke-Nssm remove $service.Name confirm 2>$null
    Invoke-Nssm install $service.Name $service.Python
    Invoke-Nssm set $service.Name AppParameters $service.Parameters
    Invoke-Nssm set $service.Name AppDirectory $service.Directory
    Invoke-Nssm set $service.Name DisplayName $service.DisplayName
    Invoke-Nssm set $service.Name Description $service.Description
    Invoke-Nssm set $service.Name Start SERVICE_AUTO_START
    Invoke-Nssm set $service.Name AppStdout $stdout
    Invoke-Nssm set $service.Name AppStderr $stderr
    Invoke-Nssm set $service.Name AppRotateFiles 1
    Invoke-Nssm set $service.Name AppRotateOnline 1
    Invoke-Nssm set $service.Name AppRotateBytes 10485760
    Invoke-Nssm set $service.Name AppExit Default Restart
    Invoke-Nssm set $service.Name AppRestartDelay 5000
}

if (-not $Apply) {
    Write-Host "Dry run only. Re-run with -Apply from an elevated PowerShell prompt."
} else {
    foreach ($envFile in $envFiles) {
        Protect-EnvFile -Path $envFile -Account $ServiceUser
    }

    $servicePassword = $null
    $builtInAccount = $ServiceUser -match "^(LocalSystem|NT AUTHORITY\\LocalService|NT AUTHORITY\\NetworkService)$"
    if (-not $builtInAccount) {
        $credential = Get-Credential -UserName $ServiceUser -Message "Password for the NSSM service account"
        $servicePassword = $credential.GetNetworkCredential().Password
    }

    foreach ($service in $services) {
        if ($builtInAccount) {
            Invoke-Nssm set $service.Name ObjectName $ServiceUser
        } else {
            Invoke-Nssm set $service.Name ObjectName $ServiceUser $servicePassword
        }
    }

    foreach ($service in $services) {
        Invoke-Nssm start $service.Name
    }

    Invoke-RestMethod "http://127.0.0.1:5023/healthz" | Out-Host
    Invoke-RestMethod "http://127.0.0.1:5024/healthz" | Out-Host
}

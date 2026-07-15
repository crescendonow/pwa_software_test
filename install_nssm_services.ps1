param(
    [string]$NssmPath = "C:\nssm-2.24\nssm-2.24\win64\nssm.exe",
    [string]$AppRoot = $PSScriptRoot,
    [string]$ServiceUser = "",
    [switch]$Apply
)

$ErrorActionPreference = "Stop"

function Invoke-Nssm {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
    # Windows PowerShell turns anything nssm writes to stderr into a
    # NativeCommandError, which $ErrorActionPreference = "Stop" would promote to
    # a terminating error. nssm is chatty on stderr even when it succeeds, so
    # let it talk and judge the outcome by the exit code instead.
    $ErrorActionPreference = "Continue"
    & $NssmPath @Args
}

function Invoke-NssmChecked {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
    Invoke-Nssm @Args
    if ($LASTEXITCODE -ne 0) {
        throw "nssm $($Args -join ' ') failed with exit code $LASTEXITCODE"
    }
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
    # SYSTEM and Administrators always get Full. Map built-in service identities to
    # the names icacls can resolve, and skip a redundant grant when the account is
    # already covered by SYSTEM (LocalSystem runs as SYSTEM).
    $grants = @('SYSTEM:(F)', 'Administrators:(F)')
    $extra = switch ($Account) {
        'LocalSystem'                 { $null }
        'NT AUTHORITY\LocalService'   { 'LOCAL SERVICE' }
        'NT AUTHORITY\NetworkService' { 'NETWORK SERVICE' }
        default                       { $Account }
    }
    if ($extra) { $grants += ('{0}:(R)' -f $extra) }
    & icacls.exe $Path /inheritance:r /grant:r @grants | Out-Host
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

    # Only tear down what is actually there. A fresh box has no service yet, and
    # asking nssm to stop it just prints "Can't open service!".
    if (Get-Service -Name $service.Name -ErrorAction SilentlyContinue) {
        Write-Host "  removing existing $($service.Name)"
        Invoke-Nssm stop $service.Name
        Invoke-NssmChecked remove $service.Name confirm
    }

    Invoke-NssmChecked install $service.Name $service.Python
    Invoke-NssmChecked set $service.Name AppParameters $service.Parameters
    Invoke-NssmChecked set $service.Name AppDirectory $service.Directory
    Invoke-NssmChecked set $service.Name DisplayName $service.DisplayName
    Invoke-NssmChecked set $service.Name Description $service.Description
    Invoke-NssmChecked set $service.Name Start SERVICE_AUTO_START
    Invoke-NssmChecked set $service.Name AppStdout $stdout
    Invoke-NssmChecked set $service.Name AppStderr $stderr
    Invoke-NssmChecked set $service.Name AppRotateFiles 1
    Invoke-NssmChecked set $service.Name AppRotateOnline 1
    Invoke-NssmChecked set $service.Name AppRotateBytes 10485760
    Invoke-NssmChecked set $service.Name AppExit Default Restart
    Invoke-NssmChecked set $service.Name AppRestartDelay 5000
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
            Invoke-NssmChecked set $service.Name ObjectName $ServiceUser
        } else {
            Invoke-NssmChecked set $service.Name ObjectName $ServiceUser $servicePassword
        }
    }

    foreach ($service in $services) {
        Invoke-NssmChecked start $service.Name
    }

    Invoke-RestMethod "http://127.0.0.1:5023/healthz" | Out-Host
    Invoke-RestMethod "http://127.0.0.1:5024/healthz" | Out-Host
}

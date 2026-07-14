# Native Thai NLP service

The Go dashboard sends comments to the full endpoint in `NLP_URL`. For a service on the same host, keep this value:

```text
NLP_URL=http://127.0.0.1:5023/wordcloud
```

Do not use Docker for the native deployment. The Go process and the NLP process must share the host when `NLP_URL` uses `127.0.0.1` or `localhost`. If they run on separate hosts, set `NLP_URL` to the reachable NLP host and retain the `/wordcloud` path.

## Linux server setup

Use Python 3.11 and create one virtual environment beside this application:

```sh
cd /opt/gisonline-uat/nlp
python3.11 -m venv .venv
.venv/bin/python -m pip install --upgrade pip
.venv/bin/python -m pip install -r requirements.txt
sh run.sh
```

The launcher only starts `.venv/bin/python`; it fails clearly when the virtual environment is missing instead of falling back to a system Python.

Confirm readiness before starting or restarting the Go server:

```sh
curl --fail http://127.0.0.1:5023/healthz
```

## Windows development setup

```powershell
py -3.11 -m venv nlp\.venv
nlp\.venv\Scripts\python.exe -m pip install --upgrade pip
nlp\.venv\Scripts\python.exe -m pip install -r nlp\requirements.txt
.\nlp\run.ps1
```

To test tokenization after the health check, post a JSON body containing a `comments` array to `http://127.0.0.1:5023/wordcloud` and confirm it returns a JSON list of word and weight items.

## Windows NSSM deployment

Run this sequence for a production deployment. Do not run the Docker Compose
NLP container at the same time as these native services.

1. Back up PostgreSQL, then apply `db/005_freetext_query_audit.sql` to a
   disposable database copy first. Verify the audit table and permissions
   before applying the migration to production.
2. Create both virtual environments and install their requirements:

```powershell
py -3.11 -m venv nlp\.venv
nlp\.venv\Scripts\python.exe -m pip install -r nlp\requirements.txt
py -3.11 -m venv freetext_query\.venv
freetext_query\.venv\Scripts\python.exe -m pip install -r freetext_query\requirements.txt
Copy-Item freetext_query\.env.example freetext_query\.env
# Edit nlp\.env and freetext_query\.env with production values.
```

3. From an elevated PowerShell prompt, install or restart NSSM. Pass the
   dedicated Windows service account; the installer applies ACLs to both
   `.env` files before starting either service and configures automatic restart
   and rotating service logs:

```powershell
.\install_nssm_services.ps1 -ServiceUser 'DOMAIN\gis-uat-services' -Apply
```

4. Verify readiness and loopback binding, then restart the Go service so it
   reloads `NLP_URL` and `FREETEXT_QUERY_URL`:

```powershell
Invoke-RestMethod http://127.0.0.1:5023/healthz
Invoke-RestMethod http://127.0.0.1:5024/healthz
netstat -ano | Select-String ':5023|:5024'
# Restart the existing Go/Nginx upstream service using its normal runbook.
```

The installer binds word cloud to `127.0.0.1:5023` and free-text query to
`127.0.0.1:5024`. Do not expose either port through Nginx or a firewall; the
browser calls the authenticated Go endpoints only. If a service fails, inspect
`nlp\logs` or `freetext_query\logs`, correct the environment/ACL, and rerun
the installer; restore the database backup if the migration validation fails.

## systemd deployment

Install the application under `/opt/gisonline-uat`, then create `/etc/systemd/system/gisonline-uat-nlp.service` with:

```ini
[Unit]
Description=GIS Online UAT Thai NLP
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/gisonline-uat/nlp
ExecStart=/bin/sh /opt/gisonline-uat/nlp/run.sh
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Run `systemctl daemon-reload`, then `systemctl enable --now gisonline-uat-nlp`. Use `systemctl status gisonline-uat-nlp` and the health check above after each deployment.

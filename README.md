# GIS Online UAT

เว็บสำหรับบันทึกผล UAT ของ PWA GIS Plugin ตามรูปแบบไฟล์ `Test PWA GIS Plugin.xlsx`

## Stack

- Frontend: HTML + Tailwind CSS CDN + JavaScript
- Backend: Go HTTP server
- Database: PostgreSQL 16

## Files

- `.docs/1_plan_for_gisonline_uat_20260630.md`: implementation plan
- `db/001_schema.sql`: schema `ut_logs`, tables, constraints, indexes, report view
- `db/002_seed_from_excel.sql`: seed test cases และ imported session จาก Excel
- `cmd/server/main.go`: entrypoint
- `internal/uat`: API, models, PostgreSQL store
- `nlp/README.md`: native Thai NLP service setup and deployment runbook
- `freetext_query/`: authenticated free-text to read-only UAT query service
- `install_nssm_services.ps1`: Windows NSSM installer for both Python services
- `web`: frontend

## Run With PostgreSQL

Create PostgreSQL database and run:

```powershell
$env:DATABASE_URL = "postgres://uat_user:uat_password@localhost:5432/gisonline_uat?sslmode=disable"
go run ./cmd/migrate
go run ./cmd/server
```

The commands also load `.env` automatically when the file exists.

Open:

```text
http://localhost:8080
```

## Docker Compose

If Docker is available:

```powershell
docker compose up -d
$env:DATABASE_URL = "postgres://uat_user:uat_password@localhost:5432/gisonline_uat?sslmode=disable"
go run ./cmd/server
```

## API

- `GET /api/health`
- `GET /api/test-cases`
- `GET /api/sessions`
- `POST /api/sessions`
- `GET /api/sessions/{id}/results`
- `PATCH /api/results/{id}`
- `GET /api/report?session_id={id}`
- `POST /api/freetext-query` with `{ "prompt": "..." }`

## Verify

```powershell
go test ./...
```

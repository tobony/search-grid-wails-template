# Search Grid Template (Wails)

A desktop document-search grid built with [Wails v2](https://wails.io) + React + TypeScript + Go,
designed for small internal teams (tens of users) in enterprise environments where
browser-based OAuth flows and Graph API access are restricted.

## What it does

- Fluent-style searchable document grid (title / author / category / status)
- Syncs with a Power BI **push dataset** through the Power BI REST API
- Authentication via the **Azure CLI** (`az login`) — no app registration, no client secrets
- Local **SQLite cache** so the grid works offline and starts fast
- Add / delete documents, CSV export (formula-injection safe), environment self-diagnosis UI

## Prerequisites

- Go 1.22+
- Node.js 18+
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Azure CLI (`az`) installed and logged in for each user: `az login`
- A Power BI push dataset (see below)

## Configuration

The app reads the Power BI push dataset ID from the `SEARCH_GRID_DATASET_ID`
environment variable:

```bash
export SEARCH_GRID_DATASET_ID="00000000-0000-0000-0000-000000000000"
```

The dataset must contain a `Docs` table with columns:
`id (Int64), title, author, category, updated, status`.
See `AGENTS.private.example.md` for the structure of a local private-config file.

## Run

```bash
wails dev        # live development
wails build      # production build
```

On Linux you may need the webkit2gtk 4.1 dev packages; see the Wails docs.

## Project layout

- `app.go` — Wails bindings (GetDocs / SyncFromPowerBI / AddDoc / DeleteDoc / ExportCSV / CheckEnv)
- `pbi/` — Power BI REST client (token via Azure CLI, retry on 401/429, 100-row batches)
- `store/` — SQLite cache (pending rows survive failed syncs)
- `frontend/` — React + TypeScript grid UI
- `diagnose/` — environment self-diagnosis (az installed? logged in? token valid?)

## License

MIT

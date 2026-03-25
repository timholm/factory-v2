# factory-v2

## Build & Test

```bash
make build        # compile to bin/factory-v2
make test         # run all tests
go test ./... -v  # verbose test output
go vet ./...      # lint
```

## Architecture

One Go binary with cobra CLI. Four commands: run, build, status, audit.

The `run` command executes a continuous loop:
1. discover: HTTP call to archive API, cluster papers by category
2. research: keyword extraction from abstracts, GitHub repo search
3. synthesize: Claude Opus CLI call to fuse techniques into a ProductSpec
4. build: scaffold workspace, one Claude Sonnet session (30 turns), auto-retry on test failure
5. validate: check module path, tests, README, no secrets
6. audit: clone from GitHub, score 0-100, delete if < 50
7. report: log stats to stdout

## Key packages

- `internal/config` — env var config with defaults
- `internal/db` — Postgres via pgx, migrations, CRUD
- `internal/discover` — paper fetching and max-diversity clustering
- `internal/research` — technique extraction and GitHub search
- `internal/synthesize` — Claude Opus integration for spec generation
- `internal/build` — the core: scaffold, invoke Claude, deps, validate, git
- `internal/audit` — quality scoring and repo deletion
- `internal/report` — pipeline stats

## Module

github.com/timholm/factory-v2

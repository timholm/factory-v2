# factory-v2

Autonomous research-to-product factory. One binary, one loop. Discovers arXiv papers, clusters them by problem space, researches techniques, synthesizes fusion specs, builds production-grade repos, validates quality, and reports.

## How it works

1. **Discover** — Fetches recent papers from an arXiv archive API, clusters them into groups of 7 diverse papers per problem space using a greedy max-diversity algorithm.

2. **Research** — Extracts the key technique from each paper (keyword-based, no LLM needed). Finds 7 relevant GitHub repos for each problem space.

3. **Synthesize** — Sends 7 techniques + 7 repos to Claude Opus via CLI. Gets back a complete ProductSpec with architecture, features, and technique map.

4. **Build** — ONE Claude session per repo (up to 30 turns). Implements code, writes tests, creates docs, fixes failures. Auto-retries if tests fail.

5. **Validate** — Checks module path, test files, README references, no secrets leaked.

6. **Audit** — Clones shipped repos from GitHub, scores 0-100. Deletes repos scoring below 50.

7. **Report** — Logs ship rate, quality scores, failures, recommendations.

## Install

```bash
go install github.com/timholm/factory-v2@latest
```

Or build from source:

```bash
make build
# Binary at bin/factory-v2
```

## Usage

```bash
# Full autonomous loop
factory-v2 run

# Build one spec manually
factory-v2 build --spec spec.json

# Show pipeline status
factory-v2 status

# Run quality audit on all shipped repos
factory-v2 audit
```

## Configuration

All config via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_URL` | `postgres://factory:factory@localhost:5432/factory?sslmode=disable` | Postgres connection string |
| `ARCHIVE_URL` | `http://localhost:8080` | arXiv archive API URL |
| `GITHUB_TOKEN` | (required) | GitHub personal access token |
| `GITHUB_USER` | `timholm` | GitHub username for repos |
| `GIT_DIR` | `~/factory-git` | Local bare repo directory |
| `CLAUDE_BINARY` | `claude` | Path to Claude CLI |
| `CYCLE_INTERVAL` | `1h` | Time between cycles |
| `MAX_BUILDS` | `5` | Max builds per cycle |
| `WORKERS` | `2` | Parallel build workers |

## Architecture

```
main.go                     CLI entry (cobra), run loop
internal/
  config/config.go          Env vars
  discover/discover.go      Fetch papers, cluster by problem space
  discover/cluster.go       Greedy max-diversity clustering
  research/research.go      Extract techniques, find repos
  research/technique.go     Keyword-based technique extraction
  research/repos.go         GitHub search API
  synthesize/synthesize.go  Claude Opus fusion
  synthesize/prompt.go      Synthesis prompt
  build/build.go            ONE Claude session per repo
  build/scaffold.go         Create workspace
  build/validate.go         Quality checks
  build/secrets.go          Secret scrubbing
  build/deps.go             Auto dependency resolution
  build/git.go              Git operations
  audit/audit.go            Clone, score, delete
  db/db.go                  Postgres (pgx)
  report/report.go          Pipeline reporting
```

## Requirements

- Go 1.22+
- PostgreSQL (running on K8s, port-forwarded)
- Claude CLI (Max subscription)
- GitHub CLI (`gh`)

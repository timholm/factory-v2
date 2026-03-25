# factory-v2 Agent Guide

## Key Files

| File | Purpose |
|------|---------|
| `main.go` | CLI entry, run loop orchestration |
| `internal/config/config.go` | All configuration from env vars |
| `internal/db/db.go` | Postgres schema, migrations, all queries |
| `internal/discover/discover.go` | Paper fetching from archive API |
| `internal/discover/cluster.go` | Max-diversity paper selection algorithm |
| `internal/research/technique.go` | Keyword-based technique extraction |
| `internal/research/repos.go` | GitHub search API integration |
| `internal/synthesize/synthesize.go` | Claude Opus invocation for spec fusion |
| `internal/synthesize/prompt.go` | The synthesis prompt template |
| `internal/build/build.go` | Core build orchestration |
| `internal/build/scaffold.go` | Workspace setup (SPEC.md, Makefile, go.mod) |
| `internal/build/prompt.go` | Build prompt for Claude Sonnet |
| `internal/build/validate.go` | Post-build quality checks |
| `internal/build/secrets.go` | Secret detection and scrubbing |
| `internal/build/deps.go` | Auto dependency resolution |
| `internal/build/git.go` | Git init, commit, push, mirror |
| `internal/audit/audit.go` | Quality scoring (0-100) and repo deletion |
| `internal/report/report.go` | Pipeline reporting |
| `prompts/build.md.tmpl` | Reference build prompt template |

## How to extend

### Add a new pipeline stage
1. Create `internal/newstage/newstage.go`
2. Add struct + New() constructor following existing patterns
3. Wire into Factory struct in `main.go`
4. Call from the Run() loop

### Change the build prompt
Edit `internal/build/prompt.go` (RenderBuildPrompt function)

### Add new quality checks
Edit `internal/build/validate.go` (Validate function) or `internal/audit/audit.go` (Score function)

### Add new secret patterns
Edit `internal/build/secrets.go` (secretPatterns var)

### Change the database schema
Edit `internal/db/db.go` (Migrate function) and add new query methods

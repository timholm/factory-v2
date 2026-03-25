# Factory v3 — Design

## Core change: pipeline not batch

v2: discover ALL → research ALL → synthesize ALL → build ALL
v3: discover 1 → research 1 → synthesize 1 → build 1 (while discovering next)

## Architecture

```
main loop
  ├── discoverer goroutine (fills spec channel continuously)
  │     discover cluster → research → synthesize → push to specCh
  │
  ├── builder pool (6 goroutines pulling from specCh)
  │     pull spec → oracle check → claude build → validate → ship
  │     if rate limited → sleep until reset → resume automatically
  │
  ├── overseer goroutine (continuous)
  │     audit → critique → auto-fix critical issues
  │
  └── healer goroutine (continuous)
        watch for stuck builds → kill after timeout → retry
        watch for rate limits → pause/resume workers
        watch for dead port-forwards → reconnect
        watch for expired creds → refresh
```

## Rate limit handling

When Claude returns "rate limit" or "usage limit":
1. Parse reset time from error message
2. Log it
3. Sleep until reset + 60s buffer
4. Resume automatically
5. Never crash, never stop

## Auto-scaling

- Start with 2 workers
- If all complete successfully, add 1 (max 6)
- If rate limited, drop to 1
- If rate limit clears, scale back up
- Track success rate — if below 40%, pause and let overseer diagnose

## Self-healing

Every 60 seconds, healer checks:
- Port forwards alive? If not, reconnect
- Claude creds valid? If not, refresh
- Any tmux session stuck > 30 min? Kill and retry
- Any build dir older than 1 hour? Clean up
- Factory process alive? (LaunchAgent handles this)

#!/bin/bash
set -euo pipefail

LOG="/tmp/factory-v2.log"
echo "$(date): factory-v2 starting" >> "$LOG"

# Prevent Mac from sleeping while factory runs
caffeinate -d -i -s &
CAFF_PID=$!
trap "kill $CAFF_PID 2>/dev/null" EXIT

# Port forwards (reconnect if dead)
setup_portforward() {
    if ! lsof -ti :5432 > /dev/null 2>&1; then
        kubectl port-forward pod/postgres-0 5432:5432 -n factory >> /tmp/portforward.log 2>&1 &
        sleep 2
    fi
    if ! lsof -ti :9090 > /dev/null 2>&1; then
        kubectl port-forward -n factory svc/archive-serve 9090:9090 >> /tmp/portforward.log 2>&1 &
        sleep 2
    fi
}

# Get Postgres password
PG_PASS=$(kubectl get secret postgres-credentials -n factory -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d)

export POSTGRES_URL="postgres://factory:${PG_PASS}@localhost:5432/arxiv?sslmode=disable"
export ARCHIVE_URL="http://localhost:9090"
export GITHUB_TOKEN=$(gh auth token)
export GITHUB_USER=timholm
export GIT_DIR="$HOME/factory-git"
export CLAUDE_BINARY=claude

mkdir -p "$GIT_DIR"

# Main loop — NEVER stops, NEVER sleeps more than 5 min
while true; do
    echo "$(date): === FACTORY V2 CYCLE ===" >> "$LOG"

    # Ensure port forwards are alive
    setup_portforward

    # Refresh GitHub token
    export GITHUB_TOKEN=$(gh auth token 2>/dev/null || echo "$GITHUB_TOKEN")

    # Refresh Claude creds (quick, prevents expiry)
    claude -p "ok" --max-turns 1 --output-format text > /dev/null 2>&1 || true
    kubectl delete secret claude-credentials --namespace factory 2>/dev/null || true
    kubectl create secret generic claude-credentials \
        --namespace factory \
        --from-file=credentials.json="$HOME/.claude/.credentials.json" 2>/dev/null || true

    # Pull latest code
    cd "$HOME/factory-v2"
    git pull --ff-only origin main >> "$LOG" 2>&1 || true

    # Run one cycle from source
    go run -buildvcs=false . run >> "$LOG" 2>&1 || echo "$(date): cycle error: $?" >> "$LOG"

    # No sleep — immediately start next cycle
    echo "$(date): cycle done, starting next immediately" >> "$LOG"
done

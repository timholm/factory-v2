#!/bin/bash
set -euo pipefail

LOG="/tmp/factory-v2.log"
echo "$(date): factory-v2 starting" >> "$LOG"

# Port forwards (reconnect if dead)
setup_portforward() {
    if ! lsof -ti :5432 > /dev/null 2>&1; then
        kubectl port-forward pod/postgres-0 5432:5432 -n factory &
        sleep 2
    fi
    if ! lsof -ti :9090 > /dev/null 2>&1; then
        kubectl port-forward -n factory svc/archive-serve 9090:9090 &
        sleep 2
    fi
}

# Refresh Claude credentials
refresh_creds() {
    claude -p "echo hello" --max-turns 1 --output-format text > /dev/null 2>&1 || true
    kubectl delete secret claude-credentials --namespace factory 2>/dev/null || true
    kubectl create secret generic claude-credentials \
        --namespace factory \
        --from-file=credentials.json="$HOME/.claude/.credentials.json" 2>/dev/null || true
    # No v1 pods to restart — factory-v2 runs locally
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

# Main loop — never stops
while true; do
    echo "$(date): === FACTORY V2 CYCLE ===" >> "$LOG"
    
    # Ensure port forwards are alive
    setup_portforward
    
    # Refresh creds every cycle (cheap, prevents expiry issues)
    refresh_creds >> "$LOG" 2>&1 || true
    
    # Refresh GitHub token (might have rotated)
    export GITHUB_TOKEN=$(gh auth token 2>/dev/null || echo "$GITHUB_TOKEN")
    
    # Pull latest code if remote has changes
    cd "$HOME/factory-v2"
    git pull --ff-only origin main >> "$LOG" 2>&1 || true

    # Run from source — picks up code changes automatically, no manual rebuild needed
    timeout 3600 go run . run >> "$LOG" 2>&1 || echo "$(date): cycle exited: $?" >> "$LOG"
    
    echo "$(date): cycle complete, sleeping 30m" >> "$LOG"
    sleep 1800
done

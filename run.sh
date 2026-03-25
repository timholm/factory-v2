#!/bin/bash
set -euo pipefail
LOG="/tmp/factory-v2.log"
caffeinate -d -i -s &
trap "kill $! 2>/dev/null" EXIT

# Port forwards
lsof -ti :5432 > /dev/null 2>&1 || kubectl port-forward pod/postgres-0 5432:5432 -n factory > /dev/null 2>&1 &
lsof -ti :9090 > /dev/null 2>&1 || kubectl port-forward -n factory svc/archive-serve 9090:9090 > /dev/null 2>&1 &
sleep 2

PG_PASS=$(kubectl get secret postgres-credentials -n factory -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d)
export POSTGRES_URL="postgres://factory:${PG_PASS}@localhost:5432/arxiv?sslmode=disable"
export ARCHIVE_URL="http://localhost:9090"
export GITHUB_TOKEN=$(gh auth token)
export GITHUB_USER=timholm
export GIT_DIR="$HOME/factory-git"
export CLAUDE_BINARY=claude

# Rebuild from source if code changed
cd "$HOME/factory-v2"
git pull --ff-only origin main >> "$LOG" 2>&1 || true
go build -buildvcs=false -o factory-v3 . >> "$LOG" 2>&1 || true

exec ./factory-v3 run >> "$LOG" 2>&1

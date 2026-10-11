#!/bin/sh
# Serve the site as it is at a git ref (default origin/main) for "before" screenshots.
# Usage: serve-ref.sh [ref=origin/main] [port=4392]   — stop with: serve-ref.sh --stop [port]
# Checks the ref out in a temporary worktree and reuses site/node_modules from this checkout.
set -e
REPO="$(git rev-parse --show-toplevel)"
if [ "$1" = "--stop" ]; then
  PORT="${2:-4392}"
  pkill -f "astro dev --port $PORT" || true
  DIR="${TMPDIR:-/tmp}/wtm-site-ref-$PORT"
  git -C "$REPO" worktree remove --force "$DIR" 2>/dev/null || true
  echo "stopped $PORT"
  exit 0
fi
REF="${1:-origin/main}"
PORT="${2:-4392}"
DIR="${TMPDIR:-/tmp}/wtm-site-ref-$PORT"
[ -d "$REPO/site/node_modules" ] || (cd "$REPO/site" && npm ci >/dev/null)
git -C "$REPO" fetch -q origin 2>/dev/null || true
git -C "$REPO" worktree remove --force "$DIR" 2>/dev/null || true
git -C "$REPO" worktree add -q --detach "$DIR" "$REF"
ln -s "$REPO/site/node_modules" "$DIR/site/node_modules"
cd "$DIR/site"
node scripts/sync-docs.mjs >/dev/null
nohup node_modules/.bin/astro dev --port "$PORT" >/dev/null 2>&1 &
for _ in $(seq 1 30); do
  curl -s -o /dev/null "http://localhost:$PORT/" && { echo "http://localhost:$PORT/ serves $REF"; exit 0; }
  sleep 1
done
echo "the server on $PORT did not answer" >&2
exit 1

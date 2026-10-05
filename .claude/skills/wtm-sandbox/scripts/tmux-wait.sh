#!/usr/bin/env bash
# Usage: tmux-wait.sh <session> <regex> [timeout-seconds, default 15]
# Polls the pane until <regex> (grep -E) appears, then prints the pane as
# plain text. Exit 1 on timeout, with the last pane on stderr so a stuck
# screen can be read rather than guessed.
set -euo pipefail

session=${1:?tmux session}
pattern=${2:?regex to wait for}
deadline=$((SECONDS + ${3:-15}))

while ((SECONDS < deadline)); do
  pane=$(tmux capture-pane -p -t "$session")
  if grep -Eq "$pattern" <<<"$pane"; then
    printf '%s\n' "$pane"
    exit 0
  fi
  sleep 0.3
done
echo "timed out waiting for /$pattern/ in $session; last screen:" >&2
tmux capture-pane -p -t "$session" >&2
exit 1

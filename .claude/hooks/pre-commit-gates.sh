#!/usr/bin/env bash
# Pre-commit gates, run from a PreToolUse hook on Bash.
#
# CLAUDE.md section 11 says the gates run before any commit. Saying so was not
# enough: this repo shipped a stale go.sum because the checks had been run once,
# earlier in the session, and treated as still true six commits later. A hook
# cannot forget.
#
# It runs `make lint` (fmt, vet, archlint, deadcode, staticcheck — about two
# seconds) and the tidy check that the CI runs first. It does NOT run the test
# suite: at ~70s that would be a gate people disable. Tests are the CI's job and
# the build-validator subagent's.
#
# Exit 2 blocks the commit and hands the reason back to Claude.
# WTM_SKIP_GATES=1 gets past it, for the case the gates are themselves broken.

set -uo pipefail

input=$(cat)
command=$(jq -r '.tool_input.command // ""' <<<"$input" 2>/dev/null)
hook_cwd=$(jq -r '.cwd // ""' <<<"$input" 2>/dev/null)

# Anchored so `git log --grep=commit` and a message that merely says "git
# commit" do not pay for a lint run. A commit is the first word of the command
# or of one of its segments.
if ! grep -Eq '(^|[;&|]|&&|\|\|)[[:space:]]*git[[:space:]]+(-[^[:space:]]+[[:space:]]+)*commit([[:space:]]|$)' <<<"$command"; then
  exit 0
fi

if [[ "${WTM_SKIP_GATES:-}" == "1" ]]; then
  exit 0
fi

# The worktree being committed, not the one this session started in: a
# subagent commits from its own worktree with `git -C <dir>` or `cd <dir> &&`.
target=$(grep -Eo 'git[[:space:]]+-C[[:space:]]+[^[:space:];&|]+' <<<"$command" | head -1 | awk '{print $3}')
if [[ -z "$target" ]]; then
  target=$(grep -Eo '(^|[;&|])[[:space:]]*cd[[:space:]]+[^[:space:];&|]+' <<<"$command" | tail -1 | awk '{print $NF}')
fi
target=${target/#\~/$HOME}
target=${target//\"/}
target=${target//\'/}
[[ -n "$target" ]] || target=${hook_cwd:-$PWD}
[[ "$target" = /* ]] || target="${hook_cwd:-$PWD}/$target"

root=$(git -C "$target" rev-parse --show-toplevel 2>/dev/null) || exit 0
cd "$root" || exit 0
[[ -f Makefile && -f go.mod ]] || exit 0

fail() {
  echo "pre-commit gates: $1" >&2
  echo >&2
  echo "$2" >&2
  echo >&2
  echo "Fix it and commit again, or set WTM_SKIP_GATES=1 for this one command if the gate is itself wrong." >&2
  exit 2
}

# go.mod tidy — the CI's first step, and the one this hook exists for. Run on a
# copy: a hook must never mutate the tree it is about to let you commit.
tidy=$(mktemp -d)
trap 'rm -rf "$tidy"' EXIT
cp go.mod go.sum "$tidy/"
if ! go mod tidy >"$tidy/out" 2>&1; then
  cp "$tidy/go.mod" go.mod && cp "$tidy/go.sum" go.sum
  fail "go mod tidy failed" "$(cat "$tidy/out")"
fi
if ! diff -q "$tidy/go.mod" go.mod >/dev/null || ! diff -q "$tidy/go.sum" go.sum >/dev/null; then
  # Left applied on purpose: the fix is exactly what tidy just wrote, and the
  # commit is blocked anyway — so the next attempt includes it.
  fail "go.mod / go.sum were not tidy" \
    "\`go mod tidy\` changed them and the change has been applied. Stage it and commit again."
fi

if ! output=$(make lint 2>&1); then
  fail "make lint failed" "$output"
fi

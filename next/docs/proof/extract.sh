#!/usr/bin/env bash
# Usage: extract.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same extract scenarios in two identical sandboxes (the docs/demos "acme"
# project), one per binary: legacy `wtm extract` against `wtm-next extract`. After
# each scenario it records the exit code, the output and the state of every
# worktree (index, the bytes of each changed file, meta.json, branches), then
# diffs the states. Outputs differ by design (next's v2 contract); the states may
# differ only where next/docs/proof/README.md says why.
set -euo pipefail

legacy=${1:?wtm binary}
next=${2:?wtm-next binary}
out=${3:?out dir}
mkdir -p "$out" && out=$(cd "$out" && pwd)
repo=$(git rev-parse --show-toplevel)

prepare() {
  local root=$1
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=24200 WTM_DEMO_API_PORT=24201 WTM_DEMO_PROXY_PORT=24202 \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  git init -q --bare "$root/origin.git"
  git remote add origin "$root/origin.git"
  git push -q origin main
  "$legacy" create feat/login --yes </dev/null >/dev/null 2>&1
  "$legacy" create topic --yes </dev/null >/dev/null 2>&1
  local login=$root/acme.trees/feat-login
  echo "# acme — login" >"$login/README.md"
  mkdir -p "$login/apps/api/new/deep"
  echo "x" >"$login/apps/api/new/x.txt"
  echo "y" >"$login/apps/api/new/deep/y.txt"
  echo "export const login = 1" >"$login/apps/web/login.ts"
  git -C "$login" rm -q apps/web/package.json
  git -C "$login" mv scripts/dev.py scripts/serve.py
}

scenario() {
  local name=$1
  shift
  local code=0
  "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr" || code=$?
  echo "$code" >"$dir/$name.exit"
  state >"$dir/$name.state"
}

state() {
  echo "## branches"
  git branch --format='%(refname:short)'
  git worktree list --porcelain | awk '/^worktree /{print $2}' | while read -r wt; do
    echo "## $(basename "$wt") status"
    git -C "$wt" status --porcelain=v1 --untracked-files=all
    git -C "$wt" status --porcelain=v1 -z --untracked-files=all | tr '\0' '\n' | awk 'length($0) > 3 {print substr($0, 4)}' | sort -u | while read -r path; do
      [[ -f "$wt/$path" ]] && echo "   $path $(shasum "$wt/$path" | cut -c1-12)"
    done
  done
  for meta in $(find .git/wtm/worktrees -name meta.json | sort); do
    echo "## $meta"
    grep -v created_at "$meta"
  done
}

run_side() {
  local side=$1
  bin=$2
  local root=/tmp/wtmext-${side:0:1}
  dir=$out/$side
  rm -rf "$dir" && mkdir -p "$dir"
  (
    prepare "$root"
    scenario 01-dir-to-new-branch extract feat/login --files apps/api --to feat/api --yes
    scenario 02-copy-to-existing extract feat/login --files apps/web/login.ts --to topic --keep --yes
    scenario 03-rename-and-delete extract feat/login --files scripts/serve.py,apps/web/package.json --to feat/misc --from main --yes
    git branch feat/local
    scenario 04-existing-local-without-from extract feat/login --files README.md --to feat/local --yes
    echo "# acme — topic" >"$root/acme.trees/topic/README.md"
    scenario 05-conflict-dirty-target extract feat/login --files README.md --to topic --yes
    scenario 06-resolve extract feat/login --files README.md --to topic --on-conflict resolve --yes
    "$legacy" create feat/c --yes </dev/null >/dev/null 2>&1
    (cd "$root/acme.trees/feat-c" && echo "# acme — c" >README.md && git -c user.email=c@acme.dev -c user.name=C commit -qam "c: readme")
    scenario 07-conflict-committed-in-target extract feat/login --files README.md --to feat/c --yes
    scenario 08-nothing-to-extract extract main --files README.md --to feat/x --yes
    scenario 09-json extract feat/login --files apps/web/login.ts --to feat/json --output json --yes
  )
  sed -i '' -E \
    -e "s#(/private)?/tmp/wtmext-${side:0:1}#<SB>#g" \
    -e 's#"created_at": "[^"]*"#"created_at": "<T>"#g' \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#wtm-next extract#wtm extract#g' \
    "$dir"/*
}

mkdir -p "$out"
run_side legacy "$legacy"
run_side next "$next"
{
  for state in "$out"/legacy/*.state; do
    name=$(basename "$state" .state)
    echo "=== $name: exit legacy $(cat "$out/legacy/$name.exit"), next $(cat "$out/next/$name.exit")"
    diff -u "$state" "$out/next/$name.state" || true
  done
} >"$out/states.diff"
echo "$out/states.diff"

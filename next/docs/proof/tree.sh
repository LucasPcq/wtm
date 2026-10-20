#!/usr/bin/env bash
# Usage: tree.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same tree scenarios in two identical sandboxes (the docs/demos "acme"
# project), one per binary: legacy `wtm tree` against `wtm-next tree`. Each
# scenario records the exit code, stdout and stderr, and the state on disk before
# and after it (refs, worktrees, every meta.json and git status, wtm's global
# directory): tree is a read, so <side>/unchanged.diff stays empty. Outputs differ
# by design (next's v2 contract); every difference is listed in
# next/docs/proof/README.md.
set -euo pipefail

# absolute <binary>: the binary's absolute path, since each scenario runs from inside its sandbox.
absolute() {
  [[ -f $1 && -x $1 ]] || { echo "$0: $1 is not an executable" >&2; return 1; }
  echo "$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
}
legacy=$(absolute "${1:?wtm binary}")
next=$(absolute "${2:?wtm-next binary}")
out=${3:?out dir}
mkdir -p "$out" && out=$(cd "$out" && pwd)
repo=$(git rev-parse --show-toplevel)

# commit <worktree> <file> <line>: commits line as the content of file in worktree.
commit() {
  echo "$3" >"$1/$2"
  git -C "$1" add "$2"
  git -C "$1" commit -q -m "$3"
}

# source_of <branch> <parent>: records parent as branch's source in its meta.json, as a hand edit would.
source_of() {
  local meta=.git/wtm/worktrees/${1//\//-}/meta.json
  python3 -c 'import json, sys
meta = json.load(open(sys.argv[1]))
meta["source_branch"] = sys.argv[2]
open(sys.argv[1], "w").write(json.dumps(meta, indent=2) + "\n")' "$meta" "$2"
}

prepare() {
  local root=$1 port=$((20000 + RANDOM % 9000))
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$port WTM_DEMO_API_PORT=$((port + 1)) WTM_DEMO_PROXY_PORT=$((port + 2)) \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  git init -q --bare "$root/origin.git"
  git remote add origin "$root/origin.git"
  git push -q origin main
  local trees=$root/acme.trees
  # each worktree a second apart: the tree orders siblings by creation
  create() {
    "$legacy" create "$@" --yes </dev/null >/dev/null 2>&1
    sleep 1
  }
  # a stack of three levels: feat/a, feat/a-ui on it, feat/a-ui-form on that
  create feat/a
  commit "$trees/feat-a" a.txt one
  commit "$trees/feat-a" a.txt two
  git -C "$trees/feat-a" push -q -u origin feat/a
  echo "x" >"$trees/feat-a/dirty.txt"
  create feat/a-ui --from feat/a
  commit "$trees/feat-a-ui" ui.txt ui
  create feat/a-ui-form --from feat/a-ui
  # a fan-out under main: feat/b, locked
  create feat/b
  commit "$trees/feat-b" b.txt b
  git worktree lock "$trees/feat-b"
  # a rebase in progress: feat/r and main change the same line of README.md
  create feat/r
  commit "$trees/feat-r" README.md "# acme, from feat/r"
  commit "$root/acme" README.md "# acme, from main"
  git -C "$trees/feat-r" rebase -q main >/dev/null 2>&1 || true
  # feat/a-ui moved on: feat/a-ui-form lacks its commit
  commit "$trees/feat-a-ui" ui.txt ui2
  # a parent with no worktree: feat/login removed, feat/login-ui left on it
  create feat/login
  create feat/login-ui --from feat/login
  "$legacy" clean feat/login --yes --force </dev/null >/dev/null 2>&1
  # a cycle: cyc-1 and cyc-2 each recorded as the other's source
  create cyc-1
  create cyc-2
  source_of cyc-1 cyc-2
  source_of cyc-2 cyc-1
  # a worktree whose directory is gone, and a detached one
  create feat/gone
  rm -rf "$trees/feat-gone"
  git worktree add -q --detach "$trees/detached"
  mkdir -p "$root/plain" "$root/bare"
  git -C "$root/bare" init -q
}

state() {
  git for-each-ref --format='%(refname) %(objectname)'
  git worktree list --porcelain
  git worktree list --porcelain | awk '/^worktree /{print $2}' | while read -r wt; do
    [[ -d $wt ]] || continue
    echo "## $wt"
    git -C "$wt" status --porcelain=v1 --untracked-files=all
  done
  for meta in $(find .git/wtm/worktrees -name meta.json | sort); do
    echo "## $meta"
    cat "$meta"
  done
  local global="$HOME/Library/Application Support/wtm"
  [[ -d $global ]] && (cd "$global" && find . -type f ! -name '*.sock' ! -name '*.log' | sort | xargs shasum)
  return 0
}

# scenario <name> <dir> <args…>: runs the binary with args in dir, without a terminal.
scenario() {
  local name=$1 dir=$2
  shift 2
  local code=0
  state >"$dir_out/.before"
  (cd "$dir" && "$bin" "$@" </dev/null >"$dir_out/$name.stdout" 2>"$dir_out/$name.stderr") || code=$?
  echo "$code" >"$dir_out/$name.exit"
  state >"$dir_out/.after"
  diff -u --label "$name before" --label "$name after" "$dir_out/.before" "$dir_out/.after" >>"$dir_out/unchanged.diff" || true
  rm -f "$dir_out/.before" "$dir_out/.after"
}

run_side() {
  local side=$1
  bin=$2
  # each run its own sandbox, removed on exit
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  dir_out=$out/$side
  rm -rf "$dir_out" && mkdir -p "$dir_out"
  (
    prepare "$root"
    : >"$dir_out/unchanged.diff"
    scenario 01-text "$root/acme" tree
    scenario 02-json "$root/acme" tree --output json
    scenario 03-mermaid "$root/acme" tree --output mermaid
    scenario 04-with-prs "$root/acme" tree --with-prs
    scenario 05-with-prs-json "$root/acme" tree --with-prs --output json
    scenario 06-from-linked "$root/acme.trees/feat-a" tree
    "$legacy" run up feat/a -d --yes </dev/null >/dev/null 2>&1
    scenario 07-jobs-up "$root/acme" tree
    scenario 08-jobs-up-json "$root/acme" tree --output json
    "$legacy" run down feat/a --yes </dev/null >/dev/null 2>&1
    scenario 09-quiet "$root/acme" tree -q
    scenario 10-quiet-mermaid "$root/acme" tree -q --output mermaid
    scenario 11-argument "$root/acme" tree feat/a
    scenario 12-unknown-format "$root/acme" tree --output dot
    scenario 13-no-config "$root/bare" tree
    scenario 14-no-config-json "$root/bare" tree --output json
    scenario 15-not-a-repository "$root/plain" tree --output json
    "$legacy" run daemon stop --yes </dev/null >/dev/null 2>&1 || true
  )
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"created_at": "[^"]*"#"created_at": "<T>"#g' \
    -e 's#[0-9a-f]{40}#<SHA>#g' \
    -e "s#$bin#<BIN>#g" \
    -e 's#wtm-next tree#wtm tree#g' \
    "$dir_out"/*
}

roots=()
trap 'rm -rf ${roots[@]+"${roots[@]}"}' EXIT
run_side legacy "$legacy"
run_side next "$next"
for side in legacy next; do
  if [[ -s $out/$side/unchanged.diff ]]; then
    echo "tree.sh: $side changed the state on disk, see $out/$side/unchanged.diff" >&2
  fi
done
echo "$out"

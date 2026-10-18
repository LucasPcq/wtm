#!/usr/bin/env bash
# Usage: list.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same list scenarios in two identical sandboxes (the docs/demos "acme"
# project), one per binary: legacy `wtm list` against `wtm-next list`. Each
# scenario records the exit code, stdout and stderr, and the state on disk before
# and after it (refs, worktrees, every meta.json and git status, wtm's global
# directory): list is a read, so <side>/unchanged.diff stays empty. Then the
# picker is driven in tmux. Outputs differ by design (next's v2 contract); every
# difference is listed in next/docs/proof/README.md.
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

prepare() {
  local root=$1 port=$((20000 + RANDOM % 9000))
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$port WTM_DEMO_API_PORT=$((port + 1)) WTM_DEMO_PROXY_PORT=$((port + 2)) \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  git init -q --bare "$root/origin.git"
  git remote add origin "$root/origin.git"
  git push -q origin main
  for branch in feat/a feat/b feat/gone; do
    "$legacy" create "$branch" --yes </dev/null >/dev/null 2>&1
  done
  local trees=$root/acme.trees
  git -C "$trees/feat-a" commit -q --allow-empty -m one
  git -C "$trees/feat-a" push -q -u origin feat/a
  git -C "$trees/feat-a" commit -q --allow-empty -m two
  echo "x" >"$trees/feat-a/dirty.txt"
  git worktree lock "$trees/feat-b"
  git worktree add -q --detach "$trees/detached"
  rm -rf "$trees/feat-gone"
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

# tty drives the picker in tmux: the worktrees, then feat/a's actions, then go; and a picker left with esc.
tty() {
  local session=wtmlist-$(basename "$root") here
  here=$root/acme
  screen() { tmux capture-pane -p -t "$session" | cat -s; }
  settle() {
    local now last="" tries=0
    while ((tries < 100)); do
      now=$(screen)
      [[ $now == *"$1"* && $now == "$last" ]] && return
      last=$now tries=$((tries + 1))
      sleep 0.3
    done
    echo "list.sh: the screen never settled on \"$1\"" >&2
    return 1
  }
  capture() {
    settle "$2"
    screen >"$dir_out/tty-$1.txt"
  }
  start() {
    tmux kill-session -t "$session" 2>/dev/null || true
    tmux new-session -d -s "$session" -x 100 -y 30 -c "$here" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "$bin list; echo EXIT=\$?" Enter
  }
  start
  # the picker is ready once the pull requests had their say: GitHub is not connected in a sandbox
  capture 1-worktrees 'gh auth login'
  tmux send-keys -t "$session" Down Enter
  capture 2-actions 'Clean'
  tmux send-keys -t "$session" Enter
  capture 3-go 'EXIT='
  start
  capture 4-again 'gh auth login'
  tmux send-keys -t "$session" Escape
  capture 5-left 'EXIT='
  tmux kill-session -t "$session"
}

run_side() {
  local side=$1
  bin=$2
  # each run its own sandbox, as long as the others' paths, removed on exit
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  dir_out=$out/$side
  rm -rf "$dir_out" && mkdir -p "$dir_out"
  (
    prepare "$root"
    : >"$dir_out/unchanged.diff"
    scenario 01-text "$root/acme" list
    scenario 02-json "$root/acme" list --output json
    scenario 03-with-prs "$root/acme" list --with-prs
    scenario 04-with-prs-json "$root/acme" list --with-prs --output json
    scenario 05-from-linked "$root/acme.trees/feat-a" list
    "$legacy" run up feat/a -d --yes </dev/null >/dev/null 2>&1
    scenario 06-jobs-up "$root/acme" list
    scenario 07-jobs-up-json "$root/acme" list --output json
    "$legacy" run down feat/a --yes </dev/null >/dev/null 2>&1
    scenario 08-quiet "$root/acme" list -q
    scenario 09-argument "$root/acme" list feat/a
    scenario 10-no-config "$root/bare" list
    scenario 11-no-config-json "$root/bare" list --output json
    scenario 12-not-a-repository "$root/plain" list --output json
    tty
    "$legacy" run daemon stop --yes </dev/null >/dev/null 2>&1 || true
  )
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"created_at": "[^"]*"#"created_at": "<T>"#g' \
    -e 's#[0-9a-f]{40}#<SHA>#g' \
    -e "s#$bin#<BIN>#g" \
    -e 's#wtm-next list#wtm list#g' \
    "$dir_out"/*
}

roots=()
trap 'tmux kill-session -t "wtmlist-$(basename "${root:-x}")" 2>/dev/null || true; rm -rf ${roots[@]+"${roots[@]}"}' EXIT
run_side legacy "$legacy"
run_side next "$next"
for side in legacy next; do
  if [[ -s $out/$side/unchanged.diff ]]; then
    echo "list.sh: $side changed the state on disk, see $out/$side/unchanged.diff" >&2
  fi
done
echo "$out"

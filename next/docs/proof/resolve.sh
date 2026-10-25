#!/usr/bin/env bash
# Usage: resolve.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same resolve scenarios in two identical sandboxes (the docs/demos "acme"
# project), one per binary: legacy `wtm resolve` against `wtm-next resolve`. Each
# scenario records the exit code, stdout and stderr, and the state on disk before
# and after it (refs, worktrees, every meta.json and git status, wtm's global
# directory): resolve is a read, so <side>/unchanged.diff stays empty. Then the
# picker is driven in tmux with stdout captured, as the shell function reads it,
# and `wtm go` enters worktrees through the wrapper `wtm shell-init` prints, the
# side's binary being `wtm` on the shell's PATH. Outputs differ by design (next's
# v2 contract); every difference is listed in next/docs/proof/README.md.
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
  # a second apart: legacy's picker orders worktrees by their creation, to the second
  for branch in feat/login feat/logout fix/nav feat/gone; do
    "$legacy" create "$branch" --yes </dev/null >/dev/null 2>&1
    sleep 1
  done
  local trees=$root/acme.trees
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

# tty drives the picker in tmux, stdout captured as the shell function captures it: an ambiguous name, then no name from fix/nav, left with esc; then wtm go through the wrapper shell-init prints, the side's binary being wtm on the shell's PATH.
tty() {
  local session=wtmresolve-$(basename "$root")
  screen() { tmux capture-pane -p -t "$session" | cat -s; }
  settle() {
    local now last="" tries=0
    while ((tries < 100)); do
      now=$(screen)
      [[ $now == *"$1"* && $now == "$last" ]] && return
      last=$now tries=$((tries + 1))
      sleep 0.3
    done
    echo "resolve.sh: the screen never settled on \"$1\"" >&2
    return 1
  }
  capture() {
    settle "$2"
    screen >"$dir_out/tty-$1.txt"
  }
  start() {
    tmux kill-session -t "$session" 2>/dev/null || true
    tmux new-session -d -s "$session" -x 100 -y 30 -c "$1" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
    settle '$'
  }
  # the side's binary is wtm on the shell's PATH: the lines typed are the same on both sides
  mkdir -p "$root/bin" && ln -sf "$bin" "$root/bin/wtm"
  start "$root/acme"
  tmux send-keys -t "$session" "dir=\$(wtm resolve feat/log); echo \"EXIT=\$? dir=\$dir\"" Enter
  capture 1-ambiguous 'feat/logout'
  tmux send-keys -t "$session" Enter
  capture 2-ambiguous-picked 'EXIT='
  start "$root/acme.trees/fix-nav"
  tmux send-keys -t "$session" "dir=\$(wtm resolve); echo \"EXIT=\$? dir=\$dir\"" Enter
  capture 3-none 'fix/nav'
  tmux send-keys -t "$session" Escape
  capture 4-none-left 'EXIT='
  # the wrapper as shell-init prints it, whose `command wtm` is the side's binary
  "$legacy" shell-init zsh >"$root/wrapper.zsh"
  start "$root/acme"
  tmux send-keys -t "$session" "source $root/wrapper.zsh && clear" Enter
  settle '$'
  tmux send-keys -t "$session" "wtm go feat/login; echo \"EXIT=\$?\"; pwd" Enter
  capture 5-go-exact 'acme.trees/feat-login'
  tmux send-keys -t "$session" "clear; wtm go nope; echo \"EXIT=\$?\"; pwd" Enter
  capture 6-go-no-match 'EXIT='
  tmux send-keys -t "$session" "clear; wtm go log; echo \"EXIT=\$?\"; pwd" Enter
  capture 7-go-ambiguous 'feat/logout'
  tmux send-keys -t "$session" Down
  sleep 0.5
  tmux send-keys -t "$session" Enter
  capture 8-go-picked 'acme.trees/feat-logout'
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
    local trees=$root/acme.trees
    scenario 01-exact "$root/acme" resolve feat/login
    scenario 02-exact-json "$root/acme" resolve feat/login --output json
    scenario 03-part "$root/acme" resolve logi
    scenario 04-part-json "$root/acme" resolve logi --output json
    scenario 05-main "$trees/fix-nav" resolve main
    scenario 06-ambiguous "$root/acme" resolve feat/log
    scenario 07-ambiguous-json "$root/acme" resolve feat/log --output json
    scenario 08-no-match "$root/acme" resolve nope
    scenario 09-no-match-json "$root/acme" resolve nope --output json
    scenario 10-no-arg "$trees/fix-nav" resolve
    scenario 11-no-arg-json "$trees/fix-nav" resolve --output json
    scenario 12-quiet "$root/acme" resolve feat/login -q
    scenario 13-quiet-no-match "$root/acme" resolve nope -q
    scenario 14-detached-path "$root/acme" resolve "$trees/detached"
    scenario 15-two-words "$root/acme" resolve feat login
    scenario 16-gone "$root/acme" resolve feat/gone
    scenario 17-gone-json "$root/acme" resolve feat/gone --output json
    scenario 18-no-config "$root/bare" resolve main
    scenario 19-no-config-json "$root/bare" resolve main --output json
    scenario 20-not-a-repository "$root/plain" resolve main --output json
    tty
    "$legacy" run daemon stop --yes </dev/null >/dev/null 2>&1 || true
  )
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"created_at": "[^"]*"#"created_at": "<T>"#g' \
    -e 's#[0-9a-f]{40}#<SHA>#g' \
    -e "s#$bin#<BIN>#g" \
    -e 's#wtm-next resolve#wtm resolve#g' \
    "$dir_out"/*
}

roots=()
trap 'tmux kill-session -t "wtmresolve-$(basename "${root:-x}")" 2>/dev/null || true; rm -rf ${roots[@]+"${roots[@]}"}' EXIT
run_side legacy "$legacy"
run_side next "$next"
for side in legacy next; do
  if [[ -s $out/$side/unchanged.diff ]]; then
    echo "resolve.sh: $side changed the state on disk, see $out/$side/unchanged.diff" >&2
  fi
done
echo "$out"

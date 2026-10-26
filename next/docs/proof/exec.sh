#!/usr/bin/env bash
# Usage: exec.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same exec scenarios in two identical sandboxes (the docs/demos "acme"
# project), one per binary: legacy `wtm exec` against `wtm-next exec`. The command
# of most scenarios fails in feat/b alone. After each scenario it records the exit
# code, the output and the state (branches, worktrees, every meta.json, the exec
# logs), then diffs the states. Then, in a fresh sandbox per side, it drives the
# wizard in tmux and captures its screens under <out dir>/tty/<side>. Outputs
# differ by design (next's v2 contract); the states may differ only where
# next/docs/proof/README.md says why.
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
# fails exits 3 in feat/b, saying why on stderr, and passes everywhere else.
fails='test "$(git branch --show-current)" != feat/b || { echo boom >&2; exit 3; }; echo ok'

prepare() {
  local root=$1
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=24320 WTM_DEMO_API_PORT=24321 WTM_DEMO_PROXY_PORT=24322 \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  local branch
  for branch in feat/a feat/b feat/c feat/gone; do
    "$legacy" create "$branch" --from main --yes </dev/null >/dev/null 2>&1
  done
  git worktree add -q --detach "$root/acme.trees/det"
  rm -rf "$root/acme.trees/feat-gone"
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
  echo "## worktrees"
  git worktree list --porcelain | awk '/^worktree /{print $2}'
  for meta in $(find .git/wtm/worktrees -name meta.json | sort); do
    echo "## $meta"
    grep -v created_at "$meta"
  done
  echo "## exec logs"
  (cd .git/wtm/exec 2>/dev/null && find . -type f | sort) || true
}

run_side() {
  local side=$1
  bin=$2
  local root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  dir=$out/$side
  rm -rf "$dir" && mkdir -p "$dir"
  (
    prepare "$root"
    state >"$dir/00-before.state"
    scenario 01-all-pass exec --all --yes -- 'echo hi from $(git branch --show-current)'
    scenario 02-one-fails exec feat/a feat/b feat/c --yes -- "$fails"
    scenario 03-json exec feat/a feat/b --yes --output json -- "$fails"
    scenario 04-print exec feat/a feat/b --yes --print -- "$fails"
    scenario 05-quiet -q exec feat/a feat/b --yes -- "$fails"
    scenario 06-unknown-json exec feat/a nope --yes --output json -- true
    scenario 07-no-selection exec --yes -- true
    scenario 08-no-selection-no-tty exec -- true
    scenario 09-no-command exec feat/a --yes
    scenario 10-invalid-command exec feat/a --yes -- 'if then'
    scenario 11-gone-named exec feat/gone --yes -- pwd
    scenario 12-all-gone-detached exec --all --yes -- pwd
    PORT=9999 scenario 13-caller-port exec feat/c --yes --print -- 'echo "PORT=${PORT-unset}"'
    scenario 14-jobs-negative exec feat/a --jobs -1 --yes -- true
    scenario 15-given-twice exec feat/a feat/a --yes -- true
    # next only: legacy has no dry run nor fingerprint
    scenario 16-dry-run exec feat/a feat/b --dry-run -- "$fails"
    scenario 17-json-dry-run exec feat/a --dry-run --output json -- true
    fp=$(sed -n 's/.*"fingerprint": "\([0-9a-f]*\)".*/\1/p' "$dir/17-json-dry-run.stdout")
    scenario 18-expect-fingerprint exec feat/a --expect-fingerprint "${fp:-none}" --yes -- true
  )
  local escaped=${root//\//%2F}
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e "s#(%2Fprivate)?$escaped#<SB>#g" \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#/[0-9a-f]{16}\.log#/<OP>.log#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#exit ([0-9]+), [0-9.]+m?s\)#exit \1, <DUR>)#g' \
    -e 's#"duration_ms": [0-9]+#"duration_ms": <DUR>#g' \
    -e 's#wtm-next exec#wtm exec#g' \
    "$dir"/*
}

# tty_side <side> <binary>: the wizard in tmux, in a sandbox of its own: the picker, the command, the recap, the
# outcome of a command that fails in feat/b; then a run left with esc at its recap.
tty_side() {
  local side=$1 bin=$2 root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  local tty=$out/tty/$side session=wtmexec-$side-$(basename "$root")
  rm -rf "$tty" && mkdir -p "$tty"
  (
    WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=24330 WTM_DEMO_API_PORT=24331 WTM_DEMO_PROXY_PORT=24332 \
      "$repo/docs/demos/setup.sh" >/dev/null
    export HOME=$root/home
    cd "$root/acme"
    "$legacy" create feat/a --from main --yes </dev/null >/dev/null 2>&1
    "$legacy" create feat/b --from main --yes </dev/null >/dev/null 2>&1
    cp "$bin" "$root/bin/wtmx"
    trap 'tmux kill-session -t "$session" 2>/dev/null' EXIT
    screen() { tmux capture-pane -p -t "$session" | cat -s; }
    settle() {
      local now last="" tries=0
      while ((tries < 100)); do
        now=$(screen)
        [[ $now == *"$1"* && $now == "$last" ]] && return
        last=$now tries=$((tries + 1))
        sleep 0.3
      done
      echo "exec.sh: the $side screen never settled on \"$1\"" >&2
      return 1
    }
    capture() {
      settle "$2"
      screen >"$tty/$1.txt"
    }
    start() {
      tmux kill-session -t "$session" 2>/dev/null || true
      tmux new-session -d -s "$session" -x 100 -y 40 -c "$root/acme" "zsh -f"
      tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
      settle '$'
      tmux send-keys -t "$session" "wtmx $*; echo EXIT=\$?" Enter
    }
    start exec
    capture 1-worktrees 'Step 1'
    tmux send-keys -t "$session" Down Space Down Space Enter
    capture 2-command 'Step 2'
    tmux send-keys -t "$session" -l "$fails"
    tmux send-keys -t "$session" Enter
    capture 3-recap 'No, cancel'
    tmux send-keys -t "$session" Enter
    capture 4-outcome 'EXIT=1'
    start exec feat/a -- true
    settle 'No, cancel'
    tmux send-keys -t "$session" Escape
    capture 5-left 'EXIT=19'
  )
  sed -i '' -E -e "s#(/private)?$root#<SB>#g" -e 's#wtmx#wtm#g' -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#exit ([0-9]+), [0-9.]+m?s\)#exit \1, <DUR>)#g' -e 's#/[0-9a-f]{16}\.log#/<OP>.log#g' "$tty"/*
}

mkdir -p "$out"
roots=()
trap 'rm -rf ${roots[@]+"${roots[@]}"}' EXIT
run_side legacy "$legacy"
run_side next "$next"
tty_side legacy "$legacy"
tty_side next "$next"
{
  for state in "$out"/legacy/*.state; do
    name=$(basename "$state" .state)
    echo "=== $name: exit legacy $(cat "$out/legacy/$name.exit" 2>/dev/null), next $(cat "$out/next/$name.exit" 2>/dev/null)"
    diff -u --label "legacy/$name" --label "next/$name" "$state" "$out/next/$name.state" || true
  done
} >"$out/states.diff"
echo "$out/states.diff"

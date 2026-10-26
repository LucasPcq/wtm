#!/usr/bin/env bash
# Usage: fast-forward.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same fast-forward scenarios in two identical sandboxes (the docs/demos
# "acme" project with a bare origin a teammate pushed to), one per binary: legacy
# `wtm fast-forward` against `wtm-next fast-forward`. After each scenario it
# records the exit code, the output and the state (each branch against origin's,
# the worktrees, a rebase under way), then diffs the states. Then, in a fresh
# sandbox per side, it drives the wizard in tmux and captures its screens under
# <out dir>/tty/<side>. Outputs differ by design (next's v2 contract); the states
# may differ only where next/docs/proof/README.md says why.
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

# sandbox <root> <port base>: the acme project at root with a bare origin holding main and each branch, then a
# teammate's commit pushed to the branches given, which the sandbox has not fetched.
sandbox() {
  local root=$1 ports=$2
  shift 2
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$ports WTM_DEMO_API_PORT=$((ports + 1)) WTM_DEMO_PROXY_PORT=$((ports + 2)) \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  git init -q --bare "$root/origin.git"
  git remote add origin "$root/origin.git"
  git push -q origin main
  git clone -q "$root/origin.git" "$root/teammate"
}

# branch <name> [pushed]: a worktree on name from main, pushed to origin unless told otherwise.
branch() {
  "$legacy" create "$1" --from main --yes </dev/null >/dev/null 2>&1
  [[ ${2:-pushed} == local ]] || git push -q origin "$1"
}

# pushed <branch> [file]: the teammate's commit on origin's branch, adding file (a commit of its own name by default).
pushed() {
  local file=${2:-$(echo "$1" | tr / -).txt}
  git -C "$root/teammate" fetch -q origin "$1"
  git -C "$root/teammate" checkout -q -B "$1" "origin/$1"
  echo upstream >"$root/teammate/$file"
  git -C "$root/teammate" add "$file"
  git -C "$root/teammate" -c user.email=mate@acme.dev -c user.name=Mate commit -qm "upstream $1"
  git -C "$root/teammate" push -q origin "$1"
}

# commit <worktree> <name>: one commit of a file of that name in the worktree.
commit() {
  echo "$2" >"$1/$2.txt"
  git -C "$1" add "$2.txt"
  git -C "$1" -c user.email=dev@acme.dev -c user.name=Dev commit -qm "$2"
}

prepare() {
  root=$1
  sandbox "$root" 24400
  local trees=$root/acme.trees name
  for name in feat/behind feat/uptodate feat/ahead feat/diverged feat/dirty feat/clash feat/rebasing feat/json feat/quiet feat/dry feat/fp feat/all; do
    branch "$name"
  done
  branch feat/noremote local
  git branch feat/loose main && git push -q origin feat/loose
  for name in feat/behind feat/diverged feat/dirty feat/rebasing feat/loose feat/json feat/quiet feat/dry feat/fp feat/all; do
    pushed "$name"
  done
  pushed feat/clash clash.txt
  commit "$trees/feat-ahead" ahead
  commit "$trees/feat-diverged" diverged
  echo "wip" >"$trees/feat-dirty/wip.txt"
  echo "mine" >"$trees/feat-clash/clash.txt"
  git -C "$trees/feat-rebasing" -c user.email=dev@acme.dev -c user.name=Dev rebase -q --exec false --root >/dev/null 2>&1 || true
}

scenario() {
  local name=$1
  shift
  local code=0
  "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr" || code=$?
  echo "$code" >"$dir/$name.exit"
  state >"$dir/$name.state"
}

# state: each local branch against origin's as origin has it now (not as last fetched), the worktrees, a rebase under way.
state() {
  local name local_tip remote_tip
  echo "## branches against origin"
  for name in $(git branch --format='%(refname:short)'); do
    local_tip=$(git rev-parse "$name")
    remote_tip=$(git --git-dir="$root/origin.git" rev-parse -q --verify "refs/heads/$name" || true)
    case $remote_tip in
      "") echo "$name: not on origin" ;;
      "$local_tip") echo "$name: at origin" ;;
      *) echo "$name: not at origin" ;;
    esac
  done
  echo "## worktrees"
  git worktree list --porcelain | awk '/^worktree /{print $2} /^branch /{print "  " $2} /^detached/{print "  detached"}'
  echo "## rebase under way"
  [[ -d $(git -C "$root/acme.trees/feat-rebasing" rev-parse --git-path rebase-merge) ]] && echo "feat/rebasing" || echo "none"
}

run_side() {
  local side=$1
  bin=$2
  local box
  box=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$box")
  dir=$out/$side
  rm -rf "$dir" && mkdir -p "$dir"
  (
    prepare "$box"
    state >"$dir/00-before.state"
    scenario 01-behind fast-forward feat/behind --yes
    scenario 02-up-to-date fast-forward feat/uptodate --yes
    scenario 03-ahead fast-forward feat/ahead --yes
    scenario 04-diverged fast-forward feat/diverged --yes
    scenario 05-not-on-origin fast-forward feat/noremote --yes
    scenario 06-dirty-refused fast-forward feat/dirty --yes
    scenario 07-dirty-forced fast-forward feat/dirty --yes --force
    scenario 08-clash-forced fast-forward feat/clash --yes --force
    scenario 09-rebasing fast-forward feat/rebasing --yes
    scenario 10-no-worktree fast-forward feat/loose --yes
    scenario 11-twice fast-forward feat/uptodate feat/uptodate --yes --output json
    scenario 12-part-of-a-branch fast-forward uptodate --yes
    scenario 13-json fast-forward feat/json --yes --output json
    scenario 14-json-without-yes fast-forward feat/json --output json
    scenario 15-json-refused fast-forward feat/clash --yes --output json
    scenario 16-quiet -q fast-forward feat/quiet --yes
    scenario 17-alias ff feat/dry --yes --dry-run
    # next only: legacy has no dry run nor fingerprint
    scenario 18-json-dry-run fast-forward feat/fp --dry-run --output json
    fp=$(sed -n 's/.*"fingerprint": "\([0-9a-f]*\)".*/\1/p' "$dir/18-json-dry-run.stdout")
    scenario 19-expect-fingerprint fast-forward feat/fp --expect-fingerprint "${fp:-none}" --yes
    scenario 20-stale-fingerprint fast-forward feat/dry --expect-fingerprint 0123456789abcdef01234567 --yes
    # next moves what is behind: run late, so the scenarios before it keep their worktrees behind
    scenario 21-no-worktree-named fast-forward
    scenario 22-all fast-forward --all --yes
  )
  sed -i '' -E \
    -e "s#(/private)?$box#<SB>#g" \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{40}#<SHA>#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#"(old|new)_tip": "[0-9a-f]+"#"\1_tip": "<SHA>"#g' \
    -e 's#[0-9a-f]{7}\.\.[0-9a-f]{7}#<SHA>..<SHA>#g' \
    -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#wtm-next (fast-forward|ff)#wtm \1#g' \
    "$dir"/*
}

# tty_side <side> <binary>: the wizard in tmux, in a sandbox of its own: the picker, the recap, the outcome; then a
# wizard left with esc.
tty_side() {
  local side=$1 bin=$2 box
  box=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$box")
  local tty=$out/tty/$side session=wtmff-$side-$(basename "$box")
  rm -rf "$tty" && mkdir -p "$tty"
  (
    root=$box
    sandbox "$box" 24410
    branch feat/tty-behind
    branch feat/tty-latest
    branch feat/tty-ahead
    pushed feat/tty-behind
    commit "$box/acme.trees/feat-tty-ahead" ahead
    # the picker reads origin as last fetched: fetched here, so it shows the push
    git fetch -q origin
    cp "$bin" "$box/bin/wtmx"
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
      echo "fast-forward.sh: the $side screen never settled on \"$1\"" >&2
      return 1
    }
    capture() {
      settle "$2"
      screen >"$tty/$1.txt"
    }
    start() {
      tmux kill-session -t "$session" 2>/dev/null || true
      tmux new-session -d -s "$session" -x 100 -y 40 -c "$box/acme" "zsh -f"
      tmux send-keys -t "$session" "export HOME=$box/home PATH=$box/bin:\$PATH PS1='$ ' && clear" Enter
      settle '$'
      tmux send-keys -t "$session" "wtmx $*; echo EXIT=\$?" Enter
    }
    start fast-forward
    if [[ $side == legacy ]]; then
      capture 1-worktrees 'Select worktrees'
      # legacy's picker lists the worktrees in no stable order: every one is picked
      tmux send-keys -t "$session" a Enter
    else
      capture 1-worktrees 'Step 1'
      tmux send-keys -t "$session" Enter
    fi
    capture 2-recap 'No, cancel'
    tmux send-keys -t "$session" Enter
    capture 3-outcome 'EXIT=0'
    start fast-forward feat/tty-latest
    settle 'No, cancel'
    tmux send-keys -t "$session" Escape
    capture 4-left 'EXIT=19'
  )
  sed -i '' -E -e "s#(/private)?$box#<SB>#g" -e 's#wtmx#wtm#g' -e 's#[0-9a-f]{7,40}\.\.[0-9a-f]{7,40}#<SHA>..<SHA>#g' "$tty"/*
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

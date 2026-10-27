#!/usr/bin/env bash
# Usage: reparent.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same reparent scenarios in two identical sandboxes (the docs/demos "acme"
# project), one per binary: legacy `wtm reparent` against `wtm-next reparent`. After
# each scenario it records the exit code, the output and the state (branches,
# worktrees, every meta.json), then diffs the states. Then, in a fresh sandbox per
# side, it drives the wizard in tmux and captures its screens under <out dir>/tty/<side>.
# Outputs differ by design (next's v2 contract); the states may differ only where
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

# prepare <root>: the acme project with a bare origin, the stack main → feat/a → feat/b → feat/c and feat/x on
# main, raw (made by git, no record), a local branch topic no worktree holds and remote-only, a branch only origin has.
prepare() {
  local root=$1
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=24330 WTM_DEMO_API_PORT=24331 WTM_DEMO_PROXY_PORT=24332 \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home PATH=$root/bin:$PATH
  cd "$root/acme"
  git init -q --bare "$root/origin.git"
  git remote add origin "$root/origin.git"
  git push -q origin main
  git branch remote-only && git push -q origin remote-only && git branch -D -q remote-only
  git branch topic
  "$legacy" create feat/a --from main --yes </dev/null >/dev/null 2>&1
  "$legacy" create feat/b --from feat/a --yes </dev/null >/dev/null 2>&1
  "$legacy" create feat/c --from feat/b --yes </dev/null >/dev/null 2>&1
  "$legacy" create feat/x --from main --yes </dev/null >/dev/null 2>&1
  git worktree add -q -b raw "$root/acme.trees/raw" main
}

# parent <branch> <parent>: writes parent into the meta.json of branch by hand, as a cycle only can be.
parent() {
  python3 - ".git/wtm/worktrees/${1//\//%2F}/meta.json" "$2" <<'PY'
import json, sys
meta = json.load(open(sys.argv[1]))
meta["source_branch"] = sys.argv[2]
json.dump(meta, open(sys.argv[1], "w"), indent=2)
PY
}

# scenario <name> <args…>: runs the binary from $at, the repository's root when unset, then records its state.
scenario() {
  local name=$1
  shift
  local code=0
  (cd "${at:-.}" && "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr") || code=$?
  echo "$code" >"$dir/$name.exit"
  state >"$dir/$name.state"
}

state() {
  echo "## branches"
  git branch --format='%(refname:short)'
  echo "## worktrees"
  git worktree list --porcelain | awk '/^worktree /{print $2} /^detached/{print "  detached"}'
  for meta in $(find .git/wtm/worktrees -name meta.json | sort); do
    echo "## $meta"
    grep -v created_at "$meta"
  done
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
    scenario 01-dry-run reparent feat/c --to main --dry-run
    scenario 02-one reparent feat/c --to main --yes
    scenario 03-batch reparent feat/b feat/x --to main --yes
    scenario 04-json reparent feat/b --to feat/a --yes --output json
    scenario 05-cycle reparent feat/a --to feat/b --yes
    scenario 06-cycle-json reparent feat/a --to feat/b --yes --output json
    scenario 07-main reparent main --to feat/a --yes
    scenario 08-unrecorded reparent raw --to main --yes
    scenario 09-remote-bare reparent feat/c --to remote-only --yes
    scenario 10-remote reparent feat/c --to origin/remote-only --yes
    scenario 11-no-parent reparent feat/c --yes
    at=$root/acme.trees/feat-c scenario 12-from-inside reparent --to main --yes
    scenario 13-converge reparent feat/c --to main --yes
    git worktree add -q --detach "$root/acme.trees/detached" main
    scenario 14-detached-elsewhere reparent feat/c --to feat/x --yes
    git worktree remove "$root/acme.trees/detached"
    scenario 15-converge reparent feat/c --to feat/x --yes
    parent feat/a feat/b
    scenario 16-cycle-elsewhere reparent feat/x --to topic --yes
    scenario 17-out-of-cycle reparent feat/a --to main --yes
    scenario 18-converge reparent feat/x --to topic --yes
    sed -i '' 's/^base_branch = "main"/base_branch = "feat\/x"/' .git/wtm/config.toml
    scenario 19-base-branch reparent feat/x --to main --yes
  )
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{40}#<SHA>#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#wtm-next reparent#wtm reparent#g' \
    "$dir"/*
}

# tty_side <side> <binary>: the wizard in tmux, in a sandbox of its own: the worktrees, feat/b checked, the new
# parent, the recap, the outcome.
tty_side() {
  local side=$1 bin=$2 root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  local tty=$out/tty/$side session=wtmreparent-$side-$(basename "$root")
  rm -rf "$tty" && mkdir -p "$tty"
  (
    prepare "$root"
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
      echo "reparent.sh: the $side screen never settled on \"$1\"" >&2
      return 1
    }
    capture() {
      settle "$2"
      screen >"$tty/$1.txt"
    }
    # highlight <text>: moves down the list until the row under the cursor (▌) holds text.
    highlight() {
      local tries=0
      until screen | grep '▌' | grep -q -- "$1"; do
        ((tries++ < 20)) || { echo "reparent.sh: no row of $1 on the $side screen" >&2; return 1; }
        tmux send-keys -t "$session" Down
        sleep 0.3
      done
    }
    tmux new-session -d -s "$session" -x 100 -y 40 -c "$root/acme" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "wtmx reparent; echo EXIT=\$?" Enter
    capture 1-worktrees 'feat/x'
    highlight feat/b
    tmux send-keys -t "$session" Space Enter
    capture 2-parent 'Step 2'
    tmux send-keys -t "$session" Enter
    capture 3-recap 'No, cancel'
    tmux send-keys -t "$session" Enter
    capture 4-outcome 'EXIT='
  )
  sed -i '' -E -e "s#(/private)?$root#<SB>#g" -e 's#wtmx#wtm#g' -e 's#[0-9a-f]{24}#<FP>#g' "$tty"/*
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

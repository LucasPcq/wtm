#!/usr/bin/env bash
# Usage: clean.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same clean scenarios in two identical sandboxes (the docs/demos "acme"
# project), one per binary: legacy `wtm clean` against `wtm-next clean`. After
# each scenario it records the exit code, the output and the state (branches,
# worktrees and their locks, every meta.json, repos.json), then diffs the states.
# Then, in a fresh sandbox per side, it drives the wizard in tmux and captures its
# screens under <out dir>/tty/<side>. Outputs differ by design (next's v2
# contract); the states may differ only where next/docs/proof/README.md says why.
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
hook='case {{branch}} in feat/bad*) echo failing; exit 3;; *) echo tearing down {{branch}};; esac'

prepare() {
  local root=$1
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=24300 WTM_DEMO_API_PORT=24301 WTM_DEMO_PROXY_PORT=24302 \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  python3 - "$hook" <<'PY'
import sys
path = ".git/wtm/config.toml"
text = open(path).read().replace("on_clean = [\n]", 'on_clean = [{ cmd = "%s" }]' % sys.argv[1].replace('"', '\\"'), 1)
open(path, "w").write(text)
PY
  grep -q 'tearing down' .git/wtm/config.toml || { echo "$0: no on_clean list to fill in config.toml" >&2; return 1; }
  git init -q --bare "$root/origin.git"
  git remote add origin "$root/origin.git"
  git push -q origin main
  local branch
  for branch in feat/plain feat/dirty feat/unpushed feat/local feat/locked feat/parent feat/p2 feat/a feat/b feat/c feat/bad feat/json feat/jsonny feat/quiet feat/dry feat/fp feat/detached; do
    "$legacy" create "$branch" --from main --yes </dev/null >/dev/null 2>&1
  done
  "$legacy" create feat/child --from feat/parent --yes </dev/null >/dev/null 2>&1
  "$legacy" create feat/child2 --from feat/parent --yes </dev/null >/dev/null 2>&1
  "$legacy" create feat/c2 --from feat/p2 --yes </dev/null >/dev/null 2>&1
  local trees=$root/acme.trees
  echo "wip" >"$trees/feat-dirty/wip.txt"
  echo "wip" >"$trees/feat-c/wip.txt"
  commit "$trees/feat-unpushed" pushed
  git -C "$trees/feat-unpushed" push -q origin feat/unpushed
  commit "$trees/feat-unpushed" unpushed
  commit "$trees/feat-local" local
  git worktree lock "$trees/feat-locked"
  git -C "$trees/feat-detached" checkout -q --detach
}

# commit <worktree> <name>: one commit of a file of that name in the worktree.
commit() {
  echo "$2" >"$1/$2.txt"
  git -C "$1" add "$2.txt"
  git -C "$1" -c user.email=dev@acme.dev -c user.name=Dev commit -qm "$2"
}

# forget <branch>: removes the worktree of branch, the branch and its record, whatever a side left of them, so the
# scenarios after one that ends apart on purpose start from the same state.
forget() {
  local path
  path=$(git worktree list --porcelain | awk -v ref="branch refs/heads/$1" '/^worktree /{at=$2} $0==ref{print at}')
  [[ -z $path ]] || git worktree remove --force "$path"
  git branch -D -q "$1" 2>/dev/null || true
  rm -rf ".git/wtm/worktrees/${1//\//%2F}"
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
  git worktree list --porcelain | awk '/^worktree /{print $2} /^locked/{print "  locked"}'
  for meta in $(find .git/wtm/worktrees -name meta.json | sort); do
    echo "## $meta"
    grep -v created_at "$meta"
  done
  echo "## repos.json"
  find "$HOME" -name repos.json -exec cat {} \; | grep -v '"last_seen\|"added_at' || true
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
    scenario 01-one clean feat/plain --yes
    scenario 02-dirty-refused clean feat/dirty --yes
    scenario 03-dirty-forced clean feat/dirty --yes --force
    scenario 04-unpushed-refused clean feat/unpushed --yes
    scenario 05-locked-refused clean feat/locked --yes
    scenario 06-locked-forced clean feat/locked --yes --force
    scenario 07-never-pushed clean feat/local --yes
    forget feat/local
    scenario 08-orphans-left clean feat/parent --yes
    scenario 09-orphans-reparented clean feat/p2 --yes --reparent-children
    scenario 10-many-one-unsafe clean feat/a feat/c --yes
    scenario 11-many clean feat/a feat/b --yes
    scenario 12-absent clean feat/nope --yes
    scenario 13-main clean main --yes
    scenario 14-hook-fails clean feat/bad --yes
    scenario 15-json clean feat/json --yes --output json
    scenario 16-json-without-yes clean feat/jsonny --output json
    forget feat/jsonny
    scenario 17-json-refused clean feat/c --yes --output json
    scenario 18-no-branch clean
    scenario 19-quiet -q clean feat/quiet --yes
    scenario 20-dry-run clean feat/dry --dry-run
    scenario 21-keep-data clean feat/dry --yes --keep-data
    forget feat/dry
    scenario 22-detached-by-path clean "$root/acme.trees/feat-detached" --yes
    git worktree remove --force "$root/acme.trees/feat-detached" 2>/dev/null || true
    # next only: legacy has no dry run nor fingerprint
    scenario 23-json-dry-run clean feat/fp --dry-run --output json
    fp=$(sed -n 's/.*"fingerprint": "\([0-9a-f]*\)".*/\1/p' "$dir/23-json-dry-run.stdout")
    scenario 24-expect-fingerprint clean feat/fp --expect-fingerprint "${fp:-none}" --yes
    forget feat/fp
    scenario 25-stale-fingerprint clean feat/c --force --expect-fingerprint 0123456789abcdef01234567 --yes
    git branch feat/orphan main
    scenario 26-branch-without-worktree clean feat/orphan --yes
  )
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"created_at": "[^"]*"#"created_at": "<T>"#g' \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#wtm-next clean#wtm clean#g' \
    "$dir"/*
}

# tty_side <side> <binary>: the wizard in tmux, in a sandbox of its own: the picker, the children question, the
# unsafe worktree's (next asks a Force step where legacy's recap offers to force), the recap, the outcome; then a
# wizard left with esc.
tty_side() {
  local side=$1 bin=$2 root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  local tty=$out/tty/$side session=wtmclean-$side-$(basename "$root")
  rm -rf "$tty" && mkdir -p "$tty"
  (
    WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=24310 WTM_DEMO_API_PORT=24311 WTM_DEMO_PROXY_PORT=24312 \
      "$repo/docs/demos/setup.sh" >/dev/null
    export HOME=$root/home
    cd "$root/acme"
    "$legacy" create feat/tty-parent --from main --yes </dev/null >/dev/null 2>&1
    "$legacy" create feat/tty-dirty --from main --yes </dev/null >/dev/null 2>&1
    "$legacy" create feat/tty-child --from feat/tty-parent --yes </dev/null >/dev/null 2>&1
    echo "wip" >"$root/acme.trees/feat-tty-dirty/wip.txt"
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
      echo "clean.sh: the $side screen never settled on \"$1\"" >&2
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
    start clean
    capture 1-worktrees 'Step 1'
    tmux send-keys -t "$session" Down Space Down Space Enter
    capture 2-reparent 'Step 2'
    if [[ $side == legacy ]]; then
      tmux send-keys -t "$session" Enter
      capture 3-recap 'No, cancel'
      tmux send-keys -t "$session" Down Enter
    else
      tmux send-keys -t "$session" Down Enter
      capture 3-force 'Step 3'
      tmux send-keys -t "$session" Down Enter
      capture 4-recap 'No, cancel'
      tmux send-keys -t "$session" Enter
    fi
    capture 5-outcome 'EXIT=0'
    start clean feat/tty-child
    settle 'No, cancel'
    tmux send-keys -t "$session" Escape
    capture 6-left 'EXIT=19'
  )
  sed -i '' -E -e "s#(/private)?$root#<SB>#g" -e 's#wtmx#wtm#g' "$tty"/*
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

#!/usr/bin/env bash
# Usage: sync.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same sync scenarios in two identical sandboxes (the docs/demos "acme" project with a bare origin a
# teammate pushed to), one per binary: legacy `wtm sync` against `wtm-next sync`. After each scenario it records the
# exit code, the output and the state (each branch's commits, whether it stands at origin's tip, a rebase under way,
# the uncommitted files), then diffs the states. Then, in a fresh sandbox per side, it drives the wizard in tmux and
# captures its screens under <out dir>/tty/<side>. Outputs differ by design (next's v2 contract); the states may
# differ only where next/docs/proof/README.md says why.
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

# sandbox <root> <port base>: the acme project at root, shared.txt on main, with a bare origin holding main and a
# teammate's clone of it.
sandbox() {
  local root=$1 ports=$2
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$ports WTM_DEMO_API_PORT=$((ports + 1)) WTM_DEMO_PROXY_PORT=$((ports + 2)) \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  commit . shared.txt base
  git init -q --bare "$root/origin.git"
  git remote add origin "$root/origin.git"
  git push -q origin main
  git clone -q "$root/origin.git" "$root/teammate"
}

# commit <dir> <file> <content>: one commit of file with content in the worktree at dir.
commit() {
  echo "$3" >"$1/$2"
  git -C "$1" add "$2"
  git -C "$1" -c user.email=dev@acme.dev -c user.name=Dev commit -qm "$2: $3"
}

# branch <name> <parent>: a worktree on name created from parent, as wtm records it.
branch() {
  "$legacy" create "$1" --from "$2" --yes </dev/null >/dev/null 2>&1
}

# pushed <branch> <file> <content>: the teammate's commit on origin's branch, which the sandbox has not fetched.
pushed() {
  git -C "$root/teammate" fetch -q origin "$1"
  git -C "$root/teammate" checkout -q -B "$1" "origin/$1"
  echo "$3" >"$root/teammate/$2"
  git -C "$root/teammate" add "$2"
  git -C "$root/teammate" -c user.email=mate@acme.dev -c user.name=Mate commit -qm "$2: $3 (teammate)"
  git -C "$root/teammate" push -q origin "$1"
}

# spaced <branch>...: their worktrees' created_at one second apart, in that order: both list siblings by it.
spaced() {
  python3 - "$@" <<'PY'
import json, sys
for i, branch in enumerate(sys.argv[1:]):
    path = ".git/wtm/worktrees/%s/meta.json" % branch.replace("/", "%2F")
    meta = json.load(open(path))
    meta["created_at"] = "2026-10-01T09:00:%02dZ" % i
    json.dump(meta, open(path, "w"), indent=2)
PY
}

# parent <branch> <parent>: the parent the meta record of branch names, rewritten.
parent() {
  python3 - "$1" "$2" <<'PY'
import json, sys
path = ".git/wtm/worktrees/%s/meta.json" % sys.argv[1].replace("/", "%2F")
meta = json.load(open(path))
meta["source_branch"] = sys.argv[2]
json.dump(meta, open(path, "w"), indent=2)
PY
}

# prepare <root>: two stacks on main, main → feat/s1 → feat/s2 and main → feat/k1 → feat/k2 → feat/k3, every
# branch pushed, then a teammate's change to shared.txt on main, which feat/k2 changes its own way; feat/dirty →
# feat/dirty-kid with an untracked file in feat/dirty, feat/solo, and feat/gone, whose directory is deleted.
prepare() {
  root=$1
  sandbox "$root" 24500
  local trees=$root/acme.trees name
  branch feat/s1 main
  commit "$trees/feat-s1" s1.txt s1
  branch feat/s2 feat/s1
  commit "$trees/feat-s2" s2.txt s2
  branch feat/k1 main
  commit "$trees/feat-k1" k1.txt k1
  branch feat/k2 feat/k1
  commit "$trees/feat-k2" shared.txt "k2's line"
  branch feat/k3 feat/k2
  commit "$trees/feat-k3" k3.txt k3
  branch feat/dirty main
  commit "$trees/feat-dirty" dirty.txt dirty
  branch feat/dirty-kid feat/dirty
  branch feat/solo main
  commit "$trees/feat-solo" solo.txt solo
  branch feat/gone main
  spaced feat/s1 feat/s2 feat/k1 feat/k2 feat/k3 feat/dirty feat/dirty-kid feat/solo feat/gone
  for name in feat/s1 feat/s2 feat/k1 feat/k2 feat/k3 feat/dirty feat/solo; do
    git push -q origin "$name"
  done
  pushed main shared.txt "main's line"
  echo "wip" >"$trees/feat-dirty/wip.txt"
  rm -rf "$trees/feat-gone"
}

scenario() {
  local name=$1
  shift
  local code=0
  "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr" || code=$?
  echo "$code" >"$dir/$name.exit"
  state >"$dir/$name.state"
}

# state: each branch's commits since main's first, whether it stands at origin's tip as origin has it now, each
# worktree with a rebase under way and its uncommitted files.
state() {
  local name local_tip remote_tip path where
  echo "## branches"
  for name in $(git branch --format='%(refname:short)' | LC_ALL=C sort); do
    local_tip=$(git rev-parse "$name")
    remote_tip=$(git --git-dir="$root/origin.git" rev-parse -q --verify "refs/heads/$name" || true)
    case $remote_tip in
      "") where="not on origin" ;;
      "$local_tip") where="at origin" ;;
      *) where="not at origin" ;;
    esac
    echo "$name ($where): $(git log --reverse --format=%s "$name" | tail -n +2 | paste -sd '|' -)"
  done
  echo "## worktrees"
  git worktree list --porcelain | awk '/^worktree /{print $2}' | while read -r path; do
    [[ -d $path ]] || { echo "$path: gone"; continue; }
    where=""
    [[ -d $(git -C "$path" rev-parse --git-path rebase-merge) ]] && where=" rebasing"
    echo "$path$where $(git -C "$path" status --porcelain | LC_ALL=C sort | paste -sd ' ' -)"
  done
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
    local trees=$box/acme.trees
    state >"$dir/00-before.state"
    scenario 01-dry-run sync feat/s1 --dry-run
    scenario 02-named-alone sync feat/s1 --descendants=false --yes
    scenario 03-stack sync feat/s1 --yes
    scenario 04-conflict-mid-stack sync feat/k1 --yes
    scenario 05-conflict-json sync feat/k2 --yes --output json
    scenario 06-keep-conflict sync feat/k2 --keep-conflict --yes
    scenario 07-rebase-under-way sync feat/k2 --yes
    # the user resolves the conflict left in feat/k2, then syncs again
    echo "resolved" >"$trees/feat-k2/shared.txt"
    git -C "$trees/feat-k2" add shared.txt
    GIT_EDITOR=true git -C "$trees/feat-k2" -c user.email=dev@acme.dev -c user.name=Dev rebase --continue >/dev/null 2>&1
    scenario 08-resumed sync feat/k2 --yes
    scenario 09-dirty-named sync feat/dirty --yes
    scenario 10-dirty-forced sync feat/dirty --force --yes
    scenario 11-gone sync feat/gone --yes
    scenario 12-base sync main --yes
    scenario 13-unknown sync feat/nope --yes
    scenario 14-part-of-a-branch sync solo --dry-run
    scenario 15-json-dry-run sync feat/solo --dry-run --output json
    fp=$(sed -n 's/.*"fingerprint": "\([0-9a-f]*\)".*/\1/p' "$dir/15-json-dry-run.stdout")
    scenario 16-expect-fingerprint sync feat/solo --expect-fingerprint "${fp:-none}" --yes
    scenario 17-stale-fingerprint sync feat/s2 --expect-fingerprint 0123456789abcdef01234567 --yes
    scenario 18-push sync feat/s1 --push --yes
    scenario 19-quiet -q sync feat/s1 --yes
    scenario 20-no-terminal sync
    git worktree add -q --detach "$trees/detached" main
    scenario 21-beside-a-detached sync feat/solo --yes
    parent feat/s1 feat/s2
    scenario 22-cycle-elsewhere sync feat/solo --yes
    scenario 23-cycle-named sync feat/s1 --yes
    scenario 24-all sync --all --yes
  )
  sed -i '' -E \
    -e "s#(/private)?$box#<SB>#g" \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{40}#<SHA>#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#"(base_old_tip|base_new_tip|old_tip|new_tip|onto_tip)": "[0-9a-f]+"#"\1": "<SHA>"#g' \
    -e 's#[0-9a-f]{7} → [0-9a-f]{7}#<SHA> → <SHA>#g' \
    -e 's#  main  [0-9a-f]{7}  #  main  <SHA>  #g' \
    -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    "$dir"/*
}

# tty_side <side> <binary>: the wizard in tmux, in a sandbox of its own: sync feat/s1 asked through to its recap,
# then the outcome; then a sync with nothing to do.
tty_side() {
  local side=$1 bin=$2 box
  box=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$box")
  local tty=$out/tty/$side session=wtmsync-$side-$(basename "$box")
  rm -rf "$tty" && mkdir -p "$tty"
  (
    root=$box
    sandbox "$box" 24510
    branch feat/tty-a main
    commit "$box/acme.trees/feat-tty-a" a.txt a
    branch feat/tty-b feat/tty-a
    commit "$box/acme.trees/feat-tty-b" b.txt b
    spaced feat/tty-a feat/tty-b
    commit "$box/acme" main.txt "moved on"
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
      echo "sync.sh: the $side screen never settled on \"$1\"" >&2
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
    start sync feat/tty-a
    if [[ $side == legacy ]]; then
      capture 1-first 'On conflict'
    else
      capture 1-first 'Descendants'
    fi
    # each question taken at its default, up to the recap
    local tries=0
    until [[ $(screen) == *"No, cancel"* ]] || ((tries > 8)); do
      tmux send-keys -t "$session" Enter
      sleep 1
      tries=$((tries + 1))
    done
    capture 2-recap 'No, cancel'
    tmux send-keys -t "$session" Enter
    if [[ $side == legacy ]]; then
      # legacy asks whether to push once the rebases ran: kept local
      capture 3-push 'Keep local'
      tmux send-keys -t "$session" Enter
    fi
    capture 4-outcome 'EXIT=0'
    start sync feat/tty-b --dry-run
    capture 5-dry-run 'EXIT='
  )
  sed -i '' -E -e "s#(/private)?$box#<SB>#g" -e 's#wtmx#wtm#g' -e 's#[0-9a-f]{24}#<FP>#g' -e 's#[0-9a-f]{7}( |  |$)#<SHA>\1#g' "$tty"/*
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

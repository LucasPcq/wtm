#!/usr/bin/env bash
# Usage: prune.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same prune scenarios in two identical sandboxes (the docs/demos "acme"
# project), one per binary: legacy `wtm prune` against `wtm-next prune`. A fake gh
# answers both from the sandbox's prs.json: legacy's query with its --jq applied,
# next's as GitHub's GraphQL answer. After each scenario it records the exit code,
# the output and the state (branches, worktrees and their locks, every meta.json),
# then diffs the states. Then, in a fresh sandbox per side, it drives the wizard in
# tmux and captures its screens under <out dir>/tty/<side>. Outputs differ by
# design (next's v2 contract); the states may differ only where
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
hook='case {{branch}} in feat/bad*) echo failing; exit 3;; *) echo tearing down {{branch}};; esac'

# fake_gh <dir>: writes in dir a gh, signed in, that answers the pull requests of <dir>/../prs.json.
fake_gh() {
  mkdir -p "$1"
  cat >"$1/gh" <<'SH'
#!/usr/bin/env bash
[[ $1 == auth ]] && exit 0
[[ $1 == api && $2 == graphql ]] || { echo '[]'; exit 0; }
exec python3 - "$(dirname "$0")/../prs.json" "$@" <<'PY'
import json, sys
nodes, args = json.load(open(sys.argv[1])), sys.argv[2:]
if "--jq" in args:
    print(json.dumps(nodes))
    sys.exit()
aliases = {}
for flag, value in zip(args, args[1:]):
    name, _, branch = value.partition("=")
    if flag == "-f" and name.startswith("b") and name[1:].isdigit():
        aliases[name] = {"nodes": [n for n in nodes if n["headRefName"] == branch]}
print(json.dumps({"data": {"repository": aliases}}))
PY
SH
  chmod +x "$1/gh"
}

prepare() {
  local root=$1
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=24320 WTM_DEMO_API_PORT=24321 WTM_DEMO_PROXY_PORT=24322 \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home PATH=$root/bin:$PATH
  fake_gh "$root/bin"
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
  local branch trees=$root/acme.trees
  for branch in feat/merged feat/moved feat/closed feat/gone feat/gone-work feat/dirty feat/locked feat/open feat/plain feat/bad feat/squash; do
    "$legacy" create "$branch" --from main --yes </dev/null >/dev/null 2>&1
  done
  "$legacy" create feat/child --from feat/merged --yes </dev/null >/dev/null 2>&1
  for branch in merged moved closed gone-work dirty locked open bad squash; do
    commit "$trees/feat-$branch" "$branch"
  done
  for branch in merged moved closed gone gone-work dirty locked open bad squash; do
    git -C "$trees/feat-$branch" push -q -u origin "feat/$branch"
  done
  commit "$trees/feat-moved" after-merge        # merged, one commit since
  commit "$trees/feat-gone-work" after-delete   # origin deleted its branch, a commit only it holds
  echo "wip" >"$trees/feat-dirty/wip.txt"        # merged, uncommitted changes
  git worktree lock "$trees/feat-locked"         # merged, locked
  git push -q origin --delete feat/gone feat/gone-work feat/squash
  git fetch -q origin   # the tracking refs of the deleted branches stay: only a prune of the refs drops them
  local head
  head() { git rev-parse "$1"; }
  cat >"$root/prs.json" <<JSON
[
 {"number":1,"url":"https://github.com/acme/acme/pull/1","state":"MERGED","headRefName":"feat/merged","headRefOid":"$(head feat/merged)"},
 {"number":2,"url":"https://github.com/acme/acme/pull/2","state":"MERGED","headRefName":"feat/moved","headRefOid":"$(head feat/moved~1)"},
 {"number":3,"url":"https://github.com/acme/acme/pull/3","state":"CLOSED","headRefName":"feat/closed","headRefOid":"$(head feat/closed)"},
 {"number":4,"url":"https://github.com/acme/acme/pull/4","state":"MERGED","headRefName":"feat/dirty","headRefOid":"$(head feat/dirty)"},
 {"number":5,"url":"https://github.com/acme/acme/pull/5","state":"MERGED","headRefName":"feat/locked","headRefOid":"$(head feat/locked)"},
 {"number":6,"url":"https://github.com/acme/acme/pull/6","state":"OPEN","headRefName":"feat/open","headRefOid":"$(head feat/open)"},
 {"number":7,"url":"https://github.com/acme/acme/pull/7","state":"MERGED","headRefName":"feat/bad","headRefOid":"$(head feat/bad)"},
 {"number":8,"url":"https://github.com/acme/acme/pull/8","state":"MERGED","headRefName":"feat/squash","headRefOid":"$(head feat/squash)"}
]
JSON
}

# commit <worktree> <name>: one commit of a file of that name in the worktree.
commit() {
  echo "$2" >"$1/$2.txt"
  git -C "$1" add "$2.txt"
  git -C "$1" -c user.email=dev@acme.dev -c user.name=Dev commit -qm "$2"
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
  echo "## remote branches"
  git branch -r --format='%(refname:short)'
  echo "## worktrees"
  git worktree list --porcelain | awk '/^worktree /{print $2} /^locked/{print "  locked"}'
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
    scenario 01-dry-run prune --dry-run
    scenario 02-json-dry-run prune --dry-run --output json
    scenario 03-gone-dry-run prune --gone --dry-run
    scenario 04-forced-dry-run prune --dry-run --force
    scenario 05-closed prune --closed --yes
    scenario 06-reparent-dry-run prune --merged --dry-run --reparent-children
    scenario 07-merged-hook-fails prune --merged --yes --reparent-children
    scenario 08-json prune --yes --output json
    scenario 09-again prune --yes
    # next only: legacy has no worktree argument
    scenario 10-named-unsafe prune feat/dirty --yes
    scenario 11-named-not-done prune feat/open --yes
    scenario 12-named-forced prune feat/dirty --force --yes
    scenario 13-no-terminal prune
  )
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"created_at": "[^"]*"#"created_at": "<T>"#g' \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{40}#<SHA>#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#wtm-next prune#wtm prune#g' \
    "$dir"/*
}

# tty_side <side> <binary>: the wizard in tmux, in a sandbox of its own: the picker, the children question, the
# unsafe worktree's (next asks a Force step once one is checked, where legacy drops it), the recap, the outcome.
tty_side() {
  local side=$1 bin=$2 root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  local tty=$out/tty/$side session=wtmprune-$side-$(basename "$root")
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
      echo "prune.sh: the $side screen never settled on \"$1\"" >&2
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
        ((tries++ < 20)) || { echo "prune.sh: no row of $1 on the $side screen" >&2; return 1; }
        tmux send-keys -t "$session" Down
        sleep 0.3
      done
    }
    tmux new-session -d -s "$session" -x 100 -y 40 -c "$root/acme" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "wtmx prune --merged; echo EXIT=\$?" Enter
    capture 1-worktrees 'feat/dirty'
    # feat/dirty is listed unchecked, unsafe: check it, as B3 does
    highlight feat/dirty
    tmux send-keys -t "$session" Space Enter
    capture 2-reparent 'Step 2'
    tmux send-keys -t "$session" Enter
    if [[ $side == next ]]; then
      capture 3-force 'Step 3'
      tmux send-keys -t "$session" Down Enter
    fi
    capture 4-recap 'No, cancel'
    tmux send-keys -t "$session" Enter
    capture 5-outcome 'EXIT='
  )
  sed -i '' -E -e "s#(/private)?$root#<SB>#g" -e 's#wtmx#wtm#g' -e 's#[0-9a-f]{24}#<FP>#g' -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' -e 's#^[0-9.]+m?s\)$#<DUR>)#' "$tty"/*
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

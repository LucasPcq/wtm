#!/usr/bin/env bash
# Usage: status.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same status scenarios in two identical sandboxes (the docs/demos
# "acme" project), one per binary: legacy `wtm status` against `wtm-next status`.
# Each scenario records the exit code, stdout and stderr, and the state on disk
# before and after it (refs, worktrees, every meta.json and git status, every
# .env, run.toml): status is a read, so <side>/unchanged.diff stays empty. Then
# the picker is driven in tmux. Outputs differ by design (next's v2 contract and
# the git part); every difference is listed in next/docs/proof/README.md.
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

# The ports are fixed, the two sides running one after the other, so a rerun draws the same addresses.
port=23460

prepare() {
  local root=$1
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$port WTM_DEMO_API_PORT=$((port + 1)) WTM_DEMO_PROXY_PORT=$((port + 2)) \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home GIT_AUTHOR_DATE=2026-10-16T12:00:00Z GIT_COMMITTER_DATE=2026-10-16T12:00:00Z
  cd "$root/acme"
  git init -q --bare "$root/origin.git"
  git remote add origin "$root/origin.git"
  git push -q origin main
  for branch in feat/a feat/b feat/old feat/rb; do
    "$legacy" create "$branch" --isolation isolated --yes </dev/null >/dev/null 2>&1
  done
  local trees=$root/acme.trees
  # feat/a: two commits ahead of main, which is one ahead of it; pushed, then one more; uncommitted work
  git -C "$trees/feat-a" commit -q --allow-empty -m "feat: add the login form"
  git -C "$trees/feat-a" push -q -u origin feat/a
  git -C "$trees/feat-a" commit -q --allow-empty -m "feat: wire the session"
  git commit -q --allow-empty -m "chore: bump"
  printf '# acme\n\nLogin.\n' >"$trees/feat-a/README.md"
  echo "draft" >"$trees/feat-a/notes.txt"
  git -C "$trees/feat-a" add notes.txt
  # feat/b: its web .env deleted
  rm "$trees/feat-b/apps/web/.env"
  # feat/old: created before the isolation choice
  python3 - "$root/acme/.git/wtm/worktrees" <<'PY'
import json, pathlib, sys
for meta in pathlib.Path(sys.argv[1]).glob("*old*/meta.json"):
    record = json.loads(meta.read_text())
    record.pop("isolation", None)
    meta.write_text(json.dumps(record, indent=2) + "\n")
PY
  # feat/rb: stopped mid-rebase on a conflict with main
  echo "rb" >"$trees/feat-rb/README.md"
  git -C "$trees/feat-rb" commit -q -am "rb: rewrite the readme"
  echo "main" >README.md
  git commit -q -am "chore: rewrite the readme"
  git -C "$trees/feat-rb" rebase main >/dev/null 2>&1 || true
  git worktree add -q --detach "$trees/detached"
  mkdir -p "$root/bare"
  git -C "$root/bare" init -q
}

state() {
  git for-each-ref --format='%(refname) %(objectname)'
  git worktree list --porcelain
  git worktree list --porcelain | awk '/^worktree /{print $2}' | while read -r wt; do
    [[ -d $wt ]] || continue
    echo "## $wt"
    git -C "$wt" status --porcelain=v1 --untracked-files=all
    find "$wt/apps" -name '.env' 2>/dev/null | sort | xargs shasum 2>/dev/null || true
  done
  for meta in $(find .git/wtm/worktrees -name meta.json | sort); do
    echo "## $meta"
    cat "$meta"
  done
  shasum .git/wtm/run.toml .git/wtm/config.toml
}

# fake_gh <dir>: writes in dir a gh, signed in, whose GraphQL answer gives feat/a an open pull request and feat/b a merged one, each under the alias (b0, b1…) its branch was asked by.
fake_gh() {
  mkdir -p "$1"
  cat >"$1/gh" <<'SH'
#!/bin/sh
[ "$1" = auth ] && exit 0
printf '{"data":{"repository":{'
sep=""
for arg in "$@"; do
  case "$arg" in
    b[0-9]*=*)
      alias=${arg%%=*}
      case "${arg#*=}" in
        feat/a) node='{"number":12,"url":"https://github.com/acme/acme/pull/12","state":"OPEN","headRefName":"feat/a","isCrossRepository":false,"title":"Add the login form","isDraft":false,"reviewDecision":"REVIEW_REQUIRED","commits":{"nodes":[{"commit":{"statusCheckRollup":{"contexts":{"checkRunCountsByState":[{"state":"SUCCESS","count":2},{"state":"IN_PROGRESS","count":1}],"statusContextCountsByState":[]}}}}]}}' ;;
        feat/b) node='{"number":9,"url":"https://github.com/acme/acme/pull/9","state":"MERGED","headRefName":"feat/b","isCrossRepository":false,"title":"Add the api","isDraft":false,"reviewDecision":"APPROVED","commits":{"nodes":[]}}' ;;
        *) node="" ;;
      esac
      printf '%s"%s":{"nodes":[%s]}' "$sep" "$alias" "$node"
      sep=",";;
  esac
done
printf '}}}\n'
SH
  chmod +x "$1/gh"
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

# tty drives the picker in tmux from inside feat/a: the picker on it, enter reads it; then a picker left with esc.
tty() {
  local session=wtmstatus-$(basename "$root") here=$root/acme.trees/feat-a
  screen() { tmux capture-pane -p -t "$session" | cat -s; }
  settle() {
    local now last="" tries=0
    while ((tries < 100)); do
      now=$(screen)
      [[ $now == *"$1"* && $now == "$last" ]] && return
      last=$now tries=$((tries + 1))
      sleep 0.3
    done
    echo "status.sh: the screen never settled on \"$1\"" >&2
    return 1
  }
  capture() {
    settle "$2"
    screen >"$dir_out/tty-$1.txt"
  }
  start() {
    tmux kill-session -t "$session" 2>/dev/null || true
    tmux new-session -d -s "$session" -x 100 -y 40 -c "$here" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "$bin status; echo EXIT=\$?" Enter
  }
  start
  capture 1-picker 'feat/old'
  tmux send-keys -t "$session" Enter
  capture 2-read 'EXIT='
  start
  capture 3-again 'feat/old'
  tmux send-keys -t "$session" Escape
  capture 4-left 'EXIT='
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
    local trees=$root/acme.trees
    : >"$dir_out/unchanged.diff"
    scenario 01-main "$root/acme" status
    scenario 02-ahead-behind-dirty "$root/acme" status feat/a
    scenario 03-missing-env "$root/acme" status feat/b
    scenario 04-pending-isolation "$root/acme" status feat/old
    scenario 05-detached "$trees/detached" status
    scenario 06-rebasing "$trees/feat-rb" status
    scenario 07-all "$root/acme" status --all
    scenario 08-json "$root/acme" status feat/a --output json
    scenario 09-all-json "$root/acme" status --all --output json
    scenario 10-pending-json "$root/acme" status feat/old --output json
    "$legacy" run up feat/a -d --yes </dev/null >/dev/null 2>&1
    scenario 11-jobs-up "$root/acme" status feat/a
    scenario 12-jobs-up-json "$root/acme" status feat/a --output json
    "$legacy" run down feat/a --yes </dev/null >/dev/null 2>&1
    "$legacy" run daemon stop --yes </dev/null >/dev/null 2>&1 || true
    scenario 13-quiet "$root/acme" status -q
    scenario 14-all-with-worktree "$root/acme" status feat/a --all
    scenario 15-not-found "$root/acme" status nope
    scenario 16-no-config "$root/bare" status
    cp .git/wtm/run.toml "$root/run.toml"
    printf 'addressing = "nope"\n' >>.git/wtm/run.toml
    scenario 17-malformed-run "$root/acme" status feat/a
    cp "$root/run.toml" .git/wtm/run.toml
    scenario 18-dot-from-subdirectory "$trees/feat-a/apps" status .
    # legacy's status reads no pull request: only next is asked, of a fake gh
    if [[ $side == next ]]; then
      fake_gh "$root/bin"
      PATH=$root/bin:$PATH scenario 21-pr-merged "$root/acme" status feat/b --with-prs
      PATH=$root/bin:$PATH scenario 22-pr-merged-json "$root/acme" status feat/b --with-prs --output json
      PATH=$root/bin:$PATH scenario 23-all-prs "$root/acme" status --all --with-prs
    fi
    tty
    rm -rf "$trees/feat-b"
    scenario 19-gone "$root/acme" status feat/b
    scenario 20-all-gone "$root/acme" status --all
  )
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"(created_at|at|started_at)": "[^"]*"#"\1": "<T>"#g' \
    -e 's#[0-9a-f]{40}#<SHA>#g' \
    -e 's#"sha": "[0-9a-f]+"#"sha": "<SHA>"#g' \
    -e 's#[[:<:]][0-9a-f]{7}[[:>:]] #<SHA> #g' \
    -e 's#[0-9]+[smhd] ago#<AGE> ago#g' \
    -e 's#up [0-9]+[smhd]#up <AGE>#g' \
    -e "s#$bin#<BIN>#g" \
    -e 's#wtm-next status#wtm status#g' \
    "$dir_out"/*
}

roots=()
trap 'tmux kill-session -t "wtmstatus-$(basename "${root:-x}")" 2>/dev/null || true; rm -rf ${roots[@]+"${roots[@]}"}' EXIT
run_side legacy "$legacy"
run_side next "$next"
for side in legacy next; do
  if [[ -s $out/$side/unchanged.diff ]]; then
    echo "status.sh: $side changed the state on disk, see $out/$side/unchanged.diff" >&2
  fi
done
echo "$out"

#!/usr/bin/env bash
# Usage: env.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same env scenarios in two identical sandboxes (the docs/demos "acme" monorepo, a third app beside its web
# and its api), one per binary: legacy `wtm env` against `wtm-next env`, each with its own flags where they differ
# (next/docs/proof/README.md lists why). After each scenario it records the exit code, the output and the state (every
# .env file of every worktree, each worktree's isolation), then diffs the states. Then, in a fresh sandbox per side, it
# drives the wizard in tmux and captures its screens under <out dir>/tty/<side>.
set -euo pipefail

absolute() {
  [[ -f $1 && -x $1 ]] || { echo "$0: $1 is not an executable" >&2; return 1; }
  echo "$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
}
legacy=$(absolute "${1:?wtm binary}")
next=$(absolute "${2:?wtm-next binary}")
out=${3:?out dir}
mkdir -p "$out" && out=$(cd "$out" && pwd)
repo=$(git rev-parse --show-toplevel)

# sandbox <root> <port base>: the acme project at root, with a docs app whose .env holds a title, an api .env holding a
# secret and a mailer address on the stack's mailpit port that no link follows.
sandbox() {
  local root=$1 ports=$2
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$ports WTM_DEMO_API_PORT=$((ports + 1)) WTM_DEMO_PROXY_PORT=$((ports + 2)) \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  mkdir -p apps/docs
  printf 'DOCS_TITLE=\n' >apps/docs/.env.example
  git add -A && git -c user.email=dev@acme.dev -c user.name=Dev commit -qm "docs app"
  printf 'DOCS_TITLE=Acme\n' >apps/docs/.env
  printf 'JWT_SECRET=s3cret\nMAILER_URL=http://localhost:8025\n' >>apps/api/.env
  printf '\n[[env.file]]\ntarget = "apps/docs/.env"\ntemplate = "apps/docs/.env.example"\n' >>.git/wtm/config.toml
}

# meta <branch> <key> <json value|null>: a key of the meta record of branch, rewritten; null removes it.
meta() {
  python3 - "$@" <<'PY'
import json, sys
path = ".git/wtm/worktrees/%s/meta.json" % sys.argv[1].replace("/", "%2F")
record = json.load(open(path))
value = json.loads(sys.argv[3])
if value is None:
    record.pop(sys.argv[2], None)
else:
    record[sys.argv[2]] = value
json.dump(record, open(path, "w"), indent=2)
PY
}

# prepare <root>: feat/a, feat/b and feat/p with their values from main, feat/c from the templates; feat/a's web PORT
# and API_URL drifted back to main's and an OLD key no source has; feat/b's secret its own and its docs .env deleted;
# feat/p's record without its isolation, as a worktree made before the choice existed.
prepare() {
  root=$1
  sandbox "$root" "$2"
  local name trees=$root/acme.trees
  for name in feat/a feat/b feat/p; do
    "$legacy" create "$name" --env-from main --yes </dev/null >/dev/null 2>&1
  done
  "$legacy" create feat/c --yes </dev/null >/dev/null 2>&1
  cp apps/web/.env "$trees/feat-a/apps/web/.env"
  echo OLD=1 >>"$trees/feat-a/apps/api/.env"
  sed -i '' 's/^JWT_SECRET=.*/JWT_SECRET=mine/' "$trees/feat-b/apps/api/.env"
  rm "$trees/feat-b/apps/docs/.env"
  meta feat/p isolation null
}

scenario() {
  local name=$1
  shift
  local code=0
  (cd "$at" && "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr") || code=$?
  echo "$code" >"$dir/$name.exit"
  state >"$dir/$name.state"
}

# both <name> <legacy args> -- <next args>: the scenario, each side with its own arguments.
both() {
  local name=$1 args=()
  shift
  while [[ $1 != -- ]]; do
    args+=("$1")
    shift
  done
  shift
  if [[ $side == legacy ]]; then
    scenario "$name" "${args[@]}"
  else
    scenario "$name" "$@"
  fi
}

# state: every .env file of every worktree, then each worktree's isolation.
state() {
  local path file
  git worktree list --porcelain | awk '/^worktree /{print $2}' | while read -r path; do
    for file in apps/api/.env apps/web/.env apps/docs/.env; do
      echo "## $path/$file"
      cat "$path/$file" 2>/dev/null || echo "(missing)"
    done
  done
  echo "## isolation"
  for path in .git/wtm/worktrees/*/meta.json; do
    echo "$path: $(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("isolation", "(none)"))' "$path")"
  done
}

run_side() {
  side=$1
  bin=$2
  local box
  box=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$box")
  dir=$out/$side
  rm -rf "$dir" && mkdir -p "$dir"
  (
    prepare "$box" 24600
    at=$box/acme
    state >"$dir/00-before.state"
    both 01-preview env feat/a --check -- env feat/a --dry-run
    both 02-preview-json env feat/a --check --output json -- env feat/a --dry-run --output json
    both 03-apply env feat/a --yes -- env feat/a --yes
    both 04-preview-after-apply env feat/a --check -- env feat/a --dry-run
    both 05-overwrite env feat/b --mode refresh --on-conflict overwrite --yes -- env feat/b --overwrite --yes
    both 06-prune env feat/a --prune --yes -- env feat/a --prune --yes
    at=$box/acme.trees/feat-c
    both 07-no-worktree-no-terminal env -- env
    at=$box/acme
    both 08-pending env feat/p --yes -- env feat/p --yes
    both 09-pending-isolated env feat/p --isolation isolated --yes -- env feat/p --isolation isolated --yes
    both 10-verbatim env feat/c --isolation verbatim --yes -- env feat/c --isolation verbatim --yes
    both 11-main-isolation env main --isolation verbatim --yes -- env main --isolation verbatim --yes
    both 12-unknown env feat/nope --yes -- env feat/nope --yes
    both 13-json env feat/c --yes --output json -- env feat/c --yes --output json
    both 14-show-values env feat/b --check --show-values -- env show feat/b --show-values
    both 15-show-json env feat/b --check --output json -- env show feat/b --output json
    both 16-quiet -q env feat/c --yes -- -q env feat/c --yes
    both 17-all env main --yes -- env --all --yes
    # a port set by hand, which holds no port of its job to move
    sed -i '' 's/^PORT=.*/PORT=9999/' "$box/acme.trees/feat-b/apps/web/.env"
    both 18-hand-set-port env feat/b --yes -- env feat/b --yes
  )
  sed -i '' -E \
    -e "s#(/private)?$box#<SB>#g" \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#"created_at": "[^"]*"#"created_at": "<DATE>"#g' \
    "$dir"/*
}

# tty_side <side> <binary>: the wizard in tmux, in a sandbox of its own, from feat/a: the first screen, each question
# taken at its default up to the recap, then the outcome.
tty_side() {
  local side=$1 bin=$2 box
  box=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$box")
  local tty=$out/tty/$side session=wtmenv-$side-$(basename "$box")
  rm -rf "$tty" && mkdir -p "$tty"
  (
    prepare "$box" 24610
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
      echo "env.sh: the $side screen never settled on \"$1\"" >&2
      return 1
    }
    capture() {
      settle "$2"
      screen >"$tty/$1.txt"
    }
    tmux new-session -d -s "$session" -x 100 -y 40 -c "$box/acme.trees/feat-a" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$box/home PATH=$box/bin:\$PATH PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "wtmx env; echo EXIT=\$?" Enter
    if [[ $side == legacy ]]; then
      capture 1-first 'Select a worktree'
    else
      capture 1-first 'Worktrees'
    fi
    local tries=0
    until [[ $(screen) == *"No, cancel"* ]] || ((tries > 8)); do
      tmux send-keys -t "$session" Enter
      sleep 1
      tries=$((tries + 1))
    done
    capture 2-recap 'No, cancel'
    tmux send-keys -t "$session" Enter
    capture 3-outcome 'EXIT=0'
  )
  sed -i '' -E -e "s#(/private)?$box#<SB>#g" -e 's#wtmx#wtm#g' -e 's#[0-9a-f]{24}#<FP>#g' "$tty"/*
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

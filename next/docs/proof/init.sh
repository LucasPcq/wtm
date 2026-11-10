#!/usr/bin/env bash
# Usage: init.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same init scenarios in two identical sandboxes (the docs/demos "acme" project, its config.toml
# removed, a pnpm lockfile and a docker compose file added, a worktree made elsewhere), one per binary:
# legacy `wtm init` against `wtm-next init`. After each scenario it records the exit code, the output and
# the state (config.toml, the repositories registered), then diffs the states. On the next side, the legacy
# binary then reads the config.toml next wrote. Then, in a fresh sandbox per side, it drives the wizard in tmux:
# a first init, then a re-init, under <out dir>/tty/<side>. Outputs differ by design (next's v2 contract);
# the states may differ only where next/docs/proof/README.md says why.
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

# prepare <root>: the acme project as the demo sets it up, then not set up: its config.toml removed, its
# registration dropped; a pnpm lockfile, a docker compose file, a branch develop and a worktree made by git
# outside any folder wtm knows.
prepare() {
  local root=$1
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=24360 WTM_DEMO_API_PORT=24361 WTM_DEMO_PROXY_PORT=24362 \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home PATH=$root/bin:$PATH
  cd "$root/acme"
  rm -rf .git/wtm/config.toml .git/wtm/config.toml.lock .git/wtm/schemas "$HOME/Library/Application Support/wtm/repos.json"
  printf '{"name": "acme", "private": true}\n' >package.json
  printf 'lockfileVersion: 9\n' >pnpm-lock.yaml
  printf 'services:\n  db:\n    image: postgres\n' >docker-compose.yml
  git add -A && git commit -q -m "tooling"
  git branch develop
  git worktree add -q -b feat/old "$root/elsewhere" main
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

# legacy_loads <name>: the legacy binary reads the config.toml the scenario left, as every one of its commands does.
legacy_loads() {
  local code=0
  "$legacy" config show </dev/null >"$dir/$1.legacy-loads.stdout" 2>&1 || code=$?
  echo "$code" >"$dir/$1.legacy-loads.exit"
}

state() {
  echo "## config.toml"
  cat .git/wtm/config.toml 2>/dev/null || echo "(none)"
  echo "## repos.json"
  grep -o '"root": *"[^"]*"' "$HOME/Library/Application Support/wtm/repos.json" 2>/dev/null || echo "(none)"
}

# drop_web_env: removes the web's .env file from config.toml by hand, as a user who does not want it copied.
drop_web_env() {
  python3 - .git/wtm/config.toml <<'PY'
import sys
path = sys.argv[1]
text = open(path).read()
text = text.replace('[[env.file]]\ntarget = "apps/web/.env"\ntemplate = "apps/web/.env.example"\n', '')
open(path, "w").write(text)
PY
}

# both <name> <legacy args…> -- <next args…>: the scenario name, run with the args of this side.
both() {
  local name=$1 mine=() seen=0
  shift
  for arg in "$@"; do
    if [[ $arg == -- ]]; then
      seen=1
      continue
    fi
    if [[ ($side == legacy && $seen == 0) || ($side == next && $seen == 1) ]]; then
      mine+=("$arg")
    fi
  done
  scenario "$name" init "${mine[@]}"
}

run_side() {
  side=$1
  bin=$2
  local root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  dir=$out/$side
  rm -rf "$dir" && mkdir -p "$dir"
  (
    prepare "$root"
    state >"$dir/00-before.state"
    scenario 01-dry-run init --yes --dry-run
    scenario 02-first init --yes
    [[ $side == next ]] && legacy_loads 02-first
    scenario 03-again init --yes
    scenario 04-base-branch init --base-branch develop --yes
    drop_web_env
    both 05-removed-file --only env --yes -- --yes
    both 06-on-clean --only hooks --yes --clean-command "make down" -- --yes --on-clean "make down"
    scenario 07-json init --yes --output json
    scenario 08-invalid init --yes --env-strategy copy
    rm .git/wtm/config.toml
    both 09-none --yes --skip-env --skip-hooks --skip-clean -- --yes --env-strategy none --on-create= --on-clean=
    [[ $side == next ]] && legacy_loads 09-none
    rm .git/wtm/config.toml
    scenario 10-no-terminal init
    both 11-hook-in-folder --yes --install-command "pnpm install && pnpm db:generate" -- --yes --on-create "pnpm install" --on-create '{"cmd": "pnpm db:generate", "cwd": "apps/api", "continue_on_error": true}'
    [[ $side == next ]] && legacy_loads 11-hook-in-folder
    at=/tmp scenario 12-outside-a-repository init --yes
  )
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    "$dir"/*
}

# tty_side <side> <binary>: the wizard in tmux, in a sandbox of its own: a first init through every step, then
# the web's .env file removed by hand and a re-init (legacy: --only env,hooks), its .env step showing it unpicked.
tty_side() {
  local side=$1 bin=$2 root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  local tty=$out/tty/$side session=wtminit-$side-$(basename "$root")
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
      echo "init.sh: the $side screen never settled on \"$1\"" >&2
      return 1
    }
    capture() {
      settle "$2"
      screen >"$tty/$1.txt"
    }
    keys() {
      for key in "$@"; do
        tmux send-keys -t "$session" "$key"
        sleep 0.4
      done
    }
    tmux new-session -d -s "$session" -x 100 -y 40 -c "$root/acme" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "wtmx init; echo EXIT=\$?" Enter
    if [[ $side == legacy ]]; then
      capture 1-base-path 'Step 1/'
      keys Enter Enter Enter Enter
      capture 2-env-files 'Env files to copy'
      keys Enter Enter
      capture 3-on-create 'Add a command'
      keys Down Down Enter Enter
      capture 4-on-clean 'Pre-clean hooks'
      keys Down Enter
      capture 5-outcome 'EXIT='
    else
      capture 1-base-path 'Step 1/'
      keys Enter Enter Enter
      capture 2-env-files '.env files'
      keys Enter
      capture 3-on-create 'Add a command'
      keys Down Down Enter
      capture 4-on-clean 'On clean'
      keys Down Down Enter
      capture 5-recap 'No, cancel'
      keys Enter
      capture 6-outcome 'EXIT='
    fi
    drop_web_env
    tmux send-keys -t "$session" "clear" Enter
    if [[ $side == legacy ]]; then
      tmux send-keys -t "$session" "wtmx init --only env,hooks; echo EXIT=\$?" Enter
      capture 7-reinit-strategy 'Step 1/'
      keys Enter
      capture 8-reinit-env-files 'Env files to copy'
    else
      tmux send-keys -t "$session" "wtmx init; echo EXIT=\$?" Enter
      capture 7-reinit-base-path 'Step 1/'
      keys Enter Enter Enter
      capture 8-reinit-env-files 'Step 4/'
    fi
    keys C-c
    settle 'EXIT='
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

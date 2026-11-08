#!/usr/bin/env bash
# Usage: run-logs.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same run logs scenarios in two identical sandboxes (the docs/demos "acme"
# project with a task, a service that crashes 2 seconds after it starts and a job
# never started), one per binary, one after the other: legacy `wtm run logs` against
# `wtm-next run logs`. Each sandbox has its own HOME, so its own daemon, socket
# and index; its jobs and its daemon are stopped before the next side starts. Each
# side starts the profile in feat/b with its own binary, then reads it. After each
# scenario it records the exit code, the output and the state (the daemon's index of
# the jobs up, the jobs' logs, whether a daemon runs). Legacy's run logs follows
# without a terminal and ignores SIGINT and SIGTERM (B1): its follows are stopped
# with SIGKILL, on the process the script started. Then, in tmux, the picker and the
# live view, and a worktree where nothing ran. Outputs differ by design (next's v2
# contract); the states may differ only where next/docs/proof/README.md says why.
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

# prepare <root> <web port>: the acme sandbox with a task, a crashing service and a job never started; ports from <web port> up.
prepare() {
  local root=$1 port=$2
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$port WTM_DEMO_API_PORT=$((port + 1)) WTM_DEMO_PROXY_PORT=$((port + 2)) \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  cat >>.git/wtm/run.toml <<TOML

[[job]]
  name = "migrate"
  kind = "task"
  cmd = "echo migrating; sleep 1; echo migrated"

[[job]]
  name = "crashy"
  kind = "service"
  cmd = "echo starting; sleep 2; echo boom >&2; exit 1"
  binds_no_port = true

[[job]]
  name = "never"
  kind = "service"
  cmd = "sleep 100"
  binds_no_port = true

[[profile]]
  name = "logs"
  jobs = ["migrate", "api", "web", "crashy"]
TOML
  local branch
  for branch in feat/b feat/c; do
    "$legacy" create "$branch" --from main --yes </dev/null >/dev/null 2>&1
  done
}

# teardown: every job of the sandbox stopped, then its daemon.
teardown() {
  "$legacy" run down --all --yes </dev/null >/dev/null 2>&1 || true
  "$next" daemon stop --yes </dev/null >/dev/null 2>&1 || true
}

# scenario <name> <seconds> <args...>: runs the side's binary with no terminal; one still running after seconds gets SIGINT, then SIGKILL a second later.
scenario() {
  local name=$1 seconds=$2
  shift 2
  local code=0 pid killer
  "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr" &
  pid=$!
  (sleep "$seconds"; kill -INT "$pid" 2>/dev/null && sleep 1 && kill -KILL "$pid" 2>/dev/null) &
  killer=$!
  wait "$pid" || code=$?
  kill "$killer" 2>/dev/null || true
  wait "$killer" 2>/dev/null || true
  echo "$code" >"$dir/$name.exit"
  state >"$dir/$name.state"
}

state() {
  echo "## jobs up (the daemon's index)"
  python3 - "$HOME/Library/Application Support/wtm/jobs.json" <<'PY' || true
import json, os, sys
try:
    jobs = json.load(open(sys.argv[1]))["jobs"]
except (OSError, ValueError, KeyError):
    jobs = []
for job in sorted(jobs, key=lambda j: (j["work_dir"], j["name"])):
    print(os.path.basename(job["work_dir"]), job["name"])
PY
  echo "## logs"
  (cd .git/wtm/logs 2>/dev/null && find . -type f | sort) || true
  echo "## a daemon answers"
  python3 - "$HOME/Library/Application Support/wtm/wtm.sock" <<'PY'
import socket, sys
s = socket.socket(socket.AF_UNIX)
try:
    s.connect(sys.argv[1])
    print("yes")
except OSError:
    print("no")
PY
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
    prepare "$root" 24450
    trap teardown EXIT
    "$bin" run up feat/b --profile logs -d --yes </dev/null >/dev/null 2>&1
    # crashy crashes 2 seconds after it starts
    sleep 4
    state >"$dir/00-before.state"
    scenario 01-lines 3 run logs feat/b --job migrate
    scenario 02-crashed 3 run logs feat/b --job crashy
    scenario 03-never-ran 3 run logs feat/b --job never
    scenario 04-undeclared 3 run logs feat/b --job ghost
    scenario 05-nothing-ran 3 run logs feat/c
    scenario 06-several 3 run logs feat/b feat/c --job migrate
    scenario 07-json 3 run logs feat/b --job crashy --output json
    scenario 08-json-refused 3 run logs nope --output json
    scenario 09-quiet 3 run logs feat/b --job crashy -q
    # next follows only with --follow (legacy has no such flag and follows always)
    if [[ $side == legacy ]]; then
      scenario 10-follow 3 run logs feat/b --job api
    else
      scenario 10-follow 3 run logs feat/b --job api --follow
    fi
    scenario 11-all 3 run logs feat/b
    # next only: a search reads each job's whole log (legacy has no --search)
    if [[ $side == next ]]; then
      scenario 13-search 3 run logs feat/b --search 'boom|migrated' --regex
    fi
    "$next" daemon stop --yes </dev/null >/dev/null 2>&1 || true
    sleep 1
    state >"$dir/12-daemon-stopped.state"
    scenario 12-daemon-down 3 run logs feat/b --job crashy
  )
  local escaped=${root//\//%2F}
  # The jobs' own lines (the demo's fake dev servers) depend on timing: their clock times are zeroed and only the first lines of a follow are kept.
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e "s#(%2Fprivate)?$escaped#<SB>#g" \
    -e 's#[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z#<TIME>#g' \
    -e 's#[0-9]{2}:[0-9]{2}:[0-9]{2} #00:00:00 #g' \
    -e 's#wtm-next run#wtm run#g' \
    "$dir"/*
  local followed
  for followed in "$dir/10-follow.stdout" "$dir/11-all.stdout"; do
    head -4 "$followed" >"$followed.head" && mv "$followed.head" "$followed"
  done
}

# tty_side <side> <binary>: in tmux at 120×36, run logs with no worktree from main (the picker), feat/b picked and the view opened, left with q; then feat/c, where nothing ran.
tty_side() {
  local side=$1 bin=$2 root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  local tty=$out/tty/$side session=wtmrunlogs-$side-$(basename "$root")
  rm -rf "$tty" && mkdir -p "$tty"
  (
    prepare "$root" 24470
    trap 'tmux kill-session -t "$session" 2>/dev/null; teardown' EXIT
    "$bin" run up feat/b --profile logs -d --yes </dev/null >/dev/null 2>&1
    sleep 4
    cp "$bin" "$root/bin/wtmx"
    screen() { tmux capture-pane -p -t "$session" | cat -s; }
    settle() {
      local now last="" tries=0
      while ((tries < 100)); do
        now=$(screen)
        [[ $now == *"$1"* && $now == "$last" ]] && return
        last=$now tries=$((tries + 1))
        sleep 0.3
      done
      echo "run-logs.sh: the $side screen never settled on \"$1\":" >&2
      screen >&2
      return 1
    }
    capture() {
      settle "$2"
      screen >"$tty/$1.txt"
    }
    tmux new-session -d -s "$session" -x 120 -y 36 -c "$root/acme" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "wtmx run logs; echo EXIT=\$?" Enter
    capture 1-picker 'feat/b'
    # main is picked (the worktree run from): unpick it, pick feat/b
    tmux send-keys -t "$session" Space
    sleep 0.3
    tmux send-keys -t "$session" Down
    sleep 0.3
    tmux send-keys -t "$session" Space
    sleep 0.3
    tmux send-keys -t "$session" Enter
    capture 2-view 'crashy'
    tmux send-keys -t "$session" q
    capture 3-left 'EXIT='
    tmux send-keys -t "$session" "clear; wtmx run logs feat/c; echo EXIT=\$?" Enter
    sleep 3
    screen >"$tty/4-nothing-ran.txt"
    tmux send-keys -t "$session" q
    settle 'EXIT='
  )
  sed -i '' -E -e "s#(/private)?$root#<SB>#g" -e 's#wtmx#wtm#g' -e 's#[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z#<TIME>#g' \
    -e 's#[0-9]{2}:[0-9]{2}:[0-9]{2}#00:00:00#g' -e 's#[0-9]+([smh]) │#0\1 │#g' "$tty"/*
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

#!/usr/bin/env bash
# Usage: run-view.sh <wtm binary> <wtm-next binary> <out dir>
# The live view `run up` opens at a terminal, legacy against next, each in a
# sandbox of its own (the docs/demos "acme" project with a task, a shared
# service with a namespace and a service that crashes 8 seconds after it
# starts), driven in tmux at 120×40 with the same keys: the start, the crash,
# the shared service and focus on it, the filter and focus on a job, the
# addresses, then q and what is printed under the view. Each sandbox has its
# own HOME, so its own run daemon; its jobs and its daemon are stopped before it
# is removed.
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

# prepare <root> <web port>: the acme sandbox with a task, a shared db and a service that crashes late; ports from <web port> up.
prepare() {
  local root=$1 port=$2
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$port WTM_DEMO_API_PORT=$((port + 1)) WTM_DEMO_PROXY_PORT=$((port + 2)) \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  cat >scripts/db.py <<'PY'
#!/usr/bin/env python3
import http.server, os, time, threading
port = int(os.environ["DB_PORT"])
s = http.server.ThreadingHTTPServer(("", port), http.server.SimpleHTTPRequestHandler)
threading.Thread(target=s.serve_forever, daemon=True).start()
print(f"db listening on {port}", flush=True)
while True:
    time.sleep(60); print("db: checkpoint", flush=True)
PY
  git add -A
  git -c core.hooksPath=/dev/null commit -qm "chore: db"
  cat >>.git/wtm/run.toml <<TOML

[[job]]
  name = "migrate"
  kind = "task"
  cmd = "echo migrating; sleep 1; echo migrated"

[[job]]
  name = "db"
  kind = "service"
  cmd = "python3 scripts/db.py"
  scope = "shared"
  [job.ports]
    DB_PORT = $((port + 3))
  [job.namespace]
    name = "acme_{worktree}"
    create = "true"

[[job]]
  name = "flaky"
  kind = "service"
  cmd = "echo flaky up; sleep 8; echo flaky gave up >&2; exit 1"
  binds_no_port = true

[[profile]]
  name = "view"
  jobs = ["migrate", "db", "api", "web", "flaky"]
TOML
  "$legacy" create feat/b --from main --yes </dev/null >/dev/null 2>&1
}

# teardown: every job of the sandbox stopped, then its daemon.
teardown() {
  "$legacy" run down --all --yes </dev/null >/dev/null 2>&1 || true
  "$legacy" run daemon stop --yes </dev/null >/dev/null 2>&1 || true
}

# side <side> <binary>: the view in tmux, in a sandbox of its own.
side() {
  local side=$1 bin=$2 root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  local dir=$out/$side session=wtmrunview-$side-$(basename "$root")
  rm -rf "$dir" && mkdir -p "$dir"
  (
    prepare "$root" 24390
    trap 'tmux kill-session -t "$session" 2>/dev/null; teardown' EXIT
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
      echo "run-view.sh: the $side screen never settled on \"$1\":" >&2
      screen >&2
      return 1
    }
    capture() {
      settle "$2"
      screen >"$dir/$1.txt"
    }
    # keys sends each key on its own, 0.3 s apart: two escapes in one write read as alt+esc.
    keys() {
      local key
      for key in "$@"; do
        tmux send-keys -t "$session" "$key"
        sleep 0.3
      done
    }
    tmux new-session -d -s "$session" -x 120 -y 40 -c "$root/acme" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "wtmx run up feat/b --profile view --yes; echo EXIT=\$?" Enter
    capture 1-started 'JOBS'
    sleep 9
    capture 2-crashed '✗'
    keys k k k k k k
    capture 3-shared 'db'
    keys Enter
    capture 4-focus-shared 'db'
    keys Escape Escape
    keys / a p i Enter
    capture 5-filter 'api'
    keys Enter
    capture 6-focus-job 'api'
    keys Escape Escape
    keys Escape
    keys a
    capture 7-addresses 'http'
    keys q
    capture 8-left 'EXIT='
  )
  # Uptimes and clock times as zeros, the columns kept.
  sed -i '' -E -e "s#(/private)?$root#<SB>#g" -e 's#wtmx#wtm#g' -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#[0-9]([smh]) │#0\1 │#g' -e 's#[0-9]0([smh]) │#00\1 │#g' -e 's#[0-9]{2}:[0-9]{2}:[0-9]{2}#00:00:00#g' "$dir"/*
}

mkdir -p "$out"
roots=()
trap 'rm -rf ${roots[@]+"${roots[@]}"}' EXIT
side legacy "$legacy"
side next "$next"

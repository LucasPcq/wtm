#!/usr/bin/env bash
# Usage: run-ps-list.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same run ps and run list scenarios in two identical sandboxes (the
# docs/demos "acme" project with a migrate task, a shared db with a namespace, a
# docker compose stack running cache, and a service that crashes at once), one per
# binary, one after the other: legacy `wtm run ps` / `wtm run list` against
# `wtm-next run ps` / `wtm-next run list`. Each sandbox has its own HOME, so its own
# daemon, socket and index; its jobs, its daemon and its compose containers are gone
# before the next side starts. Each side starts, with its own binary, profile full
# in feat/a (migrate exits, db joined, the stack, api, web) and profile crash in
# feat/b (db joined, api, crashy crashes), so db has two holders. After each
# scenario it records the exit code, the output and the state; reads change nothing,
# so the states are the same before and after. Then, in tmux, run list's picker.
# Outputs differ by design (next's v2 contract, next/docs/proof/README.md).
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
compose_plugin=$(docker info --format '{{range .ClientInfo.Plugins}}{{if eq .Name "compose"}}{{.Path}}{{end}}{{end}}' 2>/dev/null || true)

# prepare <root> <web port>: the acme sandbox, widened; ports from <web port> up.
prepare() {
  local root=$1 port=$2
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$port WTM_DEMO_API_PORT=$((port + 1)) WTM_DEMO_PROXY_PORT=$((port + 2)) \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  if [[ -n $compose_plugin ]]; then
    mkdir -p "$HOME/.docker/cli-plugins"
    ln -sf "$compose_plugin" "$HOME/.docker/cli-plugins/docker-compose"
  fi
  cd "$root/acme"
  cat >scripts/db.py <<'PY'
#!/usr/bin/env python3
import http.server, os, time, threading
port = int(os.environ["DB_PORT"])
s = http.server.ThreadingHTTPServer(("", port), http.server.SimpleHTTPRequestHandler)
threading.Thread(target=s.serve_forever, daemon=True).start()
print(f"db listening on {port}", flush=True)
while True:
    time.sleep(5); print("db: checkpoint", flush=True)
PY
  printf 'services:\n  cache:\n    image: redis:7-alpine\n    ports:\n      - "${CACHE_PORT:-6379}:6379"\n' >docker-compose.yml
  git add -A
  git -c core.hooksPath=/dev/null commit -qm "chore: db and compose"
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
    create = "echo \"\$WTM_NAMESPACE\" >> \"\$HOME/namespaces.txt\""

[[job]]
  name = "cache"
  kind = "service"
  cmd = "true"
  [job.ports]
    CACHE_PORT = $((port + 4))

[[job]]
  name = "stack"
  kind = "service"
  cmd = "docker compose up -d cache"
  stop = "docker compose down"
  runs = ["cache"]

[[job]]
  name = "crashy"
  kind = "service"
  cmd = "echo starting; sleep 0.2; echo boom >&2; exit 1"
  binds_no_port = true

[[profile]]
  name = "full"
  jobs = ["migrate", "db", "stack", "api", "web"]

[[profile]]
  name = "crash"
  jobs = ["db", "api", "crashy"]
TOML
  local branch
  for branch in feat/a feat/b; do
    "$legacy" create "$branch" --from main --yes </dev/null >/dev/null 2>&1
  done
}

# start: the side's own binary starts both profiles, unattended, the others' jobs kept.
start() {
  "$bin" run up feat/a --profile full --yes --parallel -d </dev/null >/dev/null 2>&1 || true
  "$bin" run up feat/b --profile crash --yes --parallel -d </dev/null >/dev/null 2>&1 || true
  sleep 2
}

# teardown: every job of the sandbox stopped (its stack's stop command run), then its daemon.
teardown() {
  "$legacy" run down --all --yes </dev/null >/dev/null 2>&1 || true
  "$next" run down --all --yes </dev/null >/dev/null 2>&1 || true
  "$legacy" run daemon stop --yes </dev/null >/dev/null 2>&1 || true
}

# scenario <name> <dir> <args...>: runs the side's binary from dir with no terminal.
scenario() {
  local name=$1 from=$2
  shift 2
  local code=0
  (cd "$from" && "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr") || code=$?
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
    hold = "claim" if job.get("joined") else ("stack" if job["config"].get("stop") else "own")
    print(os.path.basename(job["work_dir"]), job["name"], hold)
PY
  echo "## compose containers"
  docker ps --format '{{.Names}}' 2>/dev/null | grep '^acme-' | sort || true
}

# normalize <root> <files...>: the sandbox's paths, the times, the ages, the pids, and the demo servers' own lines, which depend on timing, made stable.
normalize() {
  local root=$1
  shift
  local escaped=${root//\//%2F}
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e "s#(%2Fprivate)?$escaped#<SB>#g" \
    -e 's#[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+(Z|[+-][0-9]{2}:[0-9]{2})#<TIME>#g' \
    -e 's#[0-9]{2}:[0-9]{2}:[0-9]{2}#00:00:00#g' \
    -e 's#"pid": [0-9]+#"pid": <PID>#g' \
    -e 's#"duration_ms": [0-9]+#"duration_ms": <MS>#g' \
    -e 's#[0-9]+(s|m|h|d) ago#0\1 ago#g' \
    -e 's#  [0-9]+(s|m|h|d)  #  0\1  #g' \
    -e 's#(up|since) [0-9]+(s|m|h|d)#\1 0\2#g' \
    -e 's#[0-9]+ms#0ms#g' \
    -e 's#00:00:00 [A-Za-z][^"]*#00:00:00 <a request>#g' \
    -e 's#(\[wtm\] namespace [^ ]* ready for [^ "]*|db listening on [0-9]+|db: checkpoint)#<db line>#g' \
    -e 's#wtm-next run#wtm run#g' -e 's#wtmx#wtm#g' \
    "$@"
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
    prepare "$root" 24850
    trap teardown EXIT
    start
    state >"$dir/00-before.state"
    scenario 01-ps "$root/acme" run ps
    scenario 02-ps-json "$root/acme" run ps --output json
    scenario 03-ps-outside "$root" run ps
    scenario 04-ps-quiet "$root/acme" run ps -q
    if [[ $side == next ]]; then
      scenario 05-ps-repo "$root" run ps --repo acme.trees/feat-a
      scenario 06-ps-repo-nowhere "$root" run ps --repo nowhere
    fi
    scenario 07-list "$root/acme.trees/feat-a" run list
    scenario 08-list-yes "$root/acme.trees/feat-a" run list --yes
    scenario 09-list-json "$root/acme.trees/feat-a" run list --output json
    scenario 10-list-outside "$root" run list
    if [[ $side == next ]]; then
      scenario 11-list-named "$root/acme" run list feat/b
    fi
  )
  normalize "$root" "$dir"/*
}

# tty_side <side> <binary>: in tmux at 100×30, run list from feat/a: the entries, a job picked and its actions, the one picked; legacy runs the action it picks, so its walk stops at the actions and leaves with esc.
tty_side() {
  local side=$1 bin=$2 root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  local tty=$out/tty/$side session=wtmrunlist-$side-$(basename "$root")
  rm -rf "$tty" && mkdir -p "$tty"
  (
    prepare "$root" 24870
    trap 'tmux kill-session -t "$session" 2>/dev/null; teardown' EXIT
    start
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
      echo "run-ps-list.sh: the $side screen never settled on \"$1\":" >&2
      screen >&2
      return 1
    }
    capture() {
      settle "$2"
      screen >"$tty/$1.txt"
    }
    tmux new-session -d -s "$session" -x 100 -y 30 -c "$root/acme.trees/feat-a" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "wtmx run list; echo EXIT=\$?" Enter
    if [[ $side == next ]]; then
      # next asks the worktree first, the one stood in picked
      capture 0-worktree 'feat/a'
      tmux send-keys -t "$session" Enter
    fi
    capture 1-entries 'crashy'
    # the profiles come first, then a rule, then the jobs: down to api
    local key
    for key in Down Down Down Down; do
      tmux send-keys -t "$session" "$key"
      sleep 0.2
    done
    tmux send-keys -t "$session" Enter
    capture 2-actions 'Action'
    if [[ $side == next ]]; then
      tmux send-keys -t "$session" Down Down Enter
      capture 3-picked 'EXIT='
    else
      tmux send-keys -t "$session" Escape
      sleep 0.3
      tmux send-keys -t "$session" Escape
      capture 3-left 'EXIT='
    fi
  )
  normalize "$root" "$tty"/*
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

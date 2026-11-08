#!/usr/bin/env bash
# Usage: daemon-commands.sh <wtm binary> <wtm-next binary> <out dir>
# Runs legacy `wtm run daemon status|stop|restart` against `wtm-next daemon
# status|stop|restart|stop-jobs` in two identical sandboxes, one after the other:
# the "acme" project of docs/demos with a shared service db, a stack (a sleep its
# stop command kills, no docker) and feat/a holding db, each side starting its
# jobs with its own `run up`. Each sandbox has its own HOME, so its own daemon,
# socket and index; its jobs and its daemon are stopped before the next side
# starts. Each scenario records its exit code, its output and the daemon's index
# (jobs.json, summarised). Then, in a third sandbox, the cutover: a legacy
# daemon holding jobs, taken over by `wtm-next daemon restart`. Then each
# side's confirmation in tmux. Paths, pids and builds are normalised.
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

# prepare <root> <port>: the acme sandbox, a shared db and a stack in its run.toml, feat/a created.
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
    time.sleep(5); print("db: checkpoint", flush=True)
PY
  git add -A
  git -c core.hooksPath=/dev/null commit -qm "chore: db"
  cat >>.git/wtm/run.toml <<TOML

[[job]]
  name = "db"
  kind = "service"
  cmd = "python3 scripts/db.py"
  scope = "shared"
  [job.ports]
    DB_PORT = $((port + 3))

[[job]]
  name = "stack"
  kind = "service"
  cmd = "nohup sleep 600 >/dev/null 2>&1 & echo \$! > \"\$HOME/stack-\$(basename \$PWD).pid\"; echo stack up"
  stop = "kill \$(cat \"\$HOME/stack-\$(basename \$PWD).pid\") && echo stack down"

[[profile]]
  name = "all"
  jobs = ["api", "web", "db", "stack"]
TOML
  "$legacy" create feat/a --from main --yes </dev/null >/dev/null 2>&1
}

# up <binary>: profile all started in main, then in feat/a, which holds main's db.
up() {
  (cd "$root/acme" && "$1" run up --profile all --yes </dev/null >/dev/null 2>&1) || true
  (cd "$root/acme.trees/feat-a" && "$1" run up --profile all --yes </dev/null >/dev/null 2>&1) || true
}

# teardown: every job of the sandbox stopped (its stacks' stop commands run), then its daemon.
teardown() {
  (cd "$root/acme" && "$legacy" run down --all --yes </dev/null >/dev/null 2>&1) || true
  (cd "$root/acme" && "$next" run down --all --yes </dev/null >/dev/null 2>&1) || true
  "$next" daemon stop --yes </dev/null >/dev/null 2>&1 || true
}

normalise() {
  sed -E -e "s#/private$root#<root>#g" -e "s#$root#<root>#g" \
    -e 's/pid [0-9]+/pid <pid>/g' -e 's/"pid": [0-9]+/"pid": <pid>/g' \
    -e 's/v0\.[0-9]+\.[0-9]+-[0-9]+-g[0-9a-f]+(-dirty)?/<build>/g' \
    -e 's/[0-9a-f]{24}/<fingerprint>/g' -e 's/"op": "[0-9a-f]+"/"op": "<id>"/g'
}

# scenario <name> <args…>: runs the side's binary from main, unattended, and records its exit, its output and the index.
scenario() {
  local name=$1
  shift
  local code=0
  (cd "$root/acme" && "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr") || code=$?
  echo "$code" >"$dir/$name.exit"
  normalise <"$dir/$name.stdout" >"$dir/$name.tmp" && mv "$dir/$name.tmp" "$dir/$name.stdout"
  normalise <"$dir/$name.stderr" >"$dir/$name.tmp" && mv "$dir/$name.tmp" "$dir/$name.stderr"
  index >"$dir/$name.state"
}

# index: the daemon's index, one job per line (its worktree, its name, what holds it, whether it records a pid), and whether a daemon answers.
index() {
  python3 - "$HOME/Library/Application Support/wtm/jobs.json" "$root" <<'PY'
import json, os, sys
path, root = sys.argv[1], os.path.realpath(sys.argv[2])
try:
    doc = json.load(open(path))
except (OSError, ValueError):
    print("(no index)"); sys.exit()
print(f"version {doc.get('version')}")
for job in sorted(doc.get("jobs", []), key=lambda j: (j.get("work_dir", ""), j.get("name", ""))):
    held = "claim" if job.get("joined") else ("stack" if (job.get("config") or {}).get("stop") else "own")
    print(f"{os.path.relpath(job.get('work_dir', ''), root):24} {job.get('name', ''):6} {held:6} {'pid' if job.get('pid') else '-'}")
PY
  [[ -S "$HOME/Library/Application Support/wtm/wtm.sock" ]] && echo "socket: yes" || echo "socket: no"
}

# snapshot keeps the index as it stands, for pids to compare with.
snapshot() {
  cp "$HOME/Library/Application Support/wtm/jobs.json" "$root/before.json" 2>/dev/null || echo '{"jobs": []}' >"$root/before.json"
}

# pids: each job of the index the snapshot kept, and what became of it: back under a new process, the same process (a stack taken over), a claim kept, or gone.
pids() {
  python3 - "$root/before.json" "$HOME/Library/Application Support/wtm/jobs.json" "$root" <<'PY'
import json, os, sys
root = os.path.realpath(sys.argv[3])
def jobs(path):
    try:
        return {(j["work_dir"], j["name"]): j for j in json.load(open(path)).get("jobs", [])}
    except (OSError, ValueError):
        return {}
before, after = jobs(sys.argv[1]), jobs(sys.argv[2])
for key in sorted(before):
    was, now = before[key], after.get(key)
    if now is None:
        fate = "gone"
    elif was.get("joined"):
        fate = "claim kept"
    elif was.get("pid") == now.get("pid"):
        fate = "same process"
    else:
        fate = "new process"
    print(f"{os.path.relpath(key[0], root):24} {key[1]:6} {fate}")
PY
}

# side <legacy|next>: the scenarios of one binary, in a sandbox of its own.
side() {
  local side=$1 port=$2
  if [[ $side == legacy ]]; then bin=$legacy; daemon=(run daemon); else bin=$next; daemon=(daemon); fi
  root=$(mktemp -d /tmp/wtmXXXXX)
  dir="$out/$side"
  mkdir -p "$dir"
  prepare "$root" "$port"

  scenario 01-status-none "${daemon[@]}" status
  scenario 02-status-none-json "${daemon[@]}" status --output json
  scenario 03-stop-none "${daemon[@]}" stop --yes
  up "$bin"
  scenario 04-status "${daemon[@]}" status
  scenario 05-status-json "${daemon[@]}" status --output json
  [[ $side == next ]] && scenario 06-restart-dry-run "${daemon[@]}" restart --dry-run
  scenario 07-restart-unattended-json "${daemon[@]}" restart --output json
  scenario 08-status-after-restart "${daemon[@]}" status
  up "$bin"
  snapshot
  scenario 09-restart "${daemon[@]}" restart --yes
  pids >"$dir/09-restart.pids"
  scenario 10-status-after-restart "${daemon[@]}" status
  [[ $side == next ]] && scenario 11-stop-dry-run "${daemon[@]}" stop --dry-run
  scenario 12-stop "${daemon[@]}" stop --yes
  scenario 13-status-stopped "${daemon[@]}" status
  if [[ $side == next ]]; then
    up "$bin"
    scenario 14-stop-jobs-dry-run daemon stop-jobs --dry-run
    scenario 15-stop-jobs-json daemon stop-jobs --repo . --yes --output json
    scenario 16-status-after-stop-jobs daemon status
  fi
  # A newer wtm's index: the daemon records nothing, and says so.
  teardown
  python3 -c 'import json,sys; json.dump({"version": 99, "jobs": []}, open(sys.argv[1], "w"))' "$HOME/Library/Application Support/wtm/jobs.json"
  scenario 17-status-index-frozen "${daemon[@]}" status
  [[ $side == next ]] && scenario 18-tree-index-frozen tree
  rm -f "$HOME/Library/Application Support/wtm/jobs.json"
  teardown
  rm -rf "$root"
}

side legacy 27400
side next 27410

# The cutover: a legacy daemon holding jobs, a next start refused, then next's restart takes them over.
root=$(mktemp -d /tmp/wtmXXXXX)
dir="$out/cutover"
mkdir -p "$dir"
prepare "$root" 27420
up "$legacy"
bin=$next
scenario 01-legacy-holds-jobs daemon status
scenario 02-next-start-refused run up --profile all --yes
snapshot
scenario 03-restart daemon restart --yes
pids >"$dir/03-restart.pids"
scenario 04-status-after daemon status
teardown
rm -rf "$root"

# The confirmation, in tmux: each side's restart asks before it cuts a service.
for side in legacy next; do
  root=$(mktemp -d /tmp/wtmXXXXX)
  prepare "$root" 27430
  if [[ $side == legacy ]]; then bin=$legacy; command="$legacy run daemon restart"; else bin=$next; command="$next daemon restart"; fi
  up "$bin"
  session=wtm-daemon-proof-$side
  tmux kill-session -t "$session" 2>/dev/null || true
  tmux new-session -d -s "$session" -x 110 -y 32 "export HOME='$HOME'; cd '$root/acme'; $command; echo EXIT=\$?; sleep 30"
  sleep 2
  tmux capture-pane -p -t "$session" | normalise >"$out/$side/tty-1-confirm.txt"
  tmux send-keys -t "$session" Escape
  sleep 1
  tmux capture-pane -p -t "$session" | normalise >"$out/$side/tty-2-declined.txt"
  tmux kill-session -t "$session" 2>/dev/null || true
  teardown
  rm -rf "$root"
done

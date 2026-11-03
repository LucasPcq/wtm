#!/usr/bin/env bash
# Usage: clean-data.sh <wtm binary> <wtm-next binary> <out dir>
# What clean and prune do with the run module, in two identical sandboxes (the
# docs/demos "acme" project with a shared service, db, whose namespace per
# worktree is a line of $HOME/namespaces.txt), one per binary, one after the
# other: legacy `wtm` against `wtm-next`. Each sandbox has its own HOME, so its
# own run daemon, socket and index; its jobs are stopped and its daemon gone
# before the next side starts. No Docker. After each scenario it records the exit
# code, the output and the state (the daemon's index, every meta.json, the jobs'
# logs, the namespaces made, pending-removals.toml), then diffs the states. Then,
# in a fresh sandbox per side, the Data step of the wizard in tmux.
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

# prepare <root> <web port>: the acme sandbox with db shared, and the worktrees feat/a to feat/d and feat/f; ports from <web port> up.
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
  [job.namespace]
    name = "acme_{worktree}"
    create = "echo \"\$WTM_NAMESPACE\" >> \"\$HOME/namespaces.txt\""
    remove = "echo \"dropped \$WTM_NAMESPACE on :\$DB_PORT\"; grep -vx \"\$WTM_NAMESPACE\" \"\$HOME/namespaces.txt\" > \"\$HOME/ns.tmp\"; mv \"\$HOME/ns.tmp\" \"\$HOME/namespaces.txt\""

[[profile]]
  name = "data"
  jobs = ["db", "api", "web"]

[[profile]]
  name = "shared"
  jobs = ["db"]
TOML
  local branch
  for branch in feat/a feat/b feat/c feat/d feat/f; do
    "$legacy" create "$branch" --from main --yes </dev/null >/dev/null 2>&1
  done
}

# teardown: every job of the sandbox stopped, then its daemon.
teardown() {
  "$legacy" run down --all --yes </dev/null >/dev/null 2>&1 || true
  "$legacy" run daemon stop --yes </dev/null >/dev/null 2>&1 || true
}

# quietly runs a step that sets a scenario up, with the side's binary unless told otherwise.
quietly() {
  "$@" </dev/null >/dev/null 2>&1 || true
  sleep 1
}

scenario() {
  local name=$1
  shift
  local code=0
  "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr" || code=$?
  echo "$code" >"$dir/$name.exit"
  sleep 1
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
  for meta in $(find .git/wtm/worktrees -name meta.json | sort); do
    echo "## $meta"
    grep -E '"(ordinal|namespaces)"|^    "' "$meta" || true
  done
  echo "## logs"
  (cd .git/wtm/logs 2>/dev/null && find . -type d -mindepth 1 | sort) || true
  echo "## namespaces made"
  sort "$HOME/namespaces.txt" 2>/dev/null || true
  echo "## pending-removals.toml"
  cat .git/wtm/pending-removals.toml 2>/dev/null || echo "(none)"
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
    prepare "$root" 24390
    trap teardown EXIT
    quietly "$bin" run up feat/a --profile data -d --yes
    quietly "$bin" run up feat/b --profile data -d --yes
    state >"$dir/00-before.state"
    # its own jobs stopped, its namespace dropped from db, which feat/b keeps up
    scenario 01-clean-stops-and-drops clean feat/a --yes
    # the namespace kept; feat/b's claim was db's last: db stops
    scenario 02-keep-data clean feat/b --keep-data --yes
    # db down: the namespace owed
    quietly "$bin" run up feat/c --profile data -d --yes
    quietly "$legacy" run down feat/c --yes
    scenario 03-service-down-defers clean feat/c --yes
    # db started again: the start pays what feat/c owes
    scenario 04-run-up-pays run up feat/d --profile data -d --yes
    # db down again: --drop-data starts it, drops, stops it again
    quietly "$legacy" run down feat/d --yes
    scenario 05-drop-data clean feat/d --drop-data --yes
    # db up in main, then a debt the legacy wtm wrote, of a worktree long gone: prune pays it first (a start would pay it too)
    quietly "$bin" run up main --profile shared -d --yes
    echo acme_feat-old >>"$HOME/namespaces.txt"
    printf '[[pending]]\n  job = "db"\n  worktree = "feat-old"\n  ordinal = 9\n' >.git/wtm/pending-removals.toml
    scenario 06-prune-pays prune --yes
    # next only: the plan says the jobs it stops and what becomes of the data
    quietly "$bin" run up feat/f --profile data -d --yes
    scenario 07-dry-run clean feat/f --dry-run --output json
  )
  local escaped=${root//\//%2F}
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e "s#(%2Fprivate)?$escaped#<SB>#g" \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#wtm-next #wtm #g' \
    "$dir"/*
}

# tty_side <side> <binary>: clean's wizard in tmux while db, which holds feat/a's data, is down: its Data step.
tty_side() {
  local side=$1 bin=$2 root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  local tty=$out/tty/$side session=wtmcleandata-$side-$(basename "$root")
  rm -rf "$tty" && mkdir -p "$tty"
  (
    prepare "$root" 24410
    trap 'tmux kill-session -t "$session" 2>/dev/null; teardown' EXIT
    quietly "$bin" run up feat/a --profile data -d --yes
    quietly "$legacy" run down feat/a --yes
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
      echo "clean-data.sh: the $side screen never settled on \"$1\"" >&2
      return 1
    }
    capture() {
      settle "$2"
      screen >"$tty/$1.txt"
    }
    tmux new-session -d -s "$session" -x 100 -y 40 -c "$root/acme" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "wtmx clean feat/a; echo EXIT=\$?" Enter
    capture 1-data 'db'
    tmux send-keys -t "$session" C-c
    capture 2-left 'EXIT='
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

#!/usr/bin/env bash
# Usage: run-down.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same run down scenarios in two identical sandboxes (run-up.sh's "acme",
# with a stack whose stop command fails), one per binary, one after the other:
# legacy `wtm run down` against `wtm-next run down`, each side starting its jobs
# with its own `run up`. Each sandbox has its own HOME, so its own run daemon,
# socket and index; its jobs are stopped, its daemon and its compose containers
# gone before the next side starts. After each scenario it records the exit code,
# the output and the state, then diffs the states. Then, in a fresh sandbox per
# side, the wizard in tmux. Outputs differ by design (next's v2 contract); the
# states may differ only where next/docs/proof/README.md says why.
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
  touches = ["db"]

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
    remove = "grep -vx \"\$WTM_NAMESPACE\" \"\$HOME/namespaces.txt\" > \"\$HOME/ns.tmp\"; mv \"\$HOME/ns.tmp\" \"\$HOME/namespaces.txt\""

[[job]]
  name = "cache"
  kind = "service"
  cmd = "docker compose up -d cache"
  stop = "docker compose down"
  [job.ports]
    CACHE_PORT = $((port + 4))

[[job]]
  name = "crashy"
  kind = "service"
  cmd = "echo starting; sleep 0.2; echo boom >&2; exit 1"
  binds_no_port = true

[[job]]
  name = "fail"
  kind = "task"
  cmd = "echo nope; exit 3"

[[profile]]
  name = "full"
  jobs = ["migrate", "db", "cache", "api", "web"]

[[profile]]
  name = "broken"
  jobs = ["fail", "web"]

[[profile]]
  name = "crash"
  jobs = ["api", "crashy"]

[[job]]
  name = "stuck"
  kind = "service"
  cmd = "echo up"
  stop = "test -f \"$HOME/unstick\" || { echo cannot stop; exit 4; }"

[[profile]]
  name = "stuck"
  jobs = ["api", "stuck"]
TOML
  local branch
  for branch in feat/a feat/b; do
    "$legacy" create "$branch" --from main --yes </dev/null >/dev/null 2>&1
  done
  "$legacy" create feat/v --from main --isolation verbatim --yes </dev/null >/dev/null 2>&1
}

# teardown: every job of the sandbox stopped (its stacks' stop commands run), then its daemon.
teardown() {
  "$legacy" run down --all --yes </dev/null >/dev/null 2>&1 || true
  "$legacy" run daemon stop --yes </dev/null >/dev/null 2>&1 || true
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

# quietly runs the side's own binary to set a scenario up.
quietly() {
  "$bin" "$@" </dev/null >/dev/null 2>&1 || true
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
    print(os.path.basename(job["work_dir"]), job["name"], hold, job.get("profile", "-"))
PY
  for meta in $(find .git/wtm/worktrees -name meta.json | sort); do
    echo "## $meta"
    grep -E '"(ordinal|namespaces|isolation)"|^    "' "$meta" || true
  done
  echo "## logs"
  (cd .git/wtm/logs 2>/dev/null && find . -type f | sort) || true
  echo "## namespaces made"
  cat "$HOME/namespaces.txt" 2>/dev/null | sort -u || true
  echo "## compose containers"
  docker ps --format '{{.Names}}' 2>/dev/null | grep '^acme-' | sort || true
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
    prepare "$root" 24750
    trap teardown EXIT
    quietly run up feat/a --profile full -d --yes
    quietly run up feat/b --profile full -d --yes
    quietly run up main -d --yes
    state >"$dir/00-before.state"
    scenario 01-one run down feat/a --yes
    scenario 02-again run down feat/a --yes
    scenario 03-several-json run down feat/b main --yes --output json
    quietly run up feat/a --profile full -d --yes
    scenario 04-profile run down feat/a --profile dev --yes
    scenario 05-unknown-profile run down feat/a --profile ghost --yes
    scenario 06-unknown-json run down nope --yes --output json
    scenario 07-all-narrowed run down feat/a --all --yes
    quietly run up feat/b -d --yes
    scenario 08-all-json run down --all --yes --output json
    quietly run up feat/a --profile full -d --yes
    "$legacy" run daemon stop --yes </dev/null >/dev/null 2>&1 || true
    scenario 09-daemon-down run down feat/a --yes
    quietly run up feat/b --profile stuck -d --yes
    scenario 10-stop-fails run down feat/b --yes
    scenario 11-stop-fails-json run down feat/b --yes --output json
    touch "$HOME/unstick"
    quietly run down feat/b --yes
    # next only: legacy has no dry run
    quietly run up feat/a --profile full -d --yes
    quietly run up feat/b --profile full -d --yes
    scenario 12-dry-run run down feat/a --dry-run
  )
  local escaped=${root//\//%2F}
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e "s#(%2Fprivate)?$escaped#<SB>#g" \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#wtm-next run#wtm run#g' \
    "$dir"/*
}

# tty_side <side> <binary>: the wizard in tmux from feat/a, in a sandbox of its own: the worktrees (feat/a ticked), then
# legacy's stop, or next's recap and its outcome.
tty_side() {
  local side=$1 bin=$2 root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  local tty=$out/tty/$side session=wtmrundown-$side-$(basename "$root")
  rm -rf "$tty" && mkdir -p "$tty"
  (
    prepare "$root" 24770
    trap 'tmux kill-session -t "$session" 2>/dev/null; teardown' EXIT
    "$bin" run up feat/a -d --yes </dev/null >/dev/null 2>&1
    "$bin" run up feat/b -d --yes </dev/null >/dev/null 2>&1
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
      echo "run-down.sh: the $side screen never settled on \"$1\"" >&2
      return 1
    }
    capture() {
      settle "$2"
      screen >"$tty/$1.txt"
    }
    tmux new-session -d -s "$session" -x 100 -y 40 -c "$root/acme.trees/feat-a" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
    settle '$'
    tmux send-keys -t "$session" "wtmx run down; echo EXIT=\$?" Enter
    capture 1-worktrees 'feat/b'
    tmux send-keys -t "$session" Enter
    if [[ $side == legacy ]]; then
      capture 2-outcome 'EXIT='
    else
      capture 2-recap 'No, cancel'
      tmux send-keys -t "$session" Enter
      capture 3-outcome 'EXIT='
    fi
  )
  sed -i '' -E -e "s#(/private)?$root#<SB>#g" -e 's#wtmx#wtm#g' -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' "$tty"/*
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

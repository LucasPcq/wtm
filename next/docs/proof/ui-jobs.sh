#!/usr/bin/env bash
# Usage: ui-jobs.sh <wtm binary> <wtm-next binary> <out dir>
# Serves the Running page of wtm-next ui (LUC-342 PR D: a job's Restart, Stop and
# Release, Start one job…) over a throwaway sandbox with its own HOME, so its own
# daemon, socket, index and server: docs/demos/setup.sh's acme (made with the
# legacy wtm), widened with a shared service db and two jobs, worker (its stop takes
# 3 s, so that a restart under way can be seen) and stuck (whose stop command fails
# until $HOME/unstick exists). main holds db itself, feat/a and feat/b claim it;
# feat/a runs full. It prints the page's address, then waits for POST /api/stop;
# then it stops every job, records the state on disk into <out dir>/state.txt and
# deletes the sandbox. Screenshots are taken by hand on the address it prints.
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

SB=$(mktemp -d /tmp/wtmuijXXXXX)
# never a pid read from a file: what runs from $SB is the sandbox's own, which pkill matches
trap 'pkill -f -- "$SB" || true; rm -rf "$SB"' EXIT
port=24910
WTM_DEMO_ROOT=$SB WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$port WTM_DEMO_API_PORT=$((port + 1)) WTM_DEMO_PROXY_PORT=$((port + 2)) \
  "$repo/docs/demos/setup.sh" >/dev/null
export HOME=$SB/home
# run from the sandbox, so that its daemon and server match the trap's pkill
mkdir -p "$SB/bin" && cp "$next" "$SB/bin/wtm-next" && next=$SB/bin/wtm-next
cd "$SB/acme"
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
git add -A && git -c core.hooksPath=/dev/null commit -qm "chore: db"
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

[[job]]
  name = "worker"
  kind = "service"
  cmd = "trap 'echo draining the queue; sleep 3; echo drained; exit 0' TERM; echo worker up; while true; do sleep 1; echo tick; done"
  binds_no_port = true

[[job]]
  name = "stuck"
  kind = "service"
  cmd = "echo up"
  stop = "test -f \"$HOME/unstick\" || { echo cannot stop; exit 4; }"

[[job]]
  name = "migrate"
  kind = "task"
  cmd = "echo migrating; sleep 1; echo migrated"

[[profile]]
  name = "full"
  jobs = ["db", "api", "web", "worker", "stuck"]
TOML
for branch in feat/a feat/b; do
  "$legacy" create "$branch" --from main --yes </dev/null >/dev/null 2>&1
done

quietly() { "$next" "$@" </dev/null >/dev/null 2>&1 || true; }
quietly run start main --job db -d --yes
quietly run up feat/a --profile full -d --yes
quietly run start feat/b --job db -d --yes

"$next" ui --no-open >/dev/null 2>&1
web=$HOME/Library/Application\ Support/wtm/web.json
url="http://127.0.0.1:$(jq -r .port "$web")/#token=$(jq -r .token "$web")"
echo "$url" >"$out/url.txt"
echo "serving: $url"
echo "stop it: curl -X POST -H \"X-Wtm-Token: $(jq -r .token "$web")\" http://127.0.0.1:$(jq -r .port "$web")/api/stop"
until ! curl -s -o /dev/null "http://127.0.0.1:$(jq -r .port "$web")/api/version"; do sleep 2; done

{
  echo "## jobs listed"
  "$next" list --output json 2>/dev/null | jq -r '.items[] | .subject as $w | (.detail.jobs // [])[] | "\($w) \(.name) \(.hold) \(.state // "-")"' || true
  echo "## run marks per log"
  (cd .git/wtm/logs && for log in $(find . -name '*.log' | sort); do echo "$log $(grep -c '\[wtm\] started' "$log")"; done) || true
  echo "## namespaces made"
  sort -u "$HOME/namespaces.txt" 2>/dev/null || true
} | sed -E "s#(/private)?$SB#<SB>#g" >"$out/state.txt"
touch "$HOME/unstick"
quietly run down --all --yes

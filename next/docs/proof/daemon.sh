#!/usr/bin/env bash
# Usage: daemon.sh <wtm binary> <wtm-next binary> <wtm-next binary of another build> <out dir>
# Drives the run daemon raw over its socket (daemon.py), before any `run`
# command exists, in one sandbox (the docs/demos "acme" project): the legacy
# daemon handing its jobs over to next's, next's daemon starting, stopping and
# restarting jobs, a hard kill of next's daemon and its recovery, a daemon of
# another build. Every daemon this script runs it starts itself, as a child it
# holds the handle of, and stops through the protocol, or kills by that handle:
# it never signals a pid read from a file or an answer. Each step records what
# the daemon answered; pids, times and the sandbox path are normalised.
set -euo pipefail

absolute() {
  [[ -f $1 && -x $1 ]] || { echo "$0: $1 is not an executable" >&2; return 1; }
  echo "$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
}
legacy=$(absolute "${1:?wtm binary}")
next=$(absolute "${2:?wtm-next binary}")
other=$(absolute "${3:?wtm-next binary of another build}")
out=${4:?out dir}
mkdir -p "$out" && out=$(cd "$out" && pwd)
here=$(cd "$(dirname "$0")" && pwd)
repo=$(git rev-parse --show-toplevel)

# next's build, as its daemon stamps it on each answer: normalised, since it changes with every commit
build=$("$next" --version | awk '{print $NF}')
root=$(mktemp -d /tmp/wtmXXXXX)
port=$((20000 + RANDOM % 9000))
WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$port WTM_DEMO_API_PORT=$((port + 1)) WTM_DEMO_PROXY_PORT=$((port + 2)) \
  "$repo/docs/demos/setup.sh" >/dev/null
export HOME=$root/home
global=$HOME/Library/Application\ Support/wtm
socket=$global/wtm.sock
# the worktree as git spells it, symbolic links resolved: the daemon knows a job by that path, as the legacy one recorded it
acme=$(cd "$root/acme" && pwd -P)
rm -rf "${out:?}"/* && cd "$acme"

daemon_pid=""
# a daemon of the sandbox never outlives the run: asked to stop, else killed by the handle this script holds
cleanup() {
  [[ -S $socket ]] && python3 -I "$here/daemon.py" "$socket" '{"action":"shutdown"}' >/dev/null 2>&1 || true
  [[ -n $daemon_pid ]] && { kill -9 "$daemon_pid" 2>/dev/null || true; wait "$daemon_pid" 2>/dev/null || true; }
  rm -rf "$root"
}
trap cleanup EXIT

normalise() {
  sed -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"(pid|pgid|daemon_pid)":[0-9]+#"\1":<PID>#g' \
    -e 's#"(at|started_at|ts)":"[^"]+"#"\1":"<T>"#g' \
    -e 's#up [0-9]+[smh]#up <T>#g' \
    -e "s#$((port + 2))#<PROXY_PORT>#g" -e "s#$((port + 1))#<API_PORT>#g" -e "s#$port#<WEB_PORT>#g" \
    -e "s#\"version\":\"$build\"#\"version\":\"<NEXT>\"#g" \
    -e "s#$next|$other|$legacy#<BIN>#g"
}

# by_name: each JSON line with its jobs sorted by name, the legacy daemon listing them in no order.
by_name() {
  python3 -I -c 'import json, sys
for line in sys.stdin:
    doc = json.loads(line)
    doc["jobs"] = sorted(doc.get("jobs") or [], key=lambda job: job["name"])
    print(json.dumps(doc, separators=(",", ":")))'
}

# ask <step> <request>: sends request to the daemon on the socket, records its answers.
ask() {
  python3 -I "$here/daemon.py" "$socket" "$2" | normalise >"$out/$1.ndjson"
}

# start_daemon <binary>: runs binary's daemon as a child of this script, and waits for it to answer.
start_daemon() {
  "$1" daemon 2>>"$out/daemon.stderr" &
  daemon_pid=$!
  local tries=0
  until python3 -I "$here/daemon.py" "$socket" '{"action":"list"}' >/dev/null 2>&1; do
    ((tries++ < 100)) || { echo "daemon.sh: the daemon never answered" >&2; return 1; }
    sleep 0.05
  done
}

# stop_daemon: asks the daemon to stop, then waits for the child to exit.
stop_daemon() {
  python3 -I "$here/daemon.py" "$socket" '{"action":"shutdown"}' >/dev/null
  wait "$daemon_pid" || true
  daemon_pid=""
}

# alive <pid>: whether ps lists the process, without signalling it.
alive() {
  ps -p "$1" >/dev/null 2>&1 && echo alive || echo gone
}

spec() { # spec <name> <kind> <cmd> [stop]
  printf '{"job":{"name":"%s","kind":"%s","cmd":"%s"%s},"work_dir":"%s","env":{"WTM_BRANCH":"main"},"origin":{"root":"%s","common_dir":"%s/.git","branch":"main"}}' \
    "$1" "$2" "$3" "${4:+,\"stop\":\"$4\"}" "$acme" "$acme" "$acme"
}

# A detached stack: its launcher leaves a loop running until its marker goes, which its stop removes; no signal ever stops it.
marker=$root/db.up
cat >>"$acme/.git/wtm/run.toml" <<EOF

[[job]]
  name = "db"
  kind = "service"
  cmd = "touch $marker && (nohup sh -c 'while [ -f $marker ]; do sleep 1; done' >/dev/null 2>&1 &) && echo db launched"
  stop = "rm -f $marker && echo db stopped"

[[profile]]
  name = "all"
  jobs = ["db", "api", "web"]
EOF

# 1. The legacy daemon of 0.30, started by legacy run up, holds a stack and two services; next reads it as it spells them.
"$legacy" run up --profile all --yes </dev/null 2>&1 | normalise >"$out/01-legacy-run-up.txt"
"$next" status </dev/null 2>&1 | sed -n '/JOBS/,$p' | normalise >"$out/02-next-status-reads-legacy.txt"
python3 -I "$here/daemon.py" "$socket" '{"action":"list"}' | by_name | normalise >"$out/03-legacy-list.ndjson"
ask 04-legacy-stop-unknown '{"action":"stop","name":"nope","work_dir":"/x"}'
# 2. Asked to stop through its own protocol, it stops its services and leaves the stack in the index it shares with next.
ask 05-legacy-shutdown '{"action":"shutdown"}'
# gone once its lock is free: the kernel releases it as the process exits, its services stopped and its index written
python3 -I -c 'import fcntl, sys, time
lock = open(sys.argv[1], "a")
for _ in range(600):
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        sys.exit(0)
    except BlockingIOError:
        time.sleep(0.05)
sys.exit(1)' "$global/wtm.lock"
by_name <"$global/jobs.json" | normalise >"$out/06-index-left-by-legacy.json"

# 3. next's daemon takes the stack over, and stops it with its stop command.
start_daemon "$next"
ask 07-next-adopts-legacy-stack '{"action":"list"}'
ask 08-next-stops-the-stack "{\"action\":\"stop\",\"name\":\"db\",\"work_dir\":\"$acme\"}"
echo "marker $( [[ -f $marker ]] && echo present || echo removed)" >"$out/08-marker.txt"

# 4. Jobs started, listed, stopped and started again, and what it refuses.
ask 09-start-task "{\"action\":\"start\",\"spec\":$(spec migrate task 'echo 2 migrations applied')}"
ask 10-start-service "{\"action\":\"start\",\"spec\":$(spec web service 'echo ready; exec sleep 300')}"
ask 11-start-same-again "{\"action\":\"start\",\"spec\":$(spec web service 'echo ready; exec sleep 300')}"
ask 12-start-failing-task "{\"action\":\"start\",\"spec\":$(spec broken task 'echo no database >&2; exit 3')}"
ask 13-list '{"action":"list"}'
ask 14-stop-unknown '{"action":"stop","name":"nope","work_dir":"/x"}'
ask 15-stop-service "{\"action\":\"stop\",\"name\":\"web\",\"work_dir\":\"$acme\",\"correlation_id\":\"proof-stop\"}"
ask 16-restart-service "{\"action\":\"start\",\"spec\":$(spec web service 'echo ready; exec sleep 300')}"
ask 17-unknown-action '{"action":"bogus"}'
ask 18-not-json 'notjson'
"$next" daemon >"$out/19-second-daemon.stdout" 2>"$out/19-second-daemon.stderr" && echo 0 >"$out/19-second-daemon.exit" || echo $? >"$out/19-second-daemon.exit"

# 5. A hard kill of the daemon: its service dies with the terminal it held, a service deaf to SIGHUP survives as an orphan.
# its loop ends once the sandbox goes, so a proof that aborts never leaks it
ask 20-start-deaf "{\"action\":\"start\",\"spec\":$(spec deaf service "trap '' HUP; while [ -d $root ]; do sleep 1; done")}"
web_pid=$(python3 -I "$here/daemon.py" "$socket" '{"action":"list"}' | python3 -I -c 'import json,sys; print(next(j["pid"] for j in json.loads(sys.stdin.read())["jobs"] if j["name"]=="web"))')
deaf_pid=$(python3 -I "$here/daemon.py" "$socket" '{"action":"list"}' | python3 -I -c 'import json,sys; print(next(j["pid"] for j in json.loads(sys.stdin.read())["jobs"] if j["name"]=="deaf"))')
normalise <"$global/jobs.json" >"$out/21-index-before-kill.json"
kill -9 "$daemon_pid"
wait "$daemon_pid" 2>/dev/null || true
daemon_pid=""
sleep 0.5
printf 'web %s\ndeaf %s\nsocket %s\n' "$(alive "$web_pid")" "$(alive "$deaf_pid")" "$([[ -S $socket ]] && echo left || echo gone)" >"$out/22-after-kill.txt"
"$next" status </dev/null 2>&1 | grep -E '!|JOBS' | normalise >"$out/23-status-with-no-daemon.txt" || true
start_daemon "$next"
ask 24-recovered-list '{"action":"list"}'
printf 'deaf %s\n' "$(alive "$deaf_pid")" >"$out/25-orphan-after-recovery.txt"
normalise <"$global/jobs.json" >"$out/26-index-after-recovery.json"
stop_daemon

# 6. A daemon of another build: every answer is stamped with its build, and next's reads take it as it is.
start_daemon "$other"
ask 27-other-build-start "{\"action\":\"start\",\"spec\":$(spec web service 'echo ready; exec sleep 300')}"
"$next" status </dev/null 2>&1 | sed -n '/JOBS/,$p' | normalise >"$out/28-status-reads-other-build.txt"
stop_daemon
sed -E -i '' "s#(/private)?$root#<SB>#g" "$out"/*.stdout "$out"/*.stderr
echo "$out"

#!/usr/bin/env bash
# Usage: events.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same event-stream scenarios in two identical sandboxes (the docs/demos
# "acme" project), one per binary: legacy `wtm events` against `wtm-next events`.
# A stream is started in the background, read until its ready line, given a
# change made by the same binary in another process (a create), then interrupted.
# Each scenario records the exit code, stdout and stderr; repos.json is recorded
# empty before the scenarios and after them. Outputs differ by design (next's v2
# stream); every difference is listed in next/docs/proof/README.md.
set -euo pipefail
set -m # a background stream runs in its own process group and takes SIGINT as a terminal's Ctrl+C

absolute() {
  [[ -f $1 && -x $1 ]] || { echo "$0: $1 is not an executable" >&2; return 1; }
  echo "$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
}
legacy=$(absolute "${1:?wtm binary}")
next=$(absolute "${2:?wtm-next binary}")
out=${3:?out dir}
mkdir -p "$out" && out=$(cd "$out" && pwd)
repo=$(git rev-parse --show-toplevel)

prepare() {
  local root=$1 port=$((20000 + RANDOM % 9000))
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$port WTM_DEMO_API_PORT=$((port + 1)) WTM_DEMO_PROXY_PORT=$((port + 2)) \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  "$legacy" create feat/a --yes </dev/null >/dev/null 2>&1
  "$legacy" run daemon stop --yes </dev/null >/dev/null 2>&1 || true
  mkdir -p "$root/plain" "$root/bare"
  git -C "$root/bare" init -q
  # every side starts from no repository known: what each registers shows in registry-after
  rm -f "$global/repos.json"
}

# wait_for <file> <pattern> [pid]: until a line of file matches pattern, or pid has ended, for 20 seconds at most.
wait_for() {
  local tries=0
  until grep -Eq "$2" "$1" 2>/dev/null || { [[ -n ${3:-} ]] && ! kill -0 "$3" 2>/dev/null; }; do
    ((tries++ < 200)) || { echo "events.sh: $1 never matched $2" >&2; return 1; }
    sleep 0.1
  done
}

# scenario <name> <dir> <args…>: runs the binary with args in dir, without a terminal, to its end.
scenario() {
  local name=$1 dir=$2 code=0
  shift 2
  (cd "$dir" && "$bin" "$@" </dev/null >"$dir_out/$name.stdout" 2>"$dir_out/$name.stderr") || code=$?
  echo "$code" >"$dir_out/$name.exit"
}

# stream <name> <dir> <branch> <args…>: starts the stream in dir, waits for its ready line, creates branch from another process when one is named, waits for its provisioning, then interrupts the stream as Ctrl+C does.
stream() {
  local name=$1 dir=$2 branch=$3 code=0
  shift 3
  (cd "$dir" && exec "$bin" "$@" </dev/null >"$dir_out/$name.stdout" 2>"$dir_out/$name.stderr") &
  local pid=$!
  if [[ " $* " == *" -q "* ]]; then
    sleep 3 # a quiet stream prints no ready line
  else
    wait_for "$dir_out/$name.stdout" '"type":"ready"|watching for changes' "$pid"
  fi
  if [[ -n $branch ]]; then
    (cd "$root/acme" && "$bin" create "$branch" --yes </dev/null >/dev/null 2>&1)
    wait_for "$dir_out/$name.stdout" 'provisioned'
  fi
  kill -INT "$pid" 2>/dev/null || true # a stream that ended on its own error has nothing left to interrupt
  wait "$pid" || code=$?
  echo "$code" >"$dir_out/$name.exit"
}

# socket_reader <name> <dir> <args…>: runs the binary with args in dir, its stdout a socket whose reader leaves after the ready line, as a Node child's stdio does; records whether it ended within 3 seconds. Leaving before it would race legacy's write of that line.
socket_reader() {
  local name=$1 dir=$2
  shift 2
  (cd "$dir" && python3 -c '
import socket, subprocess, sys
ours, theirs = socket.socketpair()
child = subprocess.Popen(sys.argv[1:], stdin=subprocess.DEVNULL, stdout=theirs, stderr=subprocess.DEVNULL)
theirs.close()
seen = b""
while b"\"type\":\"ready\"" not in seen:
    chunk = ours.recv(4096)
    if not chunk:
        break
    seen += chunk
ours.close()
try:
    print("ended with", child.wait(timeout=3))
except subprocess.TimeoutExpired:
    child.kill()
    child.wait()
    print("still running 3 seconds after its reader left")
' "$bin" "$@" >"$dir_out/$name.stdout")
}

# lifecycle <name>: streams every repository from outside any, initializes the repository late (acme's config copied in) and runs list in it, which registers it, then de-initializes it and registers later, whose registration prunes late; then interrupts the stream.
lifecycle() {
  local name=$1 code=0
  (cd "$root/plain" && exec "$bin" events --all --output json </dev/null >"$dir_out/$name.stdout" 2>"$dir_out/$name.stderr") &
  local pid=$!
  wait_for "$dir_out/$name.stdout" '"type":"ready"'
  local r
  for r in late later; do
    git init -q "$root/$r"
    git -C "$root/$r" -c user.name=x -c user.email=x@x commit -q --allow-empty -m init
    mkdir -p "$root/$r/.git/wtm" && cp "$root/acme/.git/wtm/config.toml" "$root/$r/.git/wtm/"
  done
  (cd "$root/late" && "$bin" list </dev/null >/dev/null 2>&1)
  wait_for "$dir_out/$name.stdout" 'repo.added'
  rm "$root/late/.git/wtm/config.toml"
  (cd "$root/later" && "$bin" list </dev/null >/dev/null 2>&1)
  wait_for "$dir_out/$name.stdout" 'repo.removed'
  sleep 0.5 # later's snapshot follows its repo.added
  kill -INT "$pid"
  wait "$pid" || code=$?
  echo "$code" >"$dir_out/$name.exit"
}

registry() {
  if [[ -f $global/repos.json ]]; then cat "$global/repos.json"; else echo "(no repos.json)"; fi
}

run_side() {
  local side=$1
  bin=$2
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  global=$root/home/Library/Application\ Support/wtm
  dir_out=$out/$side
  rm -rf "$dir_out" && mkdir -p "$dir_out"
  (
    prepare "$root"
    registry >"$dir_out/registry-before"
    stream 01-text "$root/acme" feat/text events
    stream 02-json "$root/acme" feat/json events --output json
    stream 03-all-json "$root/plain" feat/all events --all --output json
    stream 04-outside-any-repository "$root/plain" "" events --output json
    stream 05-repo-quiet "$root/plain" "" events --repo "$root/acme" -q
    local code=0
    (cd "$root/acme" && "$bin" events --output json </dev/null 2>"$dir_out/06-head.stderr" | head -n1 >"$dir_out/06-head.stdout"; echo "${PIPESTATUS[0]}" >"$dir_out/06-head.exit") || code=$?
    socket_reader 12-socket-reader-gone "$root/acme" events --output json
    scenario 07-all-with-repo "$root/acme" events --all --repo "$root/acme"
    scenario 08-repo-not-a-directory "$root/acme" events --repo "$root/missing"
    scenario 09-repo-not-a-repository "$root/acme" events --repo "$root/plain"
    scenario 10-no-config "$root/bare" events
    scenario 11-no-config-json "$root/bare" events --output json
    lifecycle 13-all-repository-joins-and-leaves
    (export WTM_PROJECT_DIR=$root/plain && stream 14-relative-repo-project-dir-elsewhere "$root" "" events --repo acme --output json)
    registry >"$dir_out/registry-after"
    if [[ $side == legacy ]]; then
      "$legacy" run daemon stop --yes </dev/null >/dev/null 2>&1 || true
    else
      pkill -f "^$bin daemon$" || true
    fi
  )
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"(ts|created_at|added_at)": ?"[^"]+"#"\1":"<T>"#g' \
    -e 's#"op_id": ?"[0-9a-f]+"#"op_id":"<ID>"#g' \
    -e "s#$bin#<BIN>#g" \
    -e 's#wtm-next#wtm#g' \
    "$dir_out"/*
}

roots=()
# a stream or a daemon of a sandbox never outlives the run
trap 'for r in ${roots[@]+"${roots[@]}"}; do pkill -f -- "$r" || true; done; pkill -f "^($legacy|$next) daemon" || true; rm -rf ${roots[@]+"${roots[@]}"}' EXIT
run_side legacy "$legacy"
run_side next "$next"
echo "$out"

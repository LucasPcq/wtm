#!/usr/bin/env bash
# Usage: ui.sh <wtm-next binary> <out dir>
# Drives the web server of wtm-next ui (LUC-357) end to end in a throwaway sandbox:
# two repositories A and B with their own HOME, the server started from A and
# reused from B, every route called with curl, an operation cancelled, forced and
# left by its client, the event stream of a commit and a create, the guard's
# refusals, the idle stop and a restart. Each step writes one transcript,
# <out>/NN-name.txt: every command as `$ …`, then its output. The sandbox path,
# the token, repository ids, pids, operation ids, fingerprints, the version and
# timestamps are replaced by placeholders.
set -euo pipefail

absolute() {
  [[ -f $1 && -x $1 ]] || { echo "$0: $1 is not an executable" >&2; return 1; }
  echo "$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
}
next=$(absolute "${1:?wtm-next binary}")
out=${2:?out dir}
rm -rf "$out" && mkdir -p "$out" && out=$(cd "$out" && pwd)

SB=$(mktemp -d /tmp/wtmuiXXXXX)
export HOME=$SB/home PATH=$SB/bin:$PATH GIT_AUTHOR_DATE=2026-10-16T12:00:00Z GIT_COMMITTER_DATE=2026-10-16T12:00:00Z
G=$HOME/Library/Application\ Support/wtm
mkdir -p "$HOME" "$SB/bin" && cp "$next" "$SB/bin/wtm-next"
# the sandbox, its server and every background curl never outlive the run
# never a pid read from web.json: a stale one may be anyone's; the server runs from $SB, which pkill matches
# only the jobs still running: a pid wait already reaped may be another process's by now
trap 'kill $(jobs -p) 2>/dev/null || true; pkill -f -- "$SB" || true; rm -rf "$SB"' EXIT

# A create hook that sleeps on slow/* branches gives an operation long enough to cancel.
for r in A B; do
  git init -q -b main "$SB/$r"
  git -C "$SB/$r" -c user.name=x -c user.email=x@x commit -q --allow-empty -m init
  mkdir -p "$SB/$r/.git/wtm"
  cat >"$SB/$r/.git/wtm/config.toml" <<'TOML'
[worktrees]
base_path = "../.trees"

[hooks]
on_create = ["case {{branch}} in slow/*) sleep 20;; esac"]
TOML
done

step=
section() { step=$out/$1.txt; : >"$step"; }
# show <command>: records the command, then runs it and records its output.
show() {
  printf '$ %s\n' "$*" >>"$step"
  eval "$@" >>"$step" 2>&1 || echo "(exit $?)" >>"$step"
  echo >>"$step"
}
note() { printf '# %s\n' "$*" >>"$step"; }
# wait_for <file> <pattern> [seconds]: until a line of file matches pattern.
wait_for() {
  local tries=0 max=$((${3:-20} * 10))
  until grep -Eq "$2" "$1" 2>/dev/null; do
    ((tries++ < max)) || { note "never matched within ${3:-20}s: $2"; return 1; }
    sleep 0.1
  done
}
api() { curl -s -H "X-Wtm-Token: $TOKEN" "$@"; }
# a background stream runs curl itself, not api: killing a function's subshell would leave its curl running
answers() { printf '{"answers":[{"path":"branches","values":["%s"]}]%s}' "$1" "${2:-}"; }

section 01-start
show 'cd $SB/A && wtm-next ui --no-open --idle 40s'
cd "$SB/A"
TOKEN=$(jq -r .token "$G/web.json") PORT=$(jq -r .port "$G/web.json") PID1=$(jq -r .pid "$G/web.json")
U=http://127.0.0.1:$PORT
show 'ls -l "$G" | awk "{print \$1, \$NF}"'
show 'cat "$G/web.json"'
show 'ps -o pid=,command= -p $PID1'

section 02-routes
show 'api $U/api/version'
show 'api $U/api/repos | jq .'
A=$(api "$U/api/repos" | jq -r '.[0].id')
show 'api $U/api/repos/$A/commands | jq -c ".[] | {name, questions: [.form.fields[]?.path]}"'
C=$U/api/repos/$A/commands/create
show "api -X POST \$C/questions -d '$(answers feat/x)' | jq -c '.fields[] | {path, value, entries}'"
note 'dry run: the plan and its fingerprint'
show "api -X POST \$C -d '$(answers feat/x ',"dry_run":true')' | jq ."
FP=$(api -X POST "$C" -d "$(answers feat/x ',"dry_run":true')" | jq -r .fingerprint)
note "apply with expect_fingerprint=$FP, as a stream"
show "api -N -D - -X POST -H 'Accept: text/event-stream' \$C -d '$(answers feat/x ",\"expect_fingerprint\":\"$FP\"")'"
note 'a stale fingerprint: feat/w planned as a new branch from main, then the branch made behind its back'
W='{"answers":[{"path":"branches","values":["feat/w"]},{"path":"from","values":["main"]}]'
FPW=$(api -X POST "$C" -d "$W,\"dry_run\":true}" | jq -r .fingerprint)
show "api -X POST \$C -d '$W,\"dry_run\":true}' | jq -c '{fingerprint, plan: .plan.branches}'"
show 'git -C $SB/A branch feat/w'
show "api -i -X POST \$C -d '$W,\"expect_fingerprint\":\"$FPW\"}'"

section 03-cancel
# op <branch> <name>: starts a create of branch as a stream in the background; sets OPPID (the curl) and OP (its operation id).
op() {
  curl -s -H "X-Wtm-Token: $TOKEN" -N -D "$out/.$2.h" -X POST -H 'Accept: text/event-stream' "$C" -d "$(answers "$1")" >"$out/.$2.body" &
  OPPID=$!
  wait_for "$out/.$2.h" '^X-Wtm-Operation'
  OP=$(tr -d '\r' <"$out/.$2.h" | awk '/^X-Wtm-Operation/ {print $2}')
}
curl -s -H "X-Wtm-Token: $TOKEN" -N "$U/api/events?repo=$A" >"$out/.bus" &
BUS=$!
note 'a create of slow/a (its hook sleeps 20s) as a stream; DELETE its operation while the hook runs'
op slow/a cancel
sleep 2
show 'api -i -X DELETE $U/api/repos/$A/operations/'"$OP"
wait "$OPPID" || true
show "tr -d '\r' <\"\$out/.cancel.h\" | grep -i '^X-Wtm-Operation'"
show 'cat "$out/.cancel.body"'
show 'git -C $SB/A worktree list'
note 'a create of slow/b; DELETE with ?force=true straight away'
op slow/b force
sleep 2
show 'api -i -X DELETE "$U/api/repos/$A/operations/'"$OP"'?force=true"'
wait_for "$out/.force.body" '^event: envelope' 30 && wait "$OPPID" || true
show 'cat "$out/.force.body"'
show 'git -C $SB/A worktree list'
note 'a create of slow/c whose client leaves (curl killed) while the hook runs: the operation goes on'
op slow/c leave
sleep 2
show 'kill $OPPID'; wait "$OPPID" 2>/dev/null || true
wait_for "$out/.bus" "$OP.*operation.finished|operation.finished.*$OP" 30 || true
show 'cat "$out/.leave.body"'
show "grep '$OP' \"\$out/.bus\""
show 'git -C $SB/A worktree list'
kill "$BUS" 2>/dev/null; wait "$BUS" 2>/dev/null || true

section 04-reuse
show 'cat "$G/web.json" | jq -c "{pid, port}"'
show 'cd $SB/B && wtm-next ui --no-open'
show 'cat "$G/web.json" | jq -c "{pid, port}"'
show 'api $U/api/repos | jq -c ".[] | {id, root}"'
B=$(api "$U/api/repos" | jq -r '.[] | select(.root | endswith("/B")) | .id')

section 05-events
note "the stream of A; a commit in A's feat/x worktree, then a create from the command line"
curl -s -H "X-Wtm-Token: $TOKEN" -N "$U/api/events?repo=$A" >"$out/.events" &
EV=$!
sleep 1
show 'git -C $SB/.trees/feat-x -c user.name=x -c user.email=x@x commit -q --allow-empty -m x'
wait_for "$out/.events" '"worktree.changed"' 10 || true
show 'grep worktree.changed "$out/.events"'
show 'cd $SB/A && wtm-next create feat/y --yes'
wait_for "$out/.events" 'operation.finished' 10 || true
sleep 1
kill "$EV" 2>/dev/null; wait "$EV" 2>/dev/null || true
show 'cat "$out/.events"'

section 06-refusals
show 'curl -s -i $U/api/version'
show 'curl -s -i -H "X-Wtm-Token: wrong" $U/api/version'
show 'api -i -H "Origin: http://evil.example" $U/api/version'
show 'api -i -H "Host: evil.example:$PORT" $U/api/version'
show 'api -i $U/api/schema | head -5'
show 'curl -s -i $U/'

section 07-idle
show 'kill $(jobs -p) 2>/dev/null; jobs -r | wc -l'
show 'kill -0 $PID1 && echo "server $PID1 running"'
show 'sleep 46; kill -0 $PID1 2>/dev/null || echo "server $PID1 gone"'
show 'cat "$G/web.json" 2>&1 | jq -c "{pid, port}"'
show 'cd $SB/A && wtm-next ui --no-open'
show 'cat "$G/web.json" | jq -c "{pid, port}"'
show 'kill -0 $(jq -r .pid "$G/web.json") && echo "new server $(jq -r .pid "$G/web.json") running"'
PID2=$(jq -r .pid "$G/web.json")

section 08-cleanup
show 'curl -s -o /dev/null -w "stop %{http_code}\n" -X POST -H "X-Wtm-Token: $(jq -r .token "$G/web.json")" "http://127.0.0.1:$(jq -r .port "$G/web.json")/api/stop"; sleep 1; pgrep -f -- "$SB" || echo "no process left in the sandbox"'

rm -f "$out"/.[a-z]*
sed -i '' -E \
  -e "s#(/private)?$SB#<SB>#g" \
  -e "s#$TOKEN#<TOKEN>#g" \
  -e 's#"token":"[0-9a-f]+"#"token":"<TOKEN>"#g' \
  -e 's#token=[0-9a-f]{64}#token=<TOKEN>#g' \
  -e "s#[[:<:]]${A:-<REPO_A>}[[:>:]]#<REPO_A>#g" \
  -e "s#[[:<:]]${B:-<REPO_B>}[[:>:]]#<REPO_B>#g" \
  -e "s#[[:<:]]${PID1:-<PID1>}[[:>:]]#<PID1>#g" \
  -e "s#[[:<:]]${PID2:-<PID2>}[[:>:]]#<PID2>#g" \
  -e 's#^ +(<PID[12]>)#\1#' \
  -e 's#"(ts|created_at|last_commit_at)":"[^"]+"#"\1":"<T>"#g' \
  -e 's#"version":"[^"]+"#"version":"<VERSION>"#g' \
  -e 's#^Date: .*#Date: <T>#' \
  -e 's#[A-Z][a-z]{2} [ 0-9][0-9] [0-9]{2}:[0-9]{2} #<DATE> #' \
  "$out"/*.txt
# operation ids and fingerprints numbered in order of first appearance, so a DELETE still names the operation it cancels
python3 - "$out"/*.txt <<'PY'
import re, sys
names = {}
def placeholder(match):
    token = match.group(0)
    kind = "FP" if len(token) == 24 else "OP"
    if token not in names:
        names[token] = "<%s%d>" % (kind, 1 + sum(n.startswith("<" + kind) for n in names.values()))
    return names[token]
for path in sys.argv[1:]:
    text = open(path, newline="").read()
    open(path, "w", newline="").write(re.sub(r"\b(?:[0-9a-f]{24}|[0-9a-f]{16})\b", placeholder, text))
PY
echo "$out"

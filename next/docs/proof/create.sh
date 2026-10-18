#!/usr/bin/env bash
# Usage: create.sh <wtm binary> <wtm-next binary> <out dir>
# Prints the two sandboxes it leaves behind, legacy then next, for tty.sh.
# Runs the same create scenarios in two identical sandboxes (the docs/demos "acme"
# project, same ports), one per binary: legacy `wtm create` against `wtm-next
# create`. After each scenario it records the exit code, the output and the state
# (branches, worktrees, every meta.json, every .env, config.toml), then diffs the
# states. Outputs differ by design (next's v2 contract); the states may differ
# only where next/docs/proof/README.md says why.
set -euo pipefail

legacy=${1:?wtm binary}
next=${2:?wtm-next binary}
out=${3:?out dir}
mkdir -p "$out" && out=$(cd "$out" && pwd)
repo=$(git rev-parse --show-toplevel)
hook='case {{branch}} in feat/bad*) echo failing; exit 3;; feat/slow*) trap "sleep 2" INT; sleep 30;; *) echo set up {{branch}};; esac'

prepare() {
  local root=$1
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=24100 WTM_DEMO_API_PORT=24101 WTM_DEMO_PROXY_PORT=24102 \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  python3 - "$hook" <<'PY'
import sys
path = ".git/wtm/config.toml"
text = open(path).read().replace("on_create = [\n]", 'on_create = [{ cmd = "%s" }]' % sys.argv[1].replace('"', '\\"'), 1)
open(path, "w").write(text)
PY
  git branch feature
  git branch topic
  git init -q --bare "$root/origin.git"
  git remote add origin "$root/origin.git"
  git push -q origin main
  git branch remote-only && git push -q origin remote-only && git branch -D -q remote-only
  git fetch -q origin
}

behind() {
  local root=$1
  git clone -q "$root/origin.git" "$root/upstream"
  (cd "$root/upstream" && git -c user.email=u@acme.dev -c user.name=Up commit -q --allow-empty -m "upstream" && git push -q origin main)
  git fetch -q origin
}

# config.toml as 0.30.1 writes it once "Use config default" is remembered, over env.strategy = "main".
remember_config_default() {
  python3 - <<'PY'
path = ".git/wtm/config.toml"
text = open(path).read().replace('strategy = "example"', 'strategy = "main"', 1)
open(path, "w").write(text + '\n[wizard.remembered]\nenv_strategy = "config"\n')
PY
}

scenario() {
  local name=$1
  shift
  local code=0
  "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr" || code=$?
  echo "$code" >"$dir/$name.exit"
  state >"$dir/$name.state"
}

state() {
  echo "## branches"
  git branch --format='%(refname:short) %(upstream:short)'
  echo "## worktrees"
  git worktree list --porcelain | grep '^worktree\|^branch'
  for meta in $(find .git/wtm/worktrees -name meta.json | sort); do
    echo "## $meta"
    grep -v created_at "$meta"
  done
  for env in $(find ../acme.trees -name .env -path '*apps*' | sort); do
    echo "## $env"
    cat "$env"
  done
  echo "## config.toml"
  cat .git/wtm/config.toml
}

run_side() {
  local side=$1
  bin=$2
  # each run its own sandbox, as long as /tmp/wtmcmp-l was: a screen of tty.sh wraps a path at 100 columns
  local root
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  dir=$out/$side
  rm -rf "$dir" && mkdir -p "$dir"
  (
    prepare "$root"
    scenario 01-new create feat/one --yes
    scenario 02-three-one-failing create feat/a feat/bad feat/c --yes
    scenario 03-existing-without-from create feature --yes
    scenario 04-existing-with-from create feature --from main --yes
    scenario 05-remote-only create remote-only --yes
    scenario 06-if-not-exists-1 create feat/idem --if-not-exists --yes
    scenario 07-if-not-exists-2 create feat/idem --if-not-exists --yes
    scenario 08-env-from-main create feat/env --env-from main --yes
    scenario 09-json-success create feat/json --output json --yes
    scenario 10-json-invalid create 'bad..name' feat/one --output json --yes
    scenario 11-json-failure create feat/bad-json --output json --yes
    scenario 12-quiet-failure create feat/one -q --yes
    behind "$root"
    scenario 13-ff-behind create feat/ff --from main --ff --yes
    remember_config_default
    scenario 14-remembered-config-default create feat/remembered --yes
    # next only: legacy has no dry run, fingerprint nor --remember, and refuses the flag.
    scenario 15-dry-run create feat/dry --env-from main --remember env_from --dry-run
    fingerprint=$(grep -o 'fingerprint  [0-9a-f]*' "$dir/15-dry-run.stdout" | awk '{print $2}' || true)
    scenario 16-expect-fingerprint create feat/dry --env-from main --remember env_from --expect-fingerprint "${fingerprint:-none}"
    scenario 17-stale-fingerprint create feat/stale --expect-fingerprint 000000000000000000000000
    # an apply that changes nothing (feat/json is open since 09), so tty.sh finds the sandbox as before
    scenario 18-json-dry-run create feat/json --if-not-exists --output json --dry-run
    fingerprint=$(grep -o '"fingerprint": "[0-9a-f]*"' "$dir/18-json-dry-run.stdout" | grep -o '[0-9a-f]\{24\}' || true)
    scenario 19-json-expect-fingerprint create feat/json --if-not-exists --output json --expect-fingerprint "${fingerprint:-none}"
    scenario 20-json-stale-fingerprint create feat/jstale --output json --expect-fingerprint 000000000000000000000000
  )
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#"created_at": "[^"]*"#"created_at": "<T>"#g' \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#wtm-next create#wtm create#g' \
    "$dir"/*
}

roots=()
# kept for tty.sh on success, removed on a failure
trap 'rm -rf "${roots[@]}"' ERR
run_side legacy "$legacy"
run_side next "$next"
{
  for state in "$out"/legacy/*.state; do
    name=$(basename "$state" .state)
    echo "=== $name: exit legacy $(cat "$out/legacy/$name.exit"), next $(cat "$out/next/$name.exit")"
    diff -u --label "legacy/$name" --label "next/$name" "$state" "$out/next/$name.state" || true
  done
} >"$out/states.diff"
echo "${roots[@]}"

#!/usr/bin/env bash
# Usage: compare.sh <wtm-engine binary> <wtm-next binary> <out dir>
# Runs the same create scenarios in two identical sandboxes (the docs/demos "acme"
# project, same ports), one per binary, then writes each side's normalised
# stdout, stderr, exit code and state, and their diff.
set -euo pipefail

engine=${1:?engine binary}
next=${2:?wtm-next binary}
out=${3:?out dir}
repo=$(git rev-parse --show-toplevel)
hook='case {{branch}} in feat/bad*) echo failing; exit 3;; feat/slow*) trap "sleep 2" INT; sleep 30;; *) echo set up {{branch}};; esac'

prepare() {
  local root=$1
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$engine WTM_DEMO_WEB_PORT=24100 WTM_DEMO_API_PORT=24101 WTM_DEMO_PROXY_PORT=24102 \
    "$repo/docs/demos/setup.sh"
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

scenario() {
  local name=$1
  shift
  local code=0
  "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr" || code=$?
  echo "$code" >"$dir/$name.exit"
}

state() {
  {
    echo "## branches"; git branch --format='%(refname:short) %(upstream:short)'
    echo "## worktrees"; git worktree list --porcelain | grep '^worktree\|^branch'
    for meta in $(find .git/wtm/worktrees -name meta.json | sort); do echo "## $meta"; cat "$meta"; echo; done
    for env in $(find ../acme.trees -name .env -path '*apps*' | sort); do echo "## $env"; cat "$env"; done
    echo "## config.toml"; cat .git/wtm/config.toml
  } >"$dir/state.txt"
}

run_side() {
  local side=$1
  bin=$2
  root=/tmp/wtmcmp-${side:0:1}
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
    scenario 08-dry-run create feat/dry --env-from main --remember env_from --dry-run
    fingerprint=$(grep -o 'fingerprint  [0-9a-f]*' "$dir/08-dry-run.stdout" | awk '{print $2}')
    scenario 09-expect-fingerprint create feat/dry --env-from main --remember env_from --expect-fingerprint "$fingerprint"
    scenario 10-stale-fingerprint create feat/stale --expect-fingerprint 000000000000000000000000
    scenario 11-json-success create feat/json --output json
    scenario 12-json-invalid create 'bad..name' feat/one --output json
    scenario 13-json-failure create feat/bad-json --output json
    scenario 14-quiet-failure create feat/one -q
    behind "$root"
    scenario 15-ff-behind create feat/ff --from main --ff --yes
    state
  )
  sed -i '' -E \
    -e "s#(/private)?/tmp/wtmcmp-${side:0:1}#<SB>#g" \
    -e 's#"created_at": "[^"]*"#"created_at": "<T>"#g' \
    -e 's#"op": "[0-9a-f]{16}"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' \
    -e 's#wtm-next create#wtm create#g' \
    "$dir"/*
}

mkdir -p "$out"
run_side engine "$engine"
run_side next "$next"
diff -ru "$out/engine" "$out/next" >"$out/diff.txt" || true
echo "$out/diff.txt"

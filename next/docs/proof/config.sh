#!/usr/bin/env bash
# Usage: config.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same config scenarios in two identical sandboxes (the docs/demos "acme"
# project), one per binary: legacy `wtm config show|edit` against `wtm-next config
# show|edit|set`. Before each scenario config.toml is put back as the scenario
# wants it (as init wrote it, a value wtm can't use, a file that doesn't parse, one
# a newer wtm wrote, none); each records its exit code, stdout and stderr, and the
# diff of config.toml it left. edit runs fake editors through $EDITOR (and $VISUAL)
# that write a valid file, an invalid one, a broken one, fail, remove the file, or
# take arguments. set is next's alone. Outputs differ by design; every difference
# is listed in next/docs/proof/README.md.
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

prepare() {
  local root=$1 port=$((20000 + RANDOM % 9000))
  WTM_DEMO_ROOT=$root WTM_DEMO_BIN=$legacy WTM_DEMO_WEB_PORT=$port WTM_DEMO_API_PORT=$((port + 1)) WTM_DEMO_PROXY_PORT=$((port + 2)) \
    "$repo/docs/demos/setup.sh" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
  config=$root/acme/.git/wtm/config.toml
  cp "$config" "$root/config.init"
  mkdir -p "$root/plain" "$root/editors"
  # the editors take the file as their last argument, as an editor named with arguments does
  printf '#!/bin/sh\nfor f; do :; done\nsed -i "" "s#base_path = .*#base_path = \\"../wt\\"#" "$f"\n' >"$root/editors/valid"
  printf '#!/bin/sh\nsed -i "" "s/strategy = \\"example\\"/strategy = \\"copy\\"/" "$1"\n' >"$root/editors/invalid"
  printf '#!/bin/sh\necho "x = [" >>"$1"\n' >"$root/editors/broken"
  printf '#!/bin/sh\nexit 3\n' >"$root/editors/fail"
  printf '#!/bin/sh\nrm "$1"\n' >"$root/editors/remove"
  chmod +x "$root/editors/"*
}

# config.toml as each scenario wants it: as init wrote it, or changed by sed, or none
as_init() { cp "$root/config.init" "$config"; }
as_copy() { sed 's/strategy = "example"/strategy = "copy"/' "$root/config.init" >"$config"; }
as_broken() { sed 's/base_branch = "main"/base_branch =/' "$root/config.init" >"$config"; }
as_newer() { { echo 'schema = 9'; cat "$root/config.init"; } >"$config"; }
as_none() { rm -f "$config"; }

# scenario <name> <state> <dir> <env> <args…>: env is VAR=value pairs separated by ";"; puts config.toml as state wants it, runs the binary without a terminal, records what it printed and the diff of config.toml it left.
scenario() {
  local name=$1 state=$2 dir=$3 env=$4
  shift 4
  "$state"
  local before=$root/before.toml
  { cat "$config" 2>/dev/null || echo "(no config.toml)"; } >"$before"
  local code=0 pairs=()
  [[ -n $env ]] && IFS=';' read -ra pairs <<<"$env"
  (cd "$dir" && env ${pairs[@]+"${pairs[@]}"} "$bin" "$@" </dev/null >"$dir_out/$name.stdout" 2>"$dir_out/$name.stderr") || code=$?
  echo "$code" >"$dir_out/$name.exit"
  diff -u --label "config.toml before" --label "config.toml after" "$before" <(cat "$config" 2>/dev/null || echo "(no config.toml)") >"$dir_out/$name.config.diff" || true
}

run_side() {
  local side=$1
  bin=$2
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  dir_out=$out/$side
  rm -rf "$dir_out" && mkdir -p "$dir_out"
  (
    prepare "$root"
    local acme=$root/acme ed=$root/editors
    scenario 01-show as_init "$acme" "" config show
    scenario 02-show-json as_init "$acme" "" config show --output json
    scenario 03-show-invalid-value as_copy "$acme" "" config show
    scenario 04-show-invalid-value-json as_copy "$acme" "" config show --output json
    scenario 05-show-no-parse as_broken "$acme" "" config show
    scenario 06-show-no-parse-json as_broken "$acme" "" config show --output json
    scenario 07-show-newer as_newer "$acme" "" config show
    scenario 08-show-newer-json as_newer "$acme" "" config show --output json
    scenario 09-show-no-config as_none "$acme" "" config show
    scenario 10-show-no-config-json as_none "$acme" "" config show --output json
    scenario 11-show-not-a-repository as_init "$root/plain" "" config show
    scenario 12-edit-valid as_init "$acme" "VISUAL=;EDITOR=$ed/valid" config edit
    scenario 13-edit-invalid-value as_init "$acme" "VISUAL=;EDITOR=$ed/invalid" config edit
    scenario 14-edit-no-parse as_init "$acme" "VISUAL=;EDITOR=$ed/broken" config edit
    scenario 15-edit-editor-fails as_init "$acme" "VISUAL=;EDITOR=$ed/fail" config edit
    scenario 16-edit-removes-the-file as_init "$acme" "VISUAL=;EDITOR=$ed/remove" config edit
    scenario 17-edit-editor-with-arguments as_init "$acme" "VISUAL=;EDITOR=$ed/valid --wait" config edit
    scenario 18-edit-visual-first as_init "$acme" "VISUAL=$ed/valid;EDITOR=$ed/fail" config edit
    scenario 19-edit-no-config as_none "$acme" "VISUAL=;EDITOR=$ed/valid" config edit
    # legacy starts vi with no terminal there, whose screen differs from run to run: next only
    if [[ $side == next ]]; then
      scenario 20-edit-no-editor-no-terminal as_init "$acme" "VISUAL=;EDITOR=" config edit
      scenario 21-set-dry-run as_init "$acme" "" config set --base-path ../wt --on-clean= --dry-run
      scenario 22-set as_init "$acme" "" config set --base-path ../wt --on-create "pnpm install" --yes
      scenario 23-set-json as_init "$acme" "" config set --env-strategy main --yes --output json
      scenario 24-set-refused as_init "$acme" "" config set --base-path /tmp/wt --yes
      scenario 25-set-forget as_init "$acme" "" config set --remembered= --yes
      scenario 26-set-newer as_newer "$acme" "" config set --base-path ../wt --yes
    fi
  )
  sed -i '' -E \
    -e "s#(/private)?$root#<SB>#g" \
    -e 's#[0-9a-f]{64}#<DIGEST>#g' \
    -e 's#"op": "[0-9a-f]+"#"op": "<OP>"#g' \
    -e 's#[0-9a-f]{24}#<FP>#g' \
    -e "s#$bin#<BIN>#g" \
    -e 's#wtm-next config#wtm config#g' \
    "$dir_out"/*
}

roots=()
trap 'rm -rf ${roots[@]+"${roots[@]}"}' EXIT
run_side legacy "$legacy"
run_side next "$next"
echo "$out"

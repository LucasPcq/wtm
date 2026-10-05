#!/usr/bin/env bash
# Usage: sandbox.sh <wtm-binary>     prints the sandbox directory
#        sandbox.sh --clean <dir>    stops its jobs and daemon, then deletes it
#
# A throwaway copy of the docs/demos "acme" project with its own HOME, so a
# run never reads or writes the real ~/.config/wtm, daemon or worktrees.
# `source <dir>/env.sh` enters it (HOME, PATH with the given wtm, short prompt).
set -euo pipefail

marker=.wtm-sandbox

if [[ "${1:-}" == "--clean" ]]; then
  dir=${2:?sandbox directory}
  [[ -f "$dir/$marker" ]] || { echo "refusing to delete $dir: not a sandbox" >&2; exit 1; }
  if [[ -d "$dir/acme" ]]; then
    (cd "$dir/acme" && HOME="$dir/home" "$dir/bin/wtm" run down --all --yes >/dev/null 2>&1 || true)
  fi
  HOME="$dir/home" "$dir/bin/wtm" run daemon stop --yes >/dev/null 2>&1 || true
  rm -rf "$dir"
  exit 0
fi

bin=${1:?wtm binary to put on PATH}
# Under /tmp, not $TMPDIR: macOS's long /var/folders path pushes the daemon's
# unix socket past the 104-byte limit.
dir=$(mktemp -d /tmp/wtm-sandbox.XXXXXX)
touch "$dir/$marker"
port_base=$((20000 + RANDOM % 9000))

WTM_DEMO_ROOT=$dir WTM_DEMO_BIN=$bin \
  WTM_DEMO_WEB_PORT=$port_base WTM_DEMO_API_PORT=$((port_base + 1)) WTM_DEMO_PROXY_PORT=$((port_base + 2)) \
  "$(git rev-parse --show-toplevel)/docs/demos/setup.sh"
touch "$dir/$marker"

cat >"$dir/env.sh" <<SH
export HOME=$dir/home PATH=$dir/bin:\$PATH
PS1='%1~ ❯ '
cd $dir/acme
SH
echo "$dir"

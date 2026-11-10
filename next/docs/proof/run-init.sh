#!/usr/bin/env bash
# Usage: run-init.sh <wtm binary> <wtm-next binary> <out dir>
# Runs the same run init scenarios on the monorepo of run-init-repo.sh, a fresh
# copy per scenario where it starts from no run.toml, legacy `wtm run init`
# against `wtm-next run init`, then next's only (a dry run, a fix answered, a
# job left out, the legacy reading next's run.toml, run up remembering to keep
# the others' jobs). After each, the exit code, the output and the state (run.toml,
# docker-compose.yml, config.toml, git status); then the wizard of each in tmux,
# on a tmux server of its own. No docker container is started: run init only
# writes files, and run up's scenario starts a task that echoes.
set -euo pipefail

absolute() {
  [[ -f $1 && -x $1 ]] || { echo "$0: $1 is not an executable" >&2; return 1; }
  echo "$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
}
legacy=$(absolute "${1:?wtm binary}")
next=$(absolute "${2:?wtm-next binary}")
out=${3:?out dir}
mkdir -p "$out" && out=$(cd "$out" && pwd)
here=$(cd "$(dirname "$0")" && pwd)
export TMUX_TMPDIR
TMUX_TMPDIR=$(mktemp -d /tmp/tmx.XXXX)

roots=()
# fresh: a new sandbox, entered, with config.toml as the legacy wtm init writes it.
fresh() {
  root=$(mktemp -d /tmp/wtmXXXXX)
  roots+=("$root")
  "$here/run-init-repo.sh" "$root" "$legacy" >/dev/null
  export HOME=$root/home
  cd "$root/acme"
}

state() {
  echo "## run.toml"
  cat .git/wtm/run.toml 2>/dev/null || echo "(none)"
  echo "## docker-compose.yml"
  cat docker-compose.yml
  echo "## config.toml, its .env files"
  grep -E '^(strategy|target|template|local)' .git/wtm/config.toml
  echo "## git status"
  git status --short
}

# scenario <name> <args…>: runs the side's binary with no terminal, then records what it left.
scenario() {
  local name=$1 code=0
  shift
  "$bin" "$@" </dev/null >"$dir/$name.stdout" 2>"$dir/$name.stderr" || code=$?
  echo "$code" >"$dir/$name.exit"
  state >"$dir/$name.state"
}

normalise() {
  sed -i '' -E -e "s#(/private)?/tmp/wtm[A-Za-z0-9]{5}#<SB>#g" -e 's#[0-9a-f]{24}#<FINGERPRINT>#g' -e 's#"op": "[^"]+"#"op": "<OP>"#g' -e 's#wtm-next #wtm #g' -e 's#\([0-9.]+(ms|s)\)#(<DURATION>)#g' "$@"
}

run_side() {
  local side=$1
  bin=$2
  dir=$out/$side
  rm -rf "$dir" && mkdir -p "$dir"
  (
    fresh
    scenario 01-first run init --yes
    scenario 02-again run init --yes
    fresh
    scenario 03-no-tty run init
    fresh
    scenario 04-json run init --yes --output json
    fresh
    # legacy rewrites docker-compose.yml only when asked; next rewrites it unless told not to
    if [[ $side == legacy ]]; then
      scenario 05-compose-flags run init --yes --patch-compose --link-env
    else
      scenario 05-compose-flags run init --yes --patch-compose=false
    fi
    if [[ $side == next ]]; then
      fresh
      scenario 06-dry-run run init --yes --dry-run
      scenario 07-fixed run init --yes --command 'admin=pnpm exec next dev -p ${ADMIN_PORT}'
      scenario 08-left-out run init --yes --job postgres --job redis --job mailpit --job admin --job api --job web --job migrate --job seed
      bin=$legacy scenario 09-legacy-reads run list
      fresh
      printf '[[job]]\n  name = "hello"\n  kind = "task"\n  cmd = "echo hello"\n' >.git/wtm/run.toml
      scenario 10-up-remembers-parallel run up main --parallel --remember exclusive --yes -d
      "$next" daemon stop --yes </dev/null >/dev/null 2>&1 || true
    fi
  )
  normalise "$dir"/*
}

# tty_side <side> <binary>: the wizard in tmux at 110×50, each step's screen, Enter on each (next to its end; legacy until its ports step, where Enter edits a port, then left with Esc).
tty_side() {
  local side=$1 bin=$2 session tty
  tty=$out/tty/$side
  rm -rf "$tty" && mkdir -p "$tty"
  (
    fresh
    session=wtmruninit-$side-$(basename "$root")
    trap 'tmux kill-session -t "$session" 2>/dev/null || true' EXIT
    cp "$bin" "$root/bin/wtmx"
    screen() { tmux capture-pane -p -t "$session" | cat -s; }
    tmux new-session -d -s "$session" -x 110 -y 50 -c "$root/acme" "zsh -f"
    tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear && wtmx run init; echo EXIT=\$?" Enter
    sleep 3
    local step=1 last=""
    while ((step <= 20)); do
      local now
      now=$(screen)
      [[ $now == "$last" ]] && break
      printf '%s\n' "$now" >"$tty/$(printf '%02d' "$step").txt"
      [[ $now == *EXIT=* ]] && break
      if [[ $side == legacy && $now == *"One port per service"* ]]; then
        tmux send-keys -t "$session" Escape
        sleep 1
        tmux send-keys -t "$session" C-c
        sleep 1
        screen >"$tty/$(printf '%02d' $((step + 1))).txt"
        break
      fi
      last=$now
      tmux send-keys -t "$session" Enter
      sleep 1.5
      step=$((step + 1))
    done
    if [[ $side == next ]]; then state >"$tty/state.txt"; fi
  )
  normalise "$tty"/*
}

trap 'rm -rf ${roots[@]+"${roots[@]}"} "$TMUX_TMPDIR"' EXIT
run_side legacy "$legacy"
run_side next "$next"
tty_side legacy "$legacy"
tty_side next "$next"
{
  for state in "$out"/legacy/*.state; do
    name=$(basename "$state" .state)
    echo "=== $name: exit legacy $(cat "$out/legacy/$name.exit"), next $(cat "$out/next/$name.exit")"
    diff -u --label "legacy/$name" --label "next/$name" "$state" "$out/next/$name.state" || true
  done
} >"$out/states.diff"
echo "$out/states.diff"

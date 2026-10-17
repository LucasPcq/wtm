#!/usr/bin/env bash
# Usage: tty.sh <side> <binary> <out dir>
# Drives the binary in tmux in the sandbox /tmp/wtmcmp-<first letter of side> (made by compare.sh):
# the form to its recap then applied, and Ctrl+C twice during a hook.
set -euo pipefail
side=$1 bin=$2 out=$3
root=/tmp/wtmcmp-${side:0:1}
mkdir -p "$out"
session=wtmcmp-$side
capture() { sleep "$2"; tmux capture-pane -p -t "$session" | cat -s >"$out/$1.txt"; }
cp "$bin" "$root/bin/wtmx"
start() {
  tmux kill-session -t "$session" 2>/dev/null || true
  tmux new-session -d -s "$session" -x 100 -y 40 -c "$root/acme"
  sleep 1
  tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
  sleep 0.5
  tmux send-keys -t "$session" "wtmx $*; echo EXIT=\$?" Enter
}

start create
capture form-1-branches 1.5
tmux send-keys -t "$session" "feat/form" Tab "topic" Tab
capture form-2-entries 0.8
tmux send-keys -t "$session" Enter
capture form-3-source 0.8
tmux send-keys -t "$session" Enter
capture form-4-env 0.8
tmux send-keys -t "$session" Enter
capture form-5-isolation 0.8
tmux send-keys -t "$session" Enter
capture form-6-recap 1.5
tmux send-keys -t "$session" Enter
capture form-7-outcome 3

start create feat/slow --yes
capture interrupt-1-hook 2
tmux send-keys -t "$session" C-c
capture interrupt-2-cancelling 0.5
capture interrupt-3-finished 3
start create feat/slow2 --yes
sleep 2
tmux send-keys -t "$session" C-c
sleep 0.3
tmux send-keys -t "$session" C-c
sleep 2
tmux send-keys -t "$session" 'echo EXIT=$?' Enter
capture interrupt-4-second 0.5
tmux kill-session -t "$session" || true
sed -i '' -E -e "s#(/private)?/tmp/wtmcmp-${side:0:1}#<SB>#g" -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' -e 's#wtm-next create#wtm create#g' "$out"/*

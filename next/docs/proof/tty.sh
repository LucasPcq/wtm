#!/usr/bin/env bash
# Usage: tty.sh <side> <binary> <out dir>, side legacy or next
# Drives the binary in tmux in the sandbox /tmp/wtmcmp-<first letter of side> (made by create.sh):
# the create form to its recap then applied (--ask, the sandbox remembering the env answer), Ctrl+C
# once then twice during a hook, and the extract form from its source to a new target, applied. Each screen is captured once it shows what it waits for and
# holds still, so a loaded machine slows the run without changing the screens.
set -euo pipefail
side=$1 bin=$2 out=$3
root=/tmp/wtmcmp-${side:0:1}
mkdir -p "$out"
session=wtmcmp-$side
screen() { tmux capture-pane -p -t "$session" | cat -s; }
# settle <text>: waits until the screen shows text and is the same twice in a row, else fails the run.
settle() {
  local now last="" tries=0
  while ((tries < 100)); do
    now=$(screen)
    [[ $now == *"$1"* && $now == "$last" ]] && return
    last=$now tries=$((tries + 1))
    sleep 0.3
  done
  echo "tty.sh: the screen never settled on \"$1\"" >&2
  return 1
}
capture() {
  settle "$2"
  screen >"$out/$1.txt"
}
cp "$bin" "$root/bin/wtmx"
start() {
  tmux kill-session -t "$session" 2>/dev/null || true
  tmux new-session -d -s "$session" -x 100 -y 40 -c "$root/acme" "zsh -f"
  tmux send-keys -t "$session" "export HOME=$root/home PATH=$root/bin:\$PATH PS1='$ ' && clear" Enter
  settle '$'
  tmux send-keys -t "$session" "wtmx $*; echo EXIT=\$?" Enter
}

start create --ask
capture form-1-branches 'Step 1'
tmux send-keys -t "$session" "feat/form" Tab "topic" Tab
capture form-2-entries 'topic'
tmux send-keys -t "$session" Enter
capture form-3-source 'Step 2'
tmux send-keys -t "$session" Enter
capture form-4-env 'Step 3'
tmux send-keys -t "$session" Enter
capture form-5-isolation 'Step 4'
tmux send-keys -t "$session" Enter
capture form-6-recap 'No, cancel'
tmux send-keys -t "$session" Enter
capture form-7-outcome 'EXIT=0'

# What each side shows while a hook runs.
running='› feat/slow'
[[ $side == legacy ]] && running='Hooks · On Create'
start create feat/slow --yes
capture interrupt-1-hook "$running"
sleep 1
tmux send-keys -t "$session" C-c
capture interrupt-2-cancelling '^C'
capture interrupt-3-finished 'EXIT=19'
start create feat/slow2 --yes
settle "$running"
sleep 1
tmux send-keys -t "$session" C-c
sleep 0.3
tmux send-keys -t "$session" C-c
settle '^C'
sleep 2
tmux send-keys -t "$session" 'echo EXIT=$?' Enter
capture interrupt-4-second 'EXIT=130'

echo "# acme — moved" >"$root/acme.trees/feat-form/README.md"
start extract
capture extract-1-source 'Step 1'
tmux send-keys -t "$session" Enter
capture extract-2-files 'Step 2'
tmux send-keys -t "$session" Space Enter
capture extract-3-target 'Step 3'
if [[ $side == legacy ]]; then
  # legacy lists the worktrees in git's order, which varies between runs
  awk '/────/{print; sorting=1; next} sorting && /^$/{close("sort"); sorting=0} sorting{print | "sort"; next} {print}' "$out/extract-3-target.txt" >"$out/sorted" && mv "$out/sorted" "$out/extract-3-target.txt"
fi
tmux send-keys -t "$session" Enter
capture extract-4-branch 'Step 4'
tmux send-keys -t "$session" "feat/moved" Enter
capture extract-5-from 'Step 5'
tmux send-keys -t "$session" Enter
capture extract-6-isolation 'Step 6'
tmux send-keys -t "$session" Enter
capture extract-7-mode 'Mode'
tmux send-keys -t "$session" Enter
capture extract-8-recap 'No, cancel'
tmux send-keys -t "$session" Enter
capture extract-9-outcome 'EXIT=0'
tmux kill-session -t "$session" || true
sed -i '' -E -e "s#(/private)?/tmp/wtmcmp-${side:0:1}#<SB>#g" -e 's#\(([0-9.]+m?s)\)#(<DUR>)#g' -e 's#wtm-next (create|extract)#wtm \1#g' "$out"/*

# Driving wtm with tmux

tmux gives you a real TTY you can type into and read back as text. wtm sees an interactive terminal, so wizards, pickers, `wtm ui` and `wtm run` views behave exactly as they do for a user.

## Session

```bash
name=wtm-<topic>                                   # unique: other sessions may run tmux too
tmux new-session -d -s "$name" -x 100 -y 30 "zsh -f"   # -f: no personal rc, no prompt noise
tmux send-keys -t "$name" "source $sb/env.sh; clear" Enter
```

100×30 keeps a frame readable in Markdown. Use `-x 80` to check a narrow terminal, a taller `-y` for long lists.

## Type, wait, read

```bash
tmux send-keys -t "$name" "wtm create feat/login" Enter
$S/tmux-wait.sh "$name" 'Source branch'            # waits for the text, prints the screen
tmux send-keys -t "$name" Down Enter               # keys: Enter Escape Tab BTab Up Down Left Right Space C-c, or plain letters
$S/tmux-wait.sh "$name" '❯ *$' 30                  # prompt back = command finished
```

Wait on text the screen must show, never on a fixed `sleep`: `tmux-wait.sh` polls `capture-pane` until the regex matches, prints the pane, and on timeout prints the last screen to stderr so you can see where it got stuck. Send literal text that could be read as a key name with `send-keys -l`.

Exit codes: `tmux send-keys -t "$name" 'echo "exit=$?"' Enter` after the command returns.

## Frames

```bash
tmux capture-pane -p -t "$name" > after.txt        # -p plain text; never -e (ANSI codes do not render in Markdown)
tmux capture-pane -p -S -200 -t "$name"            # include scrollback for long output
```

Keep the command line in the frame, drop blank trailing lines, keep only the lines that carry the point. For a before/after where one or two lines move, a `diff` of the two frames is tighter than two full blocks.

## Non-interactive commands

For plain output you do not need tmux at all: `(source $sb/env.sh && wtm tree --output json)` in a subshell. Use tmux when wtm must see a TTY or when keys have to be pressed.

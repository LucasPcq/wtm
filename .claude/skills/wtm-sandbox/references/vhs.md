# Recording wtm with VHS

VHS replays a `.tape` script in a headless terminal and renders it. Use it when the point is a sequence — a spinner, a step that now appears, a live refresh, a key that now does something. Check the behaviour with tmux first; a recording is for showing, not for finding out.

## Tape

Write tapes in the sandbox, never in `docs/demos/` (those are the README's).

```tape
Output after.gif
Set Shell "zsh"
Set FontSize 16
Set Width 1200
Set Height 640
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

Hide
Type "source /tmp/wtm-sandbox.XXXXXX/env.sh; clear"
Enter
Show

Type "wtm create feat/login"
Enter
Wait+Screen@15s /Source branch/
Sleep 1.5s
Enter
Wait+Screen@15s /Env strategy/
Sleep 2s
```

`vhs after.tape`, from the sandbox; for a before/after, the same tape twice with the other `env.sh` and `Output`. `Wait+Screen@<timeout> /regex/` waits on visible text rather than a fixed sleep; the `Sleep` after it is reading time for the viewer. `docs/demos/*.tape` shows the house style.

## Outputs

- `Output x.gif` — plays inline and loops everywhere GitHub renders Markdown. The default for short clips.
- `Output x.mp4` — smaller and sharper, rendered as a video player. Prefer it past ~10 s or when the GIF grows beyond a few MB. Several `Output` lines in one tape render all of them.
- `Screenshot x.png` inside the tape — one still frame, when one frame is enough but colour matters.

One idea per recording, under ~10 s.

## README GIFs are different

When a change alters what a README GIF shows, `make demos` re-records `docs/assets/*.gif` from `docs/demos/*.tape`, and those files are committed (CLAUDE.md, Docs & README). A tape that waits on text the change renamed must be updated too. `make demos` always rebuilds the fixed `/tmp/wtm-demo`, so it is not safe while another session records; if it cannot run now, update the tapes and say "README GIFs need `make demos`" rather than hand-patching copies of the demo scripts.

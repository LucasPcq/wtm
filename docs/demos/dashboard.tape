Output docs/assets/dashboard.gif
Source docs/demos/_env.tape

Hide
Type "source docs/demos/shellrc.sh"
Enter
Type "wtm create feat/login --yes >/dev/null 2>&1; wtm create feat/login-ui --from feat/login --yes >/dev/null 2>&1; wtm run up feat/login -d --yes >/dev/null 2>&1; clear"
Enter
Wait+Screen@60s /❯\s*$/
Show

Sleep 1s
Type "wtm ui"
Sleep 600ms
Enter
Sleep 3s
Down
Sleep 2s
Down
Sleep 2s
Type "n"
Wait+Screen@15s /Branch name/
Sleep 1.5s
Type "fix/typo"
Sleep 1s
Enter
Wait+Screen@15s /Source branch/
Sleep 2s
Enter
Wait+Screen@15s /Env strategy/
Sleep 1.5s
Enter
Wait+Screen@15s /Isolate it/
Sleep 2s
Enter
Wait+Screen@15s /Yes, create worktree/
Sleep 2.5s
Enter
Sleep 5s
Tab
Sleep 3.5s
Tab
Sleep 4s
Type "q"
Sleep 1s

Hide
Type "wtm run down --all --yes >/dev/null 2>&1; wtm run daemon stop --yes >/dev/null 2>&1"
Enter
Sleep 2s

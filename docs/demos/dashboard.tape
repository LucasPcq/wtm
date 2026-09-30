Output docs/assets/dashboard.gif
Source docs/demos/_env.tape
Set Height 800

Hide
Type "source docs/demos/shellrc.sh"
Enter
Type "wtm create feat/login --yes >/dev/null 2>&1; wtm create feat/login-ui --from feat/login --yes >/dev/null 2>&1; wtm create fix/typo --yes >/dev/null 2>&1; wtm run up feat/login -d --yes >/dev/null 2>&1; clear"
Enter
Wait+Screen@60s /❯\s*$/
Show

Type "wtm ui"
Sleep 300ms
Enter
Sleep 2.5s
Down
Sleep 1.2s
Down
Sleep 1.2s
Down
Sleep 1.2s
Up
Up
Sleep 1.5s
Tab
Sleep 2.5s
Tab
Sleep 3s
Tab
Sleep 1s
Type "q"
Sleep 1s

Hide
Type "wtm run down --all --yes >/dev/null 2>&1; wtm run daemon stop --yes >/dev/null 2>&1"
Enter
Sleep 2s

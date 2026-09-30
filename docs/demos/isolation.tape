Output docs/assets/isolation.gif
Source docs/demos/_env.tape

Hide
Type "source docs/demos/shellrc.sh"
Enter
Type "wtm create feat/login --yes >/dev/null 2>&1; wtm create fix/typo --yes >/dev/null 2>&1; clear"
Enter
Wait+Screen@60s /❯\s*$/
Show

Sleep 1s
Type "wtm run up"
Sleep 600ms
Enter
Wait+Screen@15s /enter confirm/
Sleep 2s
Down
Sleep 600ms
Space
Sleep 800ms
Down
Sleep 600ms
Space
Sleep 800ms
Up 2
Sleep 600ms
Space
Sleep 2s
Enter
Wait+Screen@30s /JOBS/
Sleep 5s
Down
Sleep 2.5s
Down
Sleep 2.5s
Down
Sleep 3s
Type "q"
Wait+Screen@15s /❯\s*$/
Sleep 4s

Type "wtm run ps"
Sleep 600ms
Enter
Sleep 5s

Hide
Type "wtm run down --all --yes >/dev/null 2>&1; wtm run daemon stop --yes >/dev/null 2>&1"
Enter
Sleep 2s

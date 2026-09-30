Output docs/assets/hero.gif
Source docs/demos/_env.tape

Hide
Type "source docs/demos/shellrc.sh"
Enter
Sleep 1s
Show

Sleep 1s
Type "wtm create feat/login"
Sleep 600ms
Enter
Wait+Screen@15s /Source branch/
Sleep 2s
Enter
Wait+Screen@15s /Env strategy/
Sleep 2s
Enter
Wait+Screen@15s /Isolate it/
Sleep 3.5s
Enter
Wait+Screen@15s /Review/
Sleep 3s
Enter
Wait+Screen@30s /❯\s*$/
Sleep 3.5s

Type "wtm go feat/login"
Sleep 600ms
Enter
Sleep 1.5s

Type "wtm run up"
Sleep 600ms
Enter
Wait+Screen@15s /enter confirm/
Sleep 2.5s
Enter
Wait+Screen@30s /JOBS/
Sleep 5s
Down
Sleep 4s
Type "q"
Wait+Screen@15s /❯\s*$/
Sleep 5s

Hide
Type "wtm run down --all --yes >/dev/null 2>&1; wtm run daemon stop --yes >/dev/null 2>&1"
Enter
Sleep 2s

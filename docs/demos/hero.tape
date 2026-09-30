Output docs/assets/hero.gif
Source docs/demos/_env.tape

Hide
Type "source docs/demos/shellrc.sh"
Enter
Sleep 1s
Show

Type "wtm create feat/login --yes"
Sleep 400ms
Enter
Wait+Screen@30s /❯\s*$/
Sleep 2s

Type "wtm go feat/login"
Sleep 400ms
Enter
Sleep 1s

Type "wtm run up -d"
Sleep 400ms
Enter
Wait+Screen@15s /enter confirm/
Sleep 1.5s
Enter
Wait+Screen@30s /❯\s*$/
Sleep 1.5s

Type "wtm run ps"
Sleep 400ms
Enter
Sleep 3s

Hide
Type "wtm run down --all --yes >/dev/null 2>&1; wtm run daemon stop --yes >/dev/null 2>&1"
Enter
Sleep 2s

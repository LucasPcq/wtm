Output docs/assets/isolation.gif
Source docs/demos/_env.tape

Hide
Type "source docs/demos/shellrc.sh"
Enter
Type "wtm run addressing names --yes >/dev/null 2>&1; wtm create feat/login --yes >/dev/null 2>&1; wtm create fix/typo --yes >/dev/null 2>&1; clear"
Enter
Wait+Screen@60s /❯\s*$/
Show

Type "wtm run up feat/login fix/typo -d --yes"
Sleep 400ms
Enter
Wait+Screen@60s /❯\s*$/
Sleep 3s

Type "wtm run ps"
Sleep 400ms
Enter
Sleep 4s

Hide
Type "wtm run down --all --yes >/dev/null 2>&1; wtm run daemon stop --yes >/dev/null 2>&1"
Enter
Sleep 2s

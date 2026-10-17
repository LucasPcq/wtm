Source docs/demos/_env.tape
Set Height 280

Hide
Type "source docs/demos/shellrc.sh"
Enter
Sleep 1s
Show

Type "wtm create feat/login --yes"
Enter
Wait+Screen@30s /wtm go feat\/login/
Sleep 500ms
Screenshot docs/assets/create.png
Sleep 1s

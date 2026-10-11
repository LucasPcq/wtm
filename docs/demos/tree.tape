Source docs/demos/_env.tape
Set Height 220

Hide
Type "source docs/demos/shellrc.sh"
Enter
Sleep 1s
Type "wtm create feat/login --yes && wtm create feat/login-ui --from feat/login --yes && wtm create fix/header --yes && git commit -q --allow-empty -m 'chore: move main' && clear"
Enter
Wait+Screen@60s /❯\s*$/
Show

Type "wtm tree"
Enter
Wait+Screen@15s /fix\/header/
Sleep 500ms
Screenshot docs/assets/tree.png
Sleep 1s

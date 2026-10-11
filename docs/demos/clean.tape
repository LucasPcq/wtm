Source docs/demos/_env.tape
Set Height 200

Hide
Type "source docs/demos/shellrc.sh"
Enter
Sleep 1s
Type "wtm create feat/login --yes && wtm create fix/header --yes && echo wip > ../acme.trees/fix-header/wip.txt && clear"
Enter
Wait+Screen@60s /❯\s*$/
Show

Type "wtm clean feat/login fix/header --yes"
Enter
Wait+Screen@30s /--force/
Sleep 500ms
Screenshot docs/assets/clean.png
Sleep 1s

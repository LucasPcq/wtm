Output form.gif
Set Shell "zsh"
Set FontSize 16
Set Width 1200
Set Height 760
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

Hide
Type "source $WTM_PROOF_SANDBOX/env.sh; clear"
Enter
Show

Type "wtm-next create"
Enter
Wait+Screen@15s /Branches/
Sleep 1s
Type "feat/demo"
Tab
Type "topic2"
Tab
Sleep 1s
Enter
Wait+Screen@15s /Source branch/
Sleep 1.5s
Enter
Wait+Screen@15s /Isolation/
Sleep 1s
Down
Tab
Sleep 1s
Enter
Wait+Screen@15s /Review/
Sleep 1s
Screenshot form-recap.png
Sleep 1.5s
Enter
Wait+Screen@20s /create: 2 done/
Sleep 3s

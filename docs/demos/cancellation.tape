Output docs/assets/cancellation.gif
Output docs/assets/cancellation.mp4
Source docs/demos/_env.tape

Hide
Type "bash docs/demos/cancellation.sh && source docs/demos/shellrc.sh"
Enter
Wait+Screen@120s /❯\s*$/
Show

# A batch: the worktree under way goes all the way, the rest is named.
Sleep 1s
Type "wtm clean feat/billing feat/search feat/export --yes"
Sleep 600ms
Enter
Wait+Screen@30s /feat\/search \(2\/3\)/
Sleep 1.2s
Ctrl+C
Wait+Screen@30s /❯\s*$/
Sleep 2s
Type "echo $?"
Enter
Sleep 3.5s
Type "clear"
Enter

# A rebase: aborted, the branches where they were, nothing pushed.
Sleep 800ms
Type "wtm sync stack/api stack/ui --yes"
Sleep 600ms
Enter
Wait+Screen@30s /Rebasing/
Sleep 1.5s
Ctrl+C
Wait+Screen@30s /❯\s*$/
Sleep 2s
Type "echo $? && git -C ../acme.trees/stack-api status --short --branch"
Enter
Sleep 4s
Type "clear"
Enter

# A creation: whole or not at all, and the reader is told which.
Sleep 800ms
Type "wtm create feat/login --yes"
Sleep 600ms
Enter
Wait+Screen@30s /Creating worktree/
Sleep 1s
Ctrl+C
Wait+Screen@30s /❯\s*$/
Sleep 2s
Type "echo $?"
Enter
Sleep 3.5s
Type "clear"
Enter

# A picker: Ctrl+C backs out, like Esc.
Sleep 800ms
Type "wtm clean"
Sleep 600ms
Enter
Wait+Screen@30s /feat\/login/
Sleep 1.5s
Ctrl+C
Wait+Screen@15s /❯\s*$/
Sleep 1.5s
Type "echo $?"
Enter
Sleep 3s
Type "clear"
Enter

# A start: what is already running keeps running, and the recap says so.
Sleep 800ms
Type "wtm run up --yes"
Sleep 600ms
Enter
Wait+Screen@30s /migrate/
Sleep 2s
Ctrl+C
Wait+Screen@30s /❯\s*$/
Sleep 2s
Type "echo $?"
Enter
Sleep 2s
Type "wtm run ps"
Enter
Sleep 5s

Hide
Type "wtm run down --all --yes >/dev/null 2>&1; wtm run daemon stop --yes >/dev/null 2>&1"
Enter
Sleep 2s

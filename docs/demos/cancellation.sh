#!/usr/bin/env bash
# Sets the scenes cancellation.tape interrupts, over the project setup.sh built:
# worktrees to clean, a stack to sync, a slow checkout and a slow task.
set -euo pipefail
export HOME=/tmp/wtm-demo/home PATH=/tmp/wtm-demo/bin:$PATH
cd /tmp/wtm-demo/acme

# clean: three worktrees, each with an on_clean hook long enough to Ctrl+C.
for name in feat/billing feat/search feat/export; do
  wtm create "$name" --yes >/dev/null 2>&1
done
python3 - .git/wtm/config.toml <<'PY'
import sys
path = sys.argv[1]
text = open(path).read().replace("on_clean = [\n]", 'on_clean = [\n  "sleep 3",\n]')
open(path, "w").write(text)
PY

# sync: a two-branch stack behind a main that moved.
wtm create stack/api --yes >/dev/null 2>&1
git -C ../acme.trees/stack-api commit -q --allow-empty -m "feat: api v2"
wtm create stack/ui --from stack/api --yes >/dev/null 2>&1
git -C ../acme.trees/stack-ui commit -q --allow-empty -m "feat: ui for api v2"
git commit -q --allow-empty -m "chore: main moved"

# Every checkout from here on is slow: a rebase, a new worktree.
printf '#!/bin/sh\nsleep 4\n' > .git/hooks/post-checkout
chmod +x .git/hooks/post-checkout

# run up: a migration between the API and the web app.
python3 - .git/wtm/run.toml <<'PY'
import sys
path = sys.argv[1]
text = open(path).read()
text = text.replace('''[[profile]]
  name = "dev"
  jobs = ["api", "web"]''', '''[[job]]
  name = "migrate"
  kind = "task"
  cmd = "sleep 6"

[[profile]]
  name = "dev"
  jobs = ["api", "migrate", "web"]''')
open(path, "w").write(text)
PY

#!/usr/bin/env bash
# Usage: spawn-worker.sh <branch> <source-branch> <agent-name> <brief-file>
#
# Creates the worktree with wtm, waits for herdr-wtm to open its herdr
# workspace, starts Claude in that workspace's shell pane and submits the
# brief. Prints one JSON line: {agent, pane, workspace, worktree, status}.
#
# Env: WTM (wtm binary, default wtm), WORKSPACE_TIMEOUT (seconds, default 30),
#      AGENT_ARGS (Claude's own arguments, default "--permission-mode auto").
set -euo pipefail

branch=${1:?branch to create}
source_branch=${2:?branch to create it from}
name=${3:?"herdr agent name, [a-z][a-z0-9_-]{0,31}"}
brief=${4:?brief file}

[[ "${HERDR_ENV:-}" == 1 ]] || { echo "not inside a herdr pane (HERDR_ENV != 1)" >&2; exit 1; }
[[ -s "$brief" ]] || { echo "brief file $brief is missing or empty" >&2; exit 1; }
[[ "$name" =~ ^[a-z][a-z0-9_-]{0,31}$ ]] || { echo "invalid agent name: $name" >&2; exit 1; }

wtm_bin=${WTM:-wtm}
read -r -a agent_args <<<"${AGENT_ARGS:---permission-mode auto}"

created=$("$wtm_bin" create "$branch" --from "$source_branch" --yes --output json)
worktree=$(jq -er '.results[0].path' <<<"$created") || {
  echo "wtm create reported no worktree:" >&2
  printf '%s\n' "$created" >&2
  exit 1
}

# herdr-wtm opens the workspace from `wtm events`; creating one here would
# give the worktree a duplicate the plugin never closes.
deadline=$((SECONDS + ${WORKSPACE_TIMEOUT:-30}))
workspace=
while ((SECONDS < deadline)); do
  workspace=$(herdr workspace list | jq -r --arg p "$worktree" \
    'first(.result.workspaces[] | select(.worktree.checkout_path == $p) | .workspace_id) // empty')
  [[ -n "$workspace" ]] && break
  sleep 0.5
done
[[ -n "$workspace" ]] || {
  echo "no herdr workspace opened for $worktree within ${WORKSPACE_TIMEOUT:-30}s — is the herdr-wtm plugin running?" >&2
  echo "the worktree is kept: open it with \`herdr worktree open --cwd $PWD --path $worktree --no-focus\` and re-run from agent start, or remove it with \`wtm clean $branch --yes\`" >&2
  exit 1
}

pane=$(herdr pane list --workspace "$workspace" | jq -er 'first(.result.panes[] | select(.agent == null) | .pane_id)') || {
  echo "workspace $workspace has no free shell pane" >&2
  exit 1
}

emit() {
  jq -cn --arg agent "$name" --arg pane "$pane" --arg workspace "$workspace" --arg worktree "$worktree" \
    --arg status "$(herdr agent get "$name" | jq -r '.result.agent.agent_status')" --arg brief_sent "$1" \
    '{agent: $agent, pane: $pane, workspace: $workspace, worktree: $worktree, status: $status, brief_sent: ($brief_sent == "true")}'
}

# A new worktree is a folder Claude has never seen: its trust dialog blocks
# the start, and the brief must wait until someone answers it.
herdr agent start "$name" --kind claude --pane "$pane" --timeout 60000 -- "${agent_args[@]}" >/dev/null || {
  echo "claude is not ready in $pane (folder trust dialog?): brief NOT sent — read it with \`herdr agent read $name --source visible\`" >&2
  emit false
  exit 1
}
herdr agent prompt "$name" "$(cat "$brief")" --wait --until working --until blocked --timeout 30000 >/dev/null || true
emit true

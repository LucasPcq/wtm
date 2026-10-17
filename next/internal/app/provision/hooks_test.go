package provision_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/env"

	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnCreateRunsInTheNewWorktreeAndKeepsItsLog(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}

	outcome := provision.Run(t.Context(), hooked(w.params(report), "pwd", "echo from {{from_branch}} on {{branch}}"))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{w.path(branchName), "from main on feat/x"}, report.lines())
	assert.Equal(t, env.HookLogPath(env.HookLogPathParams{StateDir: w.stateDir, Branch: branchName}), report.logOf("on_create"))
	assert.Contains(t, report.all(), wtm.Output{Subject: branchName, Line: "from main on feat/x"})
}

func TestAHookThatFailsLeavesTheWorktreeIncomplete(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}

	outcome := provision.Run(t.Context(), hooked(w.params(report), "exit 3"))

	assert.Equal(t, wtm.StatusFailed, outcome.Status)
	assert.Equal(t, wtm.ReasonIncomplete, outcome.Reason)
	assert.Equal(t, "unit.phase_failed", outcome.Err.Code)
	assert.Equal(t, wtm.Params{"phase": "on_create"}, outcome.Err.Params)
	assert.Equal(t, "on_create failed: hook failed: exit status 3", outcome.Err.Error())
	assert.DirExists(t, w.path(branchName))
	provisioned, ok := report.worktreeEvents()[1].(wtm.WorktreeProvisioned)
	require.True(t, ok)
	assert.False(t, provisioned.OK)
	assert.Equal(t, "exit 3", provisioned.Hook)
	assert.Equal(t, new(3), provisioned.ExitCode)
}

func TestAHookInterruptedLeavesTheWorktreeCancelledAndIncomplete(t *testing.T) {
	w := onDisk(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	report := &recorder{on: func(event wtm.Event) {
		if output, ok := event.(wtm.Output); ok && output.Line == "started" {
			cancel()
		}
	}}

	outcome := provision.Run(ctx, hooked(w.params(report), "echo started; sleep 5", "echo never"))

	assert.Equal(t, wtm.StatusCancelled, outcome.Status)
	assert.Equal(t, wtm.ReasonIncomplete, outcome.Reason)
	assert.Equal(t, wtm.CodeInterrupted, outcome.Err.Code)
	assert.Equal(t, wtm.Params{"phase": "on_create"}, outcome.Err.Params)
	assert.Equal(t, []string{"started"}, report.lines())
	assert.DirExists(t, w.path(branchName))
	provisioned, ok := report.worktreeEvents()[1].(wtm.WorktreeProvisioned)
	require.True(t, ok)
	assert.False(t, provisioned.OK)
}

func TestAComposeEnvThatCannotBeReadIsSaidAndTheHooksRunWithoutIt(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}
	params := hooked(composed(w.params(report)), "echo project=$COMPOSE_PROJECT_NAME")
	params.Deps.EnvFiles = unreadable{EnvFiles: params.Deps.EnvFiles}

	outcome := provision.Run(t.Context(), params)

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"project="}, report.lines())
	assert.Equal(t, []wtm.Warning{{
		Code:    "provision.hook_env_unavailable",
		Message: "read .env: permission denied — on_create hooks run without the worktree's environment",
		Params:  wtm.Params{"cause": "read .env: permission denied"},
	}}, outcome.Warnings)
}

func TestAHookReadsTheRunEnvironmentOnlyWhenTheRunReachesHooks(t *testing.T) {
	t.Setenv("WTM_PORT_OFFSET", "90")
	echo := "echo offset=$WTM_PORT_OFFSET project=$COMPOSE_PROJECT_NAME"
	plain, composedReport := &recorder{}, &recorder{}

	provision.Run(t.Context(), hooked(onDisk(t).params(plain), echo))
	provision.Run(t.Context(), hooked(composed(onDisk(t).params(composedReport)), echo))

	assert.Equal(t, []string{"worktree", "env", "meta", "on_create"}, plain.phases())
	assert.Equal(t, []string{"worktree", "env", "meta", "ordinal", "ports", "on_create"}, composedReport.phases())
	assert.Equal(t, []string{"offset=90 project="}, plain.lines())
	require.Len(t, composedReport.lines(), 1)
	assert.True(t, strings.HasPrefix(composedReport.lines()[0], "offset=10 project=main-feat-x"), composedReport.lines())
}

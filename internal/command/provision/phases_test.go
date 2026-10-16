package provision_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/command/provision"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

func TestAPortLinkedWorktreeIsNumberedAndItsEnvMovedOntoItsPorts(t *testing.T) {
	w := onDisk(t)

	item := run(t.Context(), runParams{Unit: provision.Unit(portLinked(w.params()))})

	require.Equal(t, kernel.StatusDone, item.Status, item.Error)
	require.NotNil(t, item.Detail.Ordinal)
	assert.Equal(t, 1, *item.Detail.Ordinal)
	assert.True(t, item.Detail.Allocated)
	assert.True(t, item.Detail.EnvPorts.Applied)
	assert.NotContains(t, readFile(t, filepath.Join(w.path(branch), ".env")), "PORT=3000")
	recorded, err := w.meta.Load(branch)
	require.NoError(t, err)
	assert.Equal(t, 1, recorded.Ordinal)
}

func TestARunConfigThatDoesNotLoadLeavesTheEnvAsCopiedAndSaysWhy(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params())
	params.Project.RunErr = errors.New("run.toml: line 3")

	item := run(t.Context(), runParams{Unit: provision.Unit(params)})

	require.Equal(t, kernel.StatusDone, item.Status, item.Error)
	assert.Equal(t, []kernel.Warning{{Code: provision.CodePortsNotSettledRun, Params: kernel.Params{provision.ParamBranch: branch, provision.ParamCause: "run.toml: line 3"}}}, item.Warnings)
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branch), ".env")))
}

func TestALinkToAnEnvConfigTomlDoesNotDeclareIsNamedAndLeftOut(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params())
	params.Project.Run.EnvPorts = append(params.Project.Run.EnvPorts, domain.EnvPortLink{File: "web/.env", Key: "WEB_PORT", Job: "web", Port: "http"})

	item := run(t.Context(), runParams{Unit: provision.Unit(params)})

	require.Equal(t, kernel.StatusDone, item.Status, item.Error)
	assert.Equal(t, []kernel.Warning{{Code: provision.CodeEnvPortIgnored, Params: kernel.Params{kernel.ParamKey: "WEB_PORT", provision.ParamFile: "web/.env"}}}, item.Warnings)
}

func TestAPortPassThatFailsNeverFailsTheWorktree(t *testing.T) {
	w, _ := inMemory(t, seeds{Store: storetest.FakeParams{Fail: map[string]error{"Edit": errors.New("read-only file system")}}})
	params := portLinked(w.params())

	item := run(t.Context(), runParams{Unit: provision.Unit(params)})

	require.Equal(t, kernel.StatusDone, item.Status, item.Error)
	assert.Equal(t, []kernel.Code{provision.CodePortsNotSettled}, codes(item.Warnings))
	assert.NotNil(t, item.Detail.Ordinal)
	assert.Empty(t, item.Detail.EnvPorts)
}

func TestANumberThatCannotBeGivenLeavesThePortsAsCopiedAndSaysWhy(t *testing.T) {
	w, _ := inMemory(t, seeds{GitFail: map[string]error{"Worktrees": errors.New("git worktree list: exit status 128")}})

	item := run(t.Context(), runParams{Unit: provision.Unit(portLinked(w.params()))})

	require.Equal(t, kernel.StatusDone, item.Status, item.Error)
	assert.Nil(t, item.Detail.Ordinal)
	assert.Equal(t, []kernel.Code{provision.CodePortsNotSettled}, codes(item.Warnings))
}

func TestOnCreateRunsInTheNewWorktreeAndKeepsItsLog(t *testing.T) {
	w := onDisk(t)
	recorder := &recorder{}

	item := run(t.Context(), runParams{Unit: provision.Unit(hooked(w.params(), "pwd", "echo from {{from_branch}}")), Recorder: recorder})

	require.Equal(t, kernel.StatusDone, item.Status, item.Error)
	assert.Equal(t, []string{w.path(branch), "from main"}, recorder.lines())
	assert.Equal(t, rules.HooksLogPath(rules.HooksLogPathParams{StateDir: w.stateDir, Phase: domain.HookOnCreate, Branch: branch}), recorder.logOf(string(domain.HookOnCreate)))
}

func TestAHookThatFailsLeavesTheWorktreeIncomplete(t *testing.T) {
	w := onDisk(t)

	item := run(t.Context(), runParams{Unit: provision.Unit(hooked(w.params(), "exit 3"))})

	assert.Equal(t, kernel.StatusFailed, item.Status)
	assert.Equal(t, kernel.ReasonIncomplete, item.Reason)
	assert.Equal(t, w.path(branch), item.Detail.Path)
	assert.DirExists(t, w.path(branch))
}

func TestAHookInterruptedLeavesTheWorktreeCancelledAndIncomplete(t *testing.T) {
	w := onDisk(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	recorder := &recorder{onLine: func(line string) {
		if line == "started" {
			cancel()
		}
	}}

	item := run(ctx, runParams{Unit: provision.Unit(hooked(w.params(), "echo started; sleep 5")), Recorder: recorder})

	assert.Equal(t, kernel.StatusCancelled, item.Status)
	assert.Equal(t, kernel.ReasonIncomplete, item.Reason)
	assert.DirExists(t, w.path(branch))
}

func TestAHookReadsTheRunEnvironmentOnlyWhenTheRunReachesHooks(t *testing.T) {
	w := onDisk(t)
	echo := "echo offset=$" + domain.EnvPortOffset
	plain, composedRecorder := &recorder{}, &recorder{}

	run(t.Context(), runParams{Unit: provision.Unit(hooked(w.params(), echo)), Recorder: plain})
	other := onDisk(t)
	run(t.Context(), runParams{Unit: provision.Unit(hooked(composed(other.params()), echo)), Recorder: composedRecorder})

	assert.Equal(t, []string{"offset="}, plain.lines())
	require.Len(t, composedRecorder.lines(), 1)
	assert.False(t, strings.HasSuffix(composedRecorder.lines()[0], "="), composedRecorder.lines())
}

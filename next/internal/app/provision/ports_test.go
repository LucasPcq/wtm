package provision_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPortLinkedWorktreeIsNumberedAndItsEnvMovedOntoItsPorts(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}

	outcome := provision.Run(t.Context(), portLinked(w.params(report)))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"worktree", "env", "meta", "ordinal", "ports"}, report.phases())
	require.NotNil(t, outcome.Provisioned.Ordinal)
	assert.Equal(t, 1, *outcome.Provisioned.Ordinal)
	assert.True(t, outcome.Provisioned.EnvPorts.Applied)
	require.NotEmpty(t, outcome.Provisioned.EnvPorts.Entries)
	for _, entry := range outcome.Provisioned.EnvPorts.Entries {
		assert.Empty(t, entry.CurrentValue+entry.NewValue+entry.ForeignHost, "a value of the .env left the worktree")
	}
	assert.Equal(t, "PORT=3010\n", readFile(t, filepath.Join(w.path(branchName), ".env")))
	recorded, err := w.meta.Load(branchName)
	require.NoError(t, err)
	assert.Equal(t, 1, recorded.Ordinal)
}

func TestARunConfigThatDoesNotLoadLeavesTheEnvAsCopiedAndSaysWhy(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}
	params := portLinked(w.params(report))
	params.Project.RunErr = errors.New("run.toml: line 3")

	outcome := provision.Run(t.Context(), params)

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"worktree", "env", "meta", "ports"}, report.phases())
	assert.Equal(t, []wtm.Warning{{
		Code:    "provision.ports_not_settled_run",
		Message: "run.toml: line 3 — ports not settled — run `wtm env feat/x` once run.toml is fixed",
		Params:  wtm.Params{"branch": branchName, "cause": "run.toml: line 3"},
	}}, outcome.Warnings)
	assert.Nil(t, outcome.Provisioned.Ordinal)
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branchName), ".env")))
}

func TestAVerbatimWorktreeKeepsItsEnvAsCopiedAndHasNoNumber(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}
	params := portLinked(w.params(report))
	params.Worktree.Isolation = project.Verbatim

	outcome := provision.Run(t.Context(), params)

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"worktree", "env", "meta"}, report.phases())
	assert.Nil(t, outcome.Provisioned.Ordinal)
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branchName), ".env")))
}

func TestAnEnvValueThatCannotBeExpandedIsSaidAsARunTomlToFix(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	params.Project.Run.EnvValues = []project.EnvValueLink{{File: ".env", Key: "API", Job: "ghost", Value: "x"}}

	outcome := provision.Run(t.Context(), params)

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"provision.ports_not_settled_run"}, codes(outcome.Warnings))
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branchName), ".env")))
}

func TestAProxyPortOutOfRangeIsSaidAsAConfigToFix(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	params.Project.Global.Proxy.Port = 70000

	outcome := provision.Run(t.Context(), params)

	assert.Equal(t, []string{"provision.ports_not_settled"}, codes(outcome.Warnings))
}

func TestALinkToAnEnvConfigTomlDoesNotDeclareIsNamedAndLeftOut(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	params.Project.Run.EnvPorts = append(params.Project.Run.EnvPorts, project.EnvPortLink{File: "web/.env", Key: "WEB_PORT", Job: "web", Port: "http"})
	params.Project.Run.EnvValues = []project.EnvValueLink{{File: "api/.env", Key: "API", Job: "web", Value: "x"}}

	outcome := provision.Run(t.Context(), params)

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []wtm.Warning{
		{
			Code:    "provision.env_port_ignored",
			Message: "env_port WEB_PORT in web/.env ignored: not a configured env file — add it to [env] in config.toml or drop the link",
			Params:  wtm.Params{"key": "WEB_PORT", "file": "web/.env"},
		},
		{
			Code:    "provision.env_value_ignored",
			Message: "env API in api/.env ignored: not a configured env file — add it to [env] in config.toml or drop the link",
			Params:  wtm.Params{"key": "API", "file": "api/.env"},
		},
	}, outcome.Warnings)
	assert.Equal(t, "PORT=3010\n", readFile(t, filepath.Join(w.path(branchName), ".env")))
}

func TestAnOriginWithoutTheRunProxyIsAProxyToFixNotARunToml(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	params.Project.Run.Addressing = project.AddressNames
	params.Project.Run.Jobs[0].URL = &project.URL{Port: "http"}
	params.Project.Run.EnvValues = []project.EnvValueLink{{File: ".env", Key: "API", Job: "web", Value: "{origin}"}}

	outcome := provision.Run(t.Context(), params)

	require.Equal(t, []string{"provision.ports_not_settled"}, codes(outcome.Warnings))
	assert.Equal(t, "env API in .env: {origin} of job \"web\" is served by the run proxy, which serves no port: enable the proxy, or write the host yourself — ports not settled — run `wtm env feat/x` once this is fixed", outcome.Warnings[0].Message)
}

func TestAPortPassThatFailsNeverFailsTheWorktree(t *testing.T) {
	w, _ := inMemory(t, seeds{Store: storetest.FakeParams{Fail: map[string]error{"Edit": errors.New("read-only file system")}}})

	outcome := provision.Run(t.Context(), portLinked(w.params(&recorder{})))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"provision.ports_not_settled"}, codes(outcome.Warnings))
	assert.NotNil(t, outcome.Provisioned.Ordinal)
	assert.Empty(t, outcome.Provisioned.EnvPorts)
}

func TestAComposeStackGetsItsProjectNameWithoutAnyPortLink(t *testing.T) {
	w := onDisk(t)

	outcome := provision.Run(t.Context(), composed(w.params(&recorder{})))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, mainEnv+"COMPOSE_PROJECT_NAME=main-feat-x\n", readFile(t, filepath.Join(w.path(branchName), ".env")))
}

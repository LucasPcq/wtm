package provision_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWorktreeGetsTheSmallestOrdinalNoLiveWorktreeHolds(t *testing.T) {
	w, files := inMemory(t, seeds{
		Worktrees: []worktree.Worktree{
			{Path: "/trees/a", State: worktree.OnBranch{Branch: "feat/a"}},
			{Path: "/trees/b", State: worktree.Rebasing{Branch: "feat/b"}},
			{Path: "/trees/c", State: worktree.Detached{}},
			{Path: "/trees/d", State: worktree.OnBranch{Branch: "feat/d"}},
		},
		Store: storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/a": {Ordinal: 1}, "feat/b": {Ordinal: 3}}},
	})

	outcome := w.service.Run(t.Context(), portLinked(w.params(&recorder{})))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, 2, outcome.Provisioned.Meta.Ordinal)
	recorded, _ := files.Meta(branchName)
	assert.Equal(t, 2, recorded.Ordinal)
	assert.Equal(t, "main", recorded.SourceBranch)
}

func TestAnExistingWorktreeIsNumberedPastItsNeighboursAndKeepsItsRecord(t *testing.T) {
	w, files := inMemory(t, seeds{
		Worktrees: []worktree.Worktree{
			{Path: "/trees/a", State: worktree.OnBranch{Branch: "feat/a"}},
			{Path: "/trees/b", State: worktree.OnBranch{Branch: "feat/b"}},
		},
		Store: storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/a": {Ordinal: 1}, "feat/b": {SourceBranch: "main"}}},
	})

	ordinal, err := w.service.Number(t.Context(), "feat/b")

	require.NoError(t, err)
	assert.Equal(t, 2, ordinal)
	recorded, _ := files.Meta("feat/b")
	assert.Equal(t, worktree.Meta{SourceBranch: "main", Ordinal: 2}, recorded)
}

func TestAWorktreeWithNoRecordIsNumberedAndRecorded(t *testing.T) {
	w, files := inMemory(t, seeds{Worktrees: []worktree.Worktree{{Path: "/trees/b", State: worktree.OnBranch{Branch: "feat/b"}}}})

	ordinal, err := w.service.Number(t.Context(), "feat/b")

	require.NoError(t, err)
	assert.Equal(t, 1, ordinal)
	recorded, _ := files.Meta("feat/b")
	assert.Equal(t, 1, recorded.Ordinal)
}

func TestTheProvisionedRecordCarriesItsOrdinal(t *testing.T) {
	w, files := inMemory(t, seeds{Store: storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/a": {Ordinal: 1}}}, Worktrees: []worktree.Worktree{{Path: "/trees/a", State: worktree.OnBranch{Branch: "feat/a"}}}})

	outcome := w.service.Run(t.Context(), portLinked(w.params(&recorder{})))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	recorded, _ := files.Meta(branchName)
	assert.Equal(t, 2, recorded.Ordinal)
	assert.Equal(t, recorded, outcome.Provisioned.Meta)
}

func TestTheOrdinalOfADetachedWorktreeIsNeverGivenAgain(t *testing.T) {
	w, _ := inMemory(t, seeds{
		Worktrees: []worktree.Worktree{{Path: "/trees/a", State: worktree.Detached{}}},
		Store:     storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/a": {Ordinal: 1}}},
	})

	outcome := w.service.Run(t.Context(), portLinked(w.params(&recorder{})))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, 2, outcome.Provisioned.Meta.Ordinal)
}

func TestANumberIsGivenUnderTheOrdinalLock(t *testing.T) {
	w, files := inMemory(t, seeds{})

	w.service.Run(t.Context(), portLinked(w.params(&recorder{})))

	calls := files.Calls()
	locked := slices.Index(calls, "LockOrdinals ")
	require.Positive(t, locked)
	assert.Equal(t, "Update "+branchName, calls[locked+1])
}

func TestANumberIsNeverGivenPastANeighbourWhoseRecordDoesNotRead(t *testing.T) {
	w, files := inMemory(t, seeds{
		Worktrees: []worktree.Worktree{{Path: "/trees/a", State: worktree.OnBranch{Branch: "feat/a"}}},
		Store:     storetest.FakeParams{Unreadable: []string{"feat/a"}},
	})

	outcome := w.service.Run(t.Context(), portLinked(w.params(&recorder{})))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Zero(t, outcome.Provisioned.Meta.Ordinal)
	recorded, _ := files.Meta(branchName)
	assert.Zero(t, recorded.Ordinal)
	require.Equal(t, []string{"provision.ports_not_settled"}, codes(outcome.Warnings))
	assert.Equal(t, "cannot read a live worktree's ordinal: feat/a: storetest: the record of feat/a does not read — ports not settled", outcome.Warnings[0].Message)
	assert.Equal(t, settled("what kept them"), outcome.Warnings[0].FollowUp)
}

func TestANumberIsNeverGivenPastANeighbourANewerWtmRecorded(t *testing.T) {
	w, files := inMemory(t, seeds{
		Worktrees: []worktree.Worktree{{Path: "/trees/a", State: worktree.OnBranch{Branch: "feat/a"}}},
		Store:     storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/a": {Schema: 99, Ordinal: 1}}},
	})

	outcome := w.service.Run(t.Context(), portLinked(w.params(&recorder{})))

	assert.Equal(t, domain.StatusFailed, outcome.Status)
	assert.Equal(t, domain.ReasonIncomplete, outcome.Reason)
	var newer *domain.SchemaNewer
	require.ErrorAs(t, outcome.Err, &newer)
	assert.Zero(t, outcome.Provisioned.Meta.Ordinal)
	recorded, _ := files.Meta(branchName)
	assert.Zero(t, recorded.Ordinal)
}

func TestANumberThatCannotBeGivenLeavesThePortsAsCopiedAndSaysWhy(t *testing.T) {
	w, _ := inMemory(t, seeds{GitFail: map[string]error{"Worktrees": errors.New("git worktree list: exit status 128")}})
	report := &recorder{}

	outcome := w.service.Run(t.Context(), portLinked(w.params(report)))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Zero(t, outcome.Provisioned.Meta.Ordinal)
	assert.Equal(t, []domain.Warning{{
		Code:     "provision.ports_not_settled",
		Message:  "list worktrees: git worktree list: exit status 128 — ports not settled",
		Params:   domain.Params{"branch": branchName, "cause": "list worktrees: git worktree list: exit status 128"},
		FollowUp: settled("what kept them"),
	}}, outcome.Warnings)
	assert.Len(t, report.worktreeEvents(), 2)
}

func TestANumberOnlyHooksNeededStillGivesTheComposeNameAndSaysThePortsAreMissing(t *testing.T) {
	t.Setenv("COMPOSE_PROJECT_NAME", "app-a")
	t.Setenv("WTM_PORT_OFFSET", "90")
	w, files := inMemory(t, seeds{GitFail: map[string]error{"Worktrees": errors.New("exit status 128")}})
	require.NoError(t, os.MkdirAll(w.path(branchName), 0o755))
	report := &recorder{}

	outcome := w.service.Run(t.Context(), hooked(composed(w.params(report)), "echo offset=$WTM_PORT_OFFSET project=$COMPOSE_PROJECT_NAME"))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Zero(t, outcome.Provisioned.Meta.Ordinal)
	assert.Equal(t, []string{"provision.hook_env_unnumbered"}, codes(outcome.Warnings))
	assert.Equal(t, []string{"offset= project=main-feat-x"}, report.lines())
	written, _ := files.File(filepath.Join(w.path(branchName), ".env"))
	assert.Equal(t, mainEnv+"COMPOSE_PROJECT_NAME=main-feat-x\n", written)
}

func TestAHookNeverTakesAPortOfRunTomlFromTheCallersEnvironment(t *testing.T) {
	t.Setenv("pg", "9999")
	w, _ := inMemory(t, seeds{GitFail: map[string]error{"Worktrees": errors.New("exit status 128")}})
	require.NoError(t, os.MkdirAll(w.path(branchName), 0o755))
	report := &recorder{}

	outcome := w.service.Run(t.Context(), hooked(composed(w.params(report)), "echo pg=${pg-unset}"))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"pg=unset"}, report.lines())
}

func TestAComposeStackWhosePortsCannotBeSettledStillGetsItsProjectName(t *testing.T) {
	for name, seed := range map[string]seeds{
		"no_ordinal":       {GitFail: map[string]error{"Worktrees": errors.New("exit status 128")}},
		"value_unresolved": {},
	} {
		t.Run(name, func(t *testing.T) {
			w, files := inMemory(t, seed)
			params := composed(w.params(&recorder{}))
			params.Project.Run.Jobs = append(params.Project.Run.Jobs, project.Job{Name: "web", Kind: project.Service, Cmd: "npm start", Ports: map[string]int{"http": 3000}})
			params.Project.Run.EnvPorts = []project.EnvPortLink{{File: ".env", Key: "PORT", Job: "web", Port: "http"}}
			if name == "value_unresolved" {
				params.Project.Run.EnvValues = []project.EnvValueLink{{File: ".env", Key: "API", Job: "ghost", Value: "x"}}
			}

			outcome := w.service.Run(t.Context(), params)

			require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
			assert.Len(t, outcome.Warnings, 1)
			written, _ := files.File(filepath.Join(w.path(branchName), ".env"))
			assert.Equal(t, mainEnv+"COMPOSE_PROJECT_NAME=main-feat-x\n", written)
		})
	}
}

func TestAPortLinkedWorktreeIsNumberedAndItsEnvMovedOntoItsPorts(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}

	outcome := w.service.Run(t.Context(), portLinked(w.params(report)))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"worktree", "env", "meta", "ordinal", "ports"}, report.phases())
	assert.Equal(t, 1, outcome.Provisioned.Meta.Ordinal)
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

	outcome := w.service.Run(t.Context(), params)

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"worktree", "env", "meta", "ports"}, report.phases())
	assert.Equal(t, []domain.Warning{{
		Code:     "provision.ports_not_settled_run",
		Message:  "run.toml: line 3 — ports not settled",
		Params:   domain.Params{"branch": branchName, "cause": "run.toml: line 3"},
		FollowUp: settled("run.toml"),
	}}, outcome.Warnings)
	assert.Zero(t, outcome.Provisioned.Meta.Ordinal)
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branchName), ".env")))
}

func TestAVerbatimWorktreeKeepsItsEnvAsCopiedAndHasNoNumber(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}
	params := portLinked(w.params(report))
	params.Worktree.Answers = answered(provision.Given{EnvFrom: project.EnvMain, Isolation: project.Verbatim})

	outcome := w.service.Run(t.Context(), params)

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"worktree", "env", "meta"}, report.phases())
	assert.Zero(t, outcome.Provisioned.Meta.Ordinal)
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branchName), ".env")))
}

func TestAnEnvValueThatCannotBeExpandedIsSaidAsARunTomlToFix(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	params.Project.Run.EnvValues = []project.EnvValueLink{{File: ".env", Key: "API", Job: "ghost", Value: "x"}}

	outcome := w.service.Run(t.Context(), params)

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"provision.ports_not_settled_run"}, codes(outcome.Warnings))
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branchName), ".env")))
}

func TestAProxyPortOutOfRangeIsSaidAsAConfigToFix(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	params.Project.Global.Proxy.Port = 70000

	outcome := w.service.Run(t.Context(), params)

	assert.Equal(t, []string{"provision.ports_not_settled"}, codes(outcome.Warnings))
}

func TestALinkToAnEnvConfigTomlDoesNotDeclareIsNamedAndLeftOut(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	params.Project.Run.EnvPorts = append(params.Project.Run.EnvPorts, project.EnvPortLink{File: "web/.env", Key: "WEB_PORT", Job: "web", Port: "http"})
	params.Project.Run.EnvValues = []project.EnvValueLink{{File: "api/.env", Key: "API", Job: "web", Value: "x"}}

	outcome := w.service.Run(t.Context(), params)

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []domain.Warning{
		{
			Code:    "provision.env_port_ignored",
			Message: "env_port WEB_PORT in web/.env ignored: not a configured env file — add it to [env] in config.toml or drop the link",
			Params:  domain.Params{"key": "WEB_PORT", "file": "web/.env"},
		},
		{
			Code:    "provision.env_value_ignored",
			Message: "env API in api/.env ignored: not a configured env file — add it to [env] in config.toml or drop the link",
			Params:  domain.Params{"key": "API", "file": "api/.env"},
		},
	}, outcome.Warnings)
	assert.Equal(t, "PORT=3010\n", readFile(t, filepath.Join(w.path(branchName), ".env")))
}

func TestNamedAddressesNoProxyServesAreWrittenAsPortsAndSaid(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	params.Project.Run.Addressing = project.AddressNames

	outcome := w.service.Run(t.Context(), params)

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []domain.Warning{{
		Code:     "provision.ports_not_named",
		Message:  "this project asks for named addresses: the run proxy is off on this machine, so ports were written instead",
		Params:   domain.Params{"branch": branchName},
		FollowUp: &domain.FollowUp{Command: "run proxy status", Code: "provision.proxy_status", Message: "reports what serves names"},
	}}, outcome.Warnings)
	assert.Equal(t, "PORT=3010\n", readFile(t, filepath.Join(w.path(branchName), ".env")))
}

func TestNamedAddressesAProxyServesAreNotSaid(t *testing.T) {
	w := onDisk(t)
	w.service.PublicPort = func(context.Context, int) int { return 80 }
	params := portLinked(w.params(&recorder{}))
	params.Project.Run.Addressing = project.AddressNames

	outcome := w.service.Run(t.Context(), params)

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Empty(t, outcome.Warnings)
}

func TestAnOriginWithoutTheRunProxyIsAProxyToFixNotARunToml(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	params.Project.Run.Addressing = project.AddressNames
	params.Project.Run.Jobs[0].URL = &project.URL{Port: "http"}
	params.Project.Run.EnvValues = []project.EnvValueLink{{File: ".env", Key: "API", Job: "web", Value: "{origin}"}}

	outcome := w.service.Run(t.Context(), params)

	require.Equal(t, []string{"provision.ports_not_settled"}, codes(outcome.Warnings))
	assert.Equal(t, "env API in .env: {origin} of job \"web\" is served by the run proxy, which serves no port: enable the proxy, or write the host yourself — ports not settled", outcome.Warnings[0].Message)
	assert.Equal(t, settled("what kept them"), outcome.Warnings[0].FollowUp)
}

func TestAPortPassThatFailsNeverFailsTheWorktree(t *testing.T) {
	w, _ := inMemory(t, seeds{Store: storetest.FakeParams{Fail: map[string]error{"Edit": errors.New("read-only file system")}}})

	outcome := w.service.Run(t.Context(), portLinked(w.params(&recorder{})))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"provision.ports_not_settled"}, codes(outcome.Warnings))
	assert.NotZero(t, outcome.Provisioned.Meta.Ordinal)
	assert.Empty(t, outcome.Provisioned.EnvPorts)
}

func TestAComposeStackGetsItsProjectNameWithoutAnyPortLink(t *testing.T) {
	w := onDisk(t)

	outcome := w.service.Run(t.Context(), composed(w.params(&recorder{})))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, mainEnv+"COMPOSE_PROJECT_NAME=main-feat-x\n", readFile(t, filepath.Join(w.path(branchName), ".env")))
}

func TestOnCreateRunsInTheNewWorktreeAndKeepsItsLog(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}

	outcome := w.service.Run(t.Context(), hooked(w.params(report), "pwd", "echo from {{from_branch}} on {{branch}}"))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{w.path(branchName), "from main on feat/x"}, report.lines())
	assert.Equal(t, env.HookLogPath(env.HookLogPathParams{StateDir: w.stateDir, Branch: branchName, Hooks: env.OnCreateHooks}), report.logOf("on_create"))
	assert.Contains(t, report.All(), domain.Output{Subject: branchName, Line: "from main on feat/x"})
}

func TestAHookThatFailsLeavesTheWorktreeIncomplete(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}

	outcome := w.service.Run(t.Context(), hooked(w.params(report), "exit 3"))

	assert.Equal(t, domain.StatusFailed, outcome.Status)
	assert.Equal(t, domain.ReasonIncomplete, outcome.Reason)
	assert.Equal(t, "unit.phase_failed", outcome.Err.Code)
	assert.Equal(t, domain.Params{"phase": "on_create"}, outcome.Err.Params)
	assert.Equal(t, "on_create failed: exit status 3", outcome.Err.Error())
	assert.DirExists(t, w.path(branchName))
	provisioned, ok := report.worktreeEvents()[1].(domain.WorktreeProvisioned)
	require.True(t, ok)
	assert.False(t, provisioned.OK)
	assert.Equal(t, "exit 3", provisioned.Hook)
	assert.Equal(t, new(3), provisioned.ExitCode)
}

func TestAHookInterruptedLeavesTheWorktreeCancelledAndIncomplete(t *testing.T) {
	w := onDisk(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	report := &recorder{Recorder: operationtest.Recorder{On: func(event domain.Event) {
		if output, ok := event.(domain.Output); ok && output.Line == "started" {
			cancel()
		}
	}}}

	outcome := w.service.Run(ctx, hooked(w.params(report), "echo started; sleep 5", "echo never"))

	assert.Equal(t, domain.StatusCancelled, outcome.Status)
	assert.Equal(t, domain.ReasonIncomplete, outcome.Reason)
	assert.Equal(t, domain.CodeInterrupted, outcome.Err.Code)
	assert.Equal(t, domain.Params{"phase": "on_create"}, outcome.Err.Params)
	assert.Equal(t, []string{"started"}, report.lines())
	assert.DirExists(t, w.path(branchName))
	provisioned, ok := report.worktreeEvents()[1].(domain.WorktreeProvisioned)
	require.True(t, ok)
	assert.False(t, provisioned.OK)
}

func TestAComposeEnvThatCannotBeReadIsSaidAndTheHooksRunWithoutIt(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}
	params := hooked(composed(w.params(report)), "echo project=$COMPOSE_PROJECT_NAME")
	w.service.EnvFiles = unreadable{EnvFiles: w.service.EnvFiles}

	outcome := w.service.Run(t.Context(), params)

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"project="}, report.lines())
	assert.Equal(t, []domain.Warning{{
		Code:    "provision.hook_env_unavailable",
		Message: "read .env: permission denied — on_create hooks run without the worktree's environment",
		Params:  domain.Params{"cause": "read .env: permission denied"},
	}}, outcome.Warnings)
}

func TestAHookReadsTheRunEnvironmentOnlyWhenTheRunReachesHooks(t *testing.T) {
	t.Setenv("WTM_PORT_OFFSET", "90")
	echo := "echo offset=$WTM_PORT_OFFSET project=$COMPOSE_PROJECT_NAME"
	plain, composedReport := &recorder{}, &recorder{}

	plainWorld, composedWorld := onDisk(t), onDisk(t)

	plainWorld.service.Run(t.Context(), hooked(plainWorld.params(plain), echo))
	composedWorld.service.Run(t.Context(), hooked(composed(composedWorld.params(composedReport)), echo))

	assert.Equal(t, []string{"worktree", "env", "meta", "on_create"}, plain.phases())
	assert.Equal(t, []string{"worktree", "env", "meta", "ordinal", "ports", "on_create"}, composedReport.phases())
	assert.Equal(t, []string{"offset= project="}, plain.lines())
	require.Len(t, composedReport.lines(), 1)
	assert.True(t, strings.HasPrefix(composedReport.lines()[0], "offset=10 project=main-feat-x"), composedReport.lines())
}

func TestThePhasesForeseenAreThoseRunReports(t *testing.T) {
	cases := map[string]func(provision.Params) provision.Params{
		"bare":        func(p provision.Params) provision.Params { return p },
		"port_linked": portLinked,
		"hooked":      func(p provision.Params) provision.Params { return hooked(portLinked(p), "true") },
		"composed":    composed,
	}
	for name, shape := range cases {
		t.Run(name, func(t *testing.T) {
			w := onDisk(t)
			report := &recorder{}
			params := shape(w.params(report))
			foreseen := provision.Phases(params)

			outcome := w.service.Run(t.Context(), params)

			require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
			var reported []domain.Phase
			for _, phase := range report.phases() {
				reported = append(reported, domain.Phase(phase))
			}
			assert.Equal(t, foreseen, reported)
		})
	}
}

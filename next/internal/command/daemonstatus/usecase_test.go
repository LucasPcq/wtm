package daemonstatus_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstatus"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
)

func TestADaemonOfThisBuildIsReadWithItsJobsByRepositoryAndWorktree(t *testing.T) {
	got := read(t, answering(run.Stamp{Protocol: run.ProtocolVersion, Version: build, PID: 42}, run.SkewNone))

	assert.Empty(t, got.Warnings)
	assert.Equal(t, &daemonstatus.Detail{
		Running: true, PID: 42, Build: build, Daemon: build, Protocol: run.ProtocolVersion,
		Repos: []run.HeldRepo{{Root: "/acme", Worktrees: []run.HeldWorktree{
			{Branch: "main", Path: "/acme", Jobs: []run.HeldJob{{Name: "db", Kind: project.Service, Hold: run.HoldOwn, Shared: true}, {Name: "web", Kind: project.Service, Hold: run.HoldOwn}}},
			{Branch: "feat/a", Path: "/acme/.trees/feat-a", Jobs: []run.HeldJob{{Name: "cache", Kind: project.Service, Hold: run.HoldDetached}, {Name: "db", Kind: project.Service, Hold: run.HoldClaim}}},
		}}},
		Proxy:  &daemonstatus.Proxy{Port: 11080, PublicPort: 11080},
		Socket: "/cfg/wtm/wtm.sock", Index: "/cfg/wtm/jobs.json",
	}, got.Items[0].Detail)
}

func TestADaemonOfAnotherBuildIsWarnedOfWithTheRestartThatHandsItsJobsOver(t *testing.T) {
	got := read(t, answering(run.Stamp{Protocol: run.ProtocolVersion, Version: "0.30.9", PID: 42}, run.SkewBuild))

	assert.Equal(t, []string{daemonstatus.CodeDaemonSkew}, codes(got.Warnings))
	assert.Equal(t, domain.CommandDaemonRestart, got.Warnings[0].FollowUp.Command)
	assert.Equal(t, run.SkewBuild, got.Items[0].Detail.Skew)
	assert.Equal(t, "0.30.9", got.Items[0].Detail.Daemon)
}

func TestADaemonOfAnotherProtocolIsReadWithoutItsJobs(t *testing.T) {
	daemon := answering(run.Stamp{Protocol: 9, Version: "0.40.0"}, run.SkewProtocol)
	daemon.Jobs = nil

	got := read(t, daemon).Items[0].Detail

	assert.True(t, got.JobsUnreadable)
	assert.Empty(t, got.Repos)
}

func TestWithNoDaemonTheStacksItsIndexKeepsAreRead(t *testing.T) {
	stack := run.Job{Name: "cache", Repo: "/acme", Branch: "feat/a", WorkDir: "/acme/.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldDetached, State: run.Running}

	got := read(t, run.Daemon{Jobs: []run.Job{stack}, Socket: "/s", Index: "/i"})

	assert.Empty(t, got.Warnings)
	assert.False(t, got.Items[0].Detail.Running)
	assert.Nil(t, got.Items[0].Detail.Proxy)
	assert.Equal(t, []run.HeldRepo{{Root: "/acme", Worktrees: []run.HeldWorktree{{Branch: "feat/a", Path: "/acme/.trees/feat-a", Jobs: []run.HeldJob{{Name: "cache", Kind: project.Service, Hold: run.HoldDetached}}}}}}, got.Items[0].Detail.Repos)
}

func TestAnIndexANewerWtmWroteIsWarnedOfRunningOrNot(t *testing.T) {
	cases := map[string]run.Daemon{
		"running":     answering(run.Stamp{Protocol: run.ProtocolVersion, Version: build}, run.SkewNone),
		"not_running": {Index: "/cfg/wtm/jobs.json"},
	}
	for name, daemon := range cases {
		t.Run(name, func(t *testing.T) {
			daemon.IndexFrozen = true

			got := read(t, daemon)

			assert.Equal(t, []string{daemonstatus.CodeIndexFrozen}, codes(got.Warnings))
			assert.Equal(t, "/cfg/wtm/jobs.json", got.Warnings[0].Params["path"])
			assert.True(t, got.Items[0].Detail.IndexFrozen)
		})
	}
}

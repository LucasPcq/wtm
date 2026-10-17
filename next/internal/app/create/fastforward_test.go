package create_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/assert"
)

func TestTheSourceIsFastForwardedBeforeTheWorktreeStartsFromIt(t *testing.T) {
	w := newWorld(t, seed{Behind: true, NoEnv: true})

	result, report := w.apply(t, create.Request{Branches: []string{branchName}, FastForward: new(true)})

	assert.Empty(t, result.Warnings)
	upstream := gittest.Output(t, w.main, "rev-parse", "origin/main")
	assert.Equal(t, upstream, gittest.Output(t, w.main, "rev-parse", "main"))
	assert.Equal(t, upstream, gittest.Output(t, w.path(branchName), "rev-parse", "HEAD"))
	assert.Equal(t, []wtm.Event{
		wtm.PhaseStarted{Subject: "main", Phase: "fast_forward"},
		wtm.PhaseFinished{Subject: "main", Phase: "fast_forward"},
	}, report.all()[:2])
}

func TestASourceWithUncommittedChangesIsLeftWhereItIsAndSaysSo(t *testing.T) {
	w := newWorld(t, seed{Behind: true})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, FastForward: new(true)})

	assert.Equal(t, []wtm.Warning{{
		Code:    "create.ff_dirty",
		Message: "couldn't fast-forward main: its worktree has uncommitted changes — it starts from your local branch",
		Params:  wtm.Params{"branch": "main"},
	}}, result.Warnings)
	assert.Equal(t, wtm.StatusDone, result.Items[0].Status)
	assert.NotEqual(t, gittest.Output(t, w.main, "rev-parse", "origin/main"), gittest.Output(t, w.path(branchName), "rev-parse", "HEAD"))
}

func TestASourceWithoutWorktreeIsMovedWithoutCheckingItOut(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "push", "-q", "origin", gittest.Output(t, w.main, "commit-tree", existing+"^{tree}", "-p", existing, "-m", "upstream")+":refs/heads/"+existing)
	gittest.Git(t, w.main, "fetch", "-q", "origin")

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, From: existing, FastForward: new(true)})

	assert.Empty(t, result.Warnings)
	assert.Equal(t, gittest.Output(t, w.main, "rev-parse", "origin/"+existing), gittest.Output(t, w.main, "rev-parse", existing))
}

func TestAFastForwardThatFailsIsAWarningAndTheBranchStillStarts(t *testing.T) {
	w := newWorld(t, seed{})
	behindOrigin(t, w, existing)
	gittest.Git(t, w.main, "remote", "set-url", "origin", w.root+"/gone")

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, From: existing, FastForward: new(true)})

	assert.Equal(t, []string{"create.ff_failed"}, codes(result.Warnings))
	assert.Contains(t, result.Warnings[0].Message, "couldn't fast-forward feature: ")
	assert.Contains(t, result.Warnings[0].Message, " — it starts from your local branch")
	assert.Equal(t, wtm.StatusDone, result.Items[0].Status)
}

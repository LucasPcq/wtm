package provision_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWorktreeIsAddedWithItsEnvAndItsRecord(t *testing.T) {
	w := onDisk(t)

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	meta := worktree.Meta{SourceBranch: "main", CreatedAt: "2026-10-16T12:00:00Z", EnvStrategy: project.EnvMain, Isolation: run.Isolated}
	assert.Equal(t, provision.Provisioned{Path: w.path(branchName), Metadata: meta}, outcome.Provisioned)
	assert.Empty(t, outcome.Warnings)
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branchName), ".env")))
	recorded, err := w.meta.Load(branchName)
	require.NoError(t, err)
	assert.Equal(t, meta, recorded)
}

func TestAnExistingBranchCarriesWhereItStandsAgainstOrigin(t *testing.T) {
	w := onDisk(t)
	params := w.existing(&recorder{})
	params.Worktree.Origin = provision.Origin{ExistingBranch: true, OriginState: "behind", OriginBehind: 2}

	outcome := provision.Run(t.Context(), params)

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, params.Worktree.Origin, outcome.Provisioned.Origin)
}

func TestEachEnvThatCouldNotBeCopiedIsAWarning(t *testing.T) {
	w := onDisk(t)
	require.NoError(t, os.Remove(filepath.Join(w.main, ".env")))

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []wtm.Warning{{
		Code:    "provision.env_source_missing",
		Message: "file .env not found in " + w.main + ", skipping",
		Params:  wtm.Params{"target": ".env", "dir": w.main},
	}}, outcome.Warnings)
}

func TestATemplateOrAParentThatIsMissingIsNamed(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	example := w.params(&recorder{})
	example.Project.EnvFiles = []project.EnvFile{{Target: "web/.env"}}
	example.Worktree.Strategy = project.EnvExample
	parent := w.params(&recorder{})
	parent.Worktree.Branch, parent.Worktree.Path = "feat/y", w.path("feat/y")
	parent.Worktree.Checkout = worktree.NewBranch{Name: "feat/y", From: branch.LocalBranch{Name: "main"}}
	parent.Worktree.Strategy = project.EnvParent
	parent.Worktree.ParentPath = w.path("elsewhere")

	fromTemplate := provision.Run(t.Context(), example)
	fromParent := provision.Run(t.Context(), parent)

	assert.Equal(t, []wtm.Warning{{Code: "provision.env_no_template", Message: "no template found for web/.env, skipping", Params: wtm.Params{"target": "web/.env"}}}, fromTemplate.Warnings)
	assert.Equal(t, []wtm.Warning{{Code: "provision.env_parent_fallback", Message: "file .env not found in the parent worktree, copied from main", Params: wtm.Params{"target": ".env"}}}, fromParent.Warnings)
}

func TestAProjectWithoutEnvFilesCopiesNothing(t *testing.T) {
	w, files := inMemory(t, seeds{})
	params := w.params(&recorder{})
	params.Project.EnvFiles = nil

	outcome := provision.Run(t.Context(), params)

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.NotContains(t, files.Calls(), "Copy "+w.path(branchName))
}

func TestAWriteWithoutItsLockIsSaidOnce(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	params.Deps.Meta = unlocked{w.meta}

	outcome := provision.Run(t.Context(), params)

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []wtm.Warning{{Code: "locks.unavailable", Message: "ran without operation locks"}}, outcome.Warnings)
}

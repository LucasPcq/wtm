package initialize_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFirstInitWritesWhatTheRepositorySuggestsAndRegistersIt(t *testing.T) {
	w := newWorld(t)

	outcome, err := w.usecase.Run(t.Context(), initialize.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.True(t, outcome.Plan.Created)
	assert.Equal(t, []initialize.Item{{Subject: w.usecase.ConfigPath, Status: domain.StatusDone, Detail: &initialize.Detail{Path: w.usecase.ConfigPath, Created: true}}}, outcome.Result.Items)
	assert.Equal(t, project.Config{
		Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"},
		Env:       project.Env{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}, {Target: "apps/web/.env", Template: "apps/web/.env.example"}}},
		OnCreate:  []project.Hook{{Cmd: "pnpm install"}},
		OnClean:   []project.Hook{{Cmd: "docker compose down"}},
	}, w.saved(t))
	assert.Equal(t, 1, *w.registered)
	assert.FileExists(t, filepath.Join(w.stateDir, "schemas", "project.schema.json"))
}

// Legacy's `init --only env --yes` rebuilt the .env list from what it found, bringing back a file the user removed.
func TestAReInitNeverBringsBackWhatTheUserRemoved(t *testing.T) {
	w := newWorld(t)
	w.configure(t, reInit)

	outcome, err := w.usecase.Run(t.Context(), initialize.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []project.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}}, outcome.Plan.Settings.EnvFiles)
	assert.True(t, outcome.Plan.Empty)
	assert.Equal(t, domain.ReasonUnchanged, outcome.Result.Items[0].Reason)
	assert.Equal(t, reInit, w.configText(t), "nothing changes: nothing is written")
	assert.Zero(t, *w.registered)
}

// Legacy's `init --base-branch dev --yes` on a repository set up already said "already exists" and dropped the flag.
func TestAReInitChangesTheSettingsGivenAndKeepsTheRest(t *testing.T) {
	w := newWorld(t)
	w.configure(t, reInit)
	req, err := initialize.Request{}.Answer(settings.PathBaseBranch, []string{"develop"})
	require.NoError(t, err)

	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{settings.PathBaseBranch}, outcome.Plan.Changed)
	saved := w.saved(t)
	assert.Equal(t, "develop", saved.Worktrees.BaseBranch)
	assert.Equal(t, []project.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}}, saved.Env.Files)
	assert.Equal(t, []project.Hook{{Cmd: "pnpm install"}, {Cmd: "pnpm db:generate", Cwd: "apps/api"}}, saved.OnCreate)
	assert.Equal(t, map[string]string{project.RememberIsolation: "verbatim"}, saved.Remembered)
}

func TestAnInvalidSettingIsRefusedByItsField(t *testing.T) {
	w := newWorld(t)
	req, err := initialize.Request{}.Answer(settings.PathEnvStrategy, []string{"copy"})
	require.NoError(t, err)

	_, err = w.usecase.Run(t.Context(), req, domain.RunOptions{})

	failure := failureOf(t, err)
	assert.Equal(t, domain.KindInvalid, failure.Kind)
	require.Len(t, failure.Fields, 1)
	assert.Equal(t, settings.PathEnvStrategy, failure.Fields[0].Path)
	assert.NoFileExists(t, w.usecase.ConfigPath)
}

func TestAConfigThatDoesNotParseIsEditedByHandFirst(t *testing.T) {
	w := newWorld(t)
	w.configure(t, "[worktrees]\nbase_branch =\n")

	_, err := w.usecase.Run(t.Context(), initialize.Request{}, domain.RunOptions{})

	failure := failureOf(t, err)
	assert.Equal(t, initialize.CodeConfigInvalid, failure.Code)
	require.NotNil(t, failure.FollowUp)
	assert.Equal(t, initialize.CodeEditConfig, failure.FollowUp.Code)
}

// Review of #282: a remembered answer init cannot keep was a problem of no section of its form, which read answered; init writes the remembered answers back as they are, so it cannot fix one.
func TestARememberedAnswerInitCannotKeepIsEditedByHandFirst(t *testing.T) {
	w := newWorld(t)
	w.configure(t, "[wizard.remembered]\nisolation = \"bogus\"\n")

	_, ran := w.usecase.Run(t.Context(), initialize.Request{}, domain.RunOptions{})
	_, asked := w.usecase.Questions(t.Context(), initialize.Request{})

	for _, err := range []error{ran, asked} {
		failure := failureOf(t, err)
		assert.Equal(t, initialize.CodeConfigInvalid, failure.Code)
		assert.Contains(t, failure.Message, "remembered.isolation cannot remember bogus")
		require.NotNil(t, failure.FollowUp)
		assert.Equal(t, initialize.CodeEditConfig, failure.FollowUp.Code)
	}
}

func TestAConfigANewerWtmWroteIsNotSetUpAgain(t *testing.T) {
	w := newWorld(t)
	w.configure(t, "schema = 2\n[worktrees]\n")

	_, err := w.usecase.Run(t.Context(), initialize.Request{}, domain.RunOptions{})

	failure := failureOf(t, err)
	assert.Equal(t, initialize.CodeConfigNewer, failure.Code)
	_, newer := errors.AsType[*domain.SchemaNewer](failure)
	assert.True(t, newer)
}

// The web's Couldn't save showed git's raw error, absolute paths and all: a folder it may not write names the folder to fix.
func TestAConfigItMayNotWriteNamesTheFolderToFix(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, os.MkdirAll(w.stateDir, 0o755))
	require.NoError(t, os.Chmod(w.stateDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(w.stateDir, 0o755) })

	outcome, err := w.usecase.Run(t.Context(), initialize.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	failure := outcome.Result.Items[0].Err
	require.NotNil(t, failure)
	assert.Equal(t, initialize.CodeWriteFailed, failure.Code)
	assert.Equal(t, "permission denied on .git/wtm/config.toml: check who owns .git/wtm, then set it up again", failure.Text())
}

func TestAConfigChangedOnDiskSinceThePlanIsNotOverwritten(t *testing.T) {
	w := newWorld(t)
	w.configure(t, reInit)
	req, err := initialize.Request{}.Answer(settings.PathOnClean, []string{"make down"})
	require.NoError(t, err)
	plan, _, err := w.usecase.Plan(t.Context(), req)
	require.NoError(t, err)
	w.configure(t, reInit+"\n")

	result := w.usecase.Apply(t.Context(), plan, func(domain.Event) {})

	require.Len(t, result.Items, 1)
	assert.Equal(t, initialize.CodeChangedOnDisk, result.Items[0].Err.Code)
	assert.Equal(t, domain.KindConflict, result.Items[0].Err.Kind)
	assert.Equal(t, reInit+"\n", w.configText(t))
}

// Legacy offered relocate after every init, with no worktree to relocate.
func TestRelocateIsOfferedOnlyForWorktreesOutsideTheFolder(t *testing.T) {
	w := newWorld(t)
	req, err := initialize.Request{}.Answer(settings.PathBasePath, []string{".."})
	require.NoError(t, err)

	other := newWorld(t)
	elsewhere, err := other.usecase.Run(t.Context(), initialize.Request{}, domain.RunOptions{})
	require.NoError(t, err)
	inside, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})
	require.NoError(t, err)

	assert.Equal(t, []string{initialize.CodeCreate, initialize.CodeRelocate, initialize.CodeRunInit}, codes(elsewhere.Result.FollowUps))
	assert.Equal(t, []string{initialize.CodeCreate, initialize.CodeRunInit}, codes(inside.Result.FollowUps))
}

func TestRunInitIsOfferedOnlyWhileRunTomlDeclaresNothing(t *testing.T) {
	w := newWorld(t)
	writeFile(t, filepath.Join(w.stateDir, "run.toml"), "[jobs.web]\ncmd = \"pnpm dev\"\n")

	outcome, err := w.usecase.Run(t.Context(), initialize.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.NotContains(t, codes(outcome.Result.FollowUps), initialize.CodeRunInit)
}

func TestMovingTheFolderWarnsOfTheWorktreesLeftInTheOldOne(t *testing.T) {
	w := newWorld(t)
	w.configure(t, "[worktrees]\nbase_path = \"..\"\n")
	req, err := initialize.Request{}.Answer(settings.PathBasePath, []string{"../trees"})
	require.NoError(t, err)

	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{DryRun: true})

	require.NoError(t, err)
	require.Len(t, outcome.Warnings, 1)
	assert.Equal(t, initialize.CodeBasePathMoved, outcome.Warnings[0].Code)
	assert.Equal(t, initialize.CodeRelocate, outcome.Warnings[0].FollowUp.Code)
	assert.Equal(t, "1", outcome.Warnings[0].Params["count"])
}

func TestADryRunWritesNothingAndTakesNoLock(t *testing.T) {
	w := newWorld(t)

	outcome, err := w.usecase.Run(t.Context(), initialize.Request{}, domain.RunOptions{DryRun: true})

	require.NoError(t, err)
	assert.NotEmpty(t, outcome.Fingerprint)
	assert.Nil(t, outcome.Result)
	_, statErr := os.Stat(w.usecase.ConfigPath)
	assert.True(t, os.IsNotExist(statErr))
}

// Review of #282: `init --yes --env-file apps/api/.env` on a repository copying no .env file said "1 unchanged".
func TestAnEnvFileGivenWhereNoneIsCopiedIsWritten(t *testing.T) {
	w := newWorld(t)
	w.configure(t, "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n")
	req, err := initialize.Request{}.Answer(settings.PathEnvFiles, []string{"apps/api/.env"})
	require.NoError(t, err)

	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{settings.PathEnvStrategy, settings.PathEnvFiles}, outcome.Plan.Changed)
	assert.Equal(t, project.Env{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}}}, w.saved(t).Env)
}

func TestTheSameFolderSpelledAnotherWayIsNoChange(t *testing.T) {
	w := newWorld(t)
	w.configure(t, reInit)
	req, err := initialize.Request{}.Answer(settings.PathBasePath, []string{"../.trees/"})
	require.NoError(t, err)

	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{DryRun: true})

	require.NoError(t, err)
	assert.True(t, outcome.Plan.Empty)
	assert.Empty(t, outcome.Warnings)
}

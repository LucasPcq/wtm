package configset_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/configset"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASettingGivenIsWrittenAndTheOthersKept(t *testing.T) {
	dir := stateDir(t, acme)
	req := answered(t, map[string][]string{settings.PathEnvStrategy: {"main"}})

	outcome, err := usecase(dir, operations()).Run(t.Context(), req, domain.RunOptions{})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	assert.Nil(t, outcome.Failure())
	assert.Equal(t, []configset.Change{{Path: settings.PathEnvStrategy, Before: []string{"example"}, After: []string{"main"}}}, outcome.Plan.Changes)
	written := read(t, dir)
	assert.Contains(t, written, `strategy = "main"`)
	assert.Contains(t, written, `base_path = "../.trees"`)
	assert.Contains(t, written, `cwd = "packages/db"`)
	assert.Contains(t, written, `env_strategy = "parent"`)
}

func TestAnAppliedChangeIsFollowedByItsUndo(t *testing.T) {
	dir := stateDir(t, acme)
	req := answered(t, map[string][]string{settings.PathBasePath: {"../wt"}})

	outcome, err := usecase(dir, operations()).Run(t.Context(), req, domain.RunOptions{})

	require.NoError(t, err)
	saved, err := usecase(dir, operations()).Settings.Read()
	require.NoError(t, err)
	assert.Equal(t, []domain.FollowUp{{Command: configset.Name, Answers: []domain.Answered{{Path: settings.PathBasePath, Values: []string{"../.trees"}}, {Path: configset.PathReplaces, Values: []string{saved.Digest}}}, Code: configset.CodeUndo, Message: "put back what it held"}}, outcome.Result.FollowUps)
}

// an Undo clicked after a hand edit wrote the old value over the edit
func TestAnUndoAfterTheFileChangedIsRefused(t *testing.T) {
	dir := stateDir(t, acme)
	u := usecase(dir, operations())
	outcome, err := u.Run(t.Context(), answered(t, map[string][]string{settings.PathBasePath: {"../wt"}}), domain.RunOptions{})
	require.NoError(t, err)
	edited := read(t, dir) + "\n# edited by hand\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(edited), 0o644))
	req := configset.Request{}
	for _, answer := range outcome.Result.FollowUps[0].Answers {
		req, err = req.Answer(answer.Path, answer.Values)
		require.NoError(t, err)
	}

	_, err = u.Run(t.Context(), req, domain.RunOptions{})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, configset.CodeChangedOnDisk, refusal.Code)
	assert.Equal(t, edited, read(t, dir))
}

func TestTheUndoPutsTheFileBack(t *testing.T) {
	dir := stateDir(t, acme)
	u := usecase(dir, operations())
	outcome, err := u.Run(t.Context(), answered(t, map[string][]string{settings.PathOnClean: {}}), domain.RunOptions{})
	require.NoError(t, err)
	undo := outcome.Result.FollowUps[0]
	req := configset.Request{}
	for _, answer := range undo.Answers {
		req, err = req.Answer(answer.Path, answer.Values)
		require.NoError(t, err)
	}

	_, err = u.Run(t.Context(), req, domain.RunOptions{})

	require.NoError(t, err)
	assert.Contains(t, read(t, dir), `"docker compose down"`)
}

func TestForgettingARememberedAnswerKeepsTheSettings(t *testing.T) {
	dir := stateDir(t, acme)

	_, err := usecase(dir, operations()).Run(t.Context(), answered(t, map[string][]string{configset.PathRemembered: {}}), domain.RunOptions{})

	require.NoError(t, err)
	written := read(t, dir)
	assert.NotContains(t, written, `env_strategy = "parent"`)
	assert.Contains(t, written, `strategy = "example"`)
}

func TestARequestThatChangesNothingWritesNothing(t *testing.T) {
	dir := stateDir(t, acme)

	outcome, err := usecase(dir, operations()).Run(t.Context(), answered(t, map[string][]string{settings.PathBaseBranch: {"main"}}), domain.RunOptions{})

	require.NoError(t, err)
	assert.True(t, outcome.Plan.Empty)
	assert.Empty(t, outcome.Result.Items)
	assert.Equal(t, acme, read(t, dir))
}

func TestAnInvalidValueIsRefusedOnItsField(t *testing.T) {
	cases := map[string]struct {
		answers map[string][]string
		path    string
		code    string
	}{
		"absolute_base_path": {map[string][]string{settings.PathBasePath: {"/tmp/wt"}}, settings.PathBasePath, configset.Name + "." + settings.CodeBasePathAbsolute},
		"unknown_branch":     {map[string][]string{settings.PathBaseBranch: {"nope"}}, settings.PathBaseBranch, configset.Name + "." + settings.CodeBaseBranchUnknown},
		"unknown_strategy":   {map[string][]string{settings.PathEnvStrategy: {"copy"}}, settings.PathEnvStrategy, domain.CodeOneOf},
		"remembered_unknown": {map[string][]string{configset.PathRemembered: {"nope=yes"}}, configset.PathRemembered + "[0]", configset.Name + "." + settings.CodeRememberedUnknown},
		"remembered_invalid": {map[string][]string{configset.PathRemembered: {"env_strategy=copy"}}, configset.PathRemembered + "[0]", configset.Name + "." + settings.CodeRememberedNotAllowed},
		"remembered_twice":   {map[string][]string{configset.PathRemembered: {"env_strategy=main", "env_strategy=parent"}}, configset.PathRemembered + "[1]", configset.CodeRememberedTwice},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := stateDir(t, acme)

			_, err := usecase(dir, operations()).Run(t.Context(), answered(t, c.answers), domain.RunOptions{})

			refusal, ok := errors.AsType[*domain.Error](err)
			require.True(t, ok, "%v", err)
			require.Len(t, refusal.Fields, 1)
			assert.Equal(t, c.path, refusal.Fields[0].Path)
			assert.Equal(t, c.code, refusal.Fields[0].Code)
			assert.Equal(t, acme, read(t, dir))
		})
	}
}

func TestAFileChangedOnDiskSinceThePreviewIsAStalePlan(t *testing.T) {
	dir := stateDir(t, acme)
	u := usecase(dir, operations())
	req := answered(t, map[string][]string{settings.PathBasePath: {"../worktrees"}})
	preview, err := u.Run(t.Context(), req, domain.RunOptions{DryRun: true})
	require.NoError(t, err)
	theirs := strings.Replace(acme, `"../.trees"`, `"../wt"`, 1)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(theirs), 0o644))

	_, err = u.Run(t.Context(), req, domain.RunOptions{ExpectFingerprint: preview.Fingerprint})

	assert.True(t, domain.PlanStale(err), "%v", err)
	assert.Equal(t, theirs, read(t, dir))
}

func TestNoConfigIsRefusedWithInit(t *testing.T) {
	u := usecase(stateDir(t, ""), operations())
	u.Init = &domain.FollowUp{Command: domain.CommandInit}

	_, err := u.Run(t.Context(), configset.Request{}, domain.RunOptions{DryRun: true})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, configset.CodeConfigNotFound, refusal.Code)
	require.NotNil(t, refusal.FollowUp)
	assert.Equal(t, domain.CommandInit, refusal.FollowUp.Command)
}

func TestAConfigThatDoesNotParseIsRefused(t *testing.T) {
	u := usecase(stateDir(t, acme+"base_path = [\n"), operations())

	_, err := u.Run(t.Context(), configset.Request{}, domain.RunOptions{DryRun: true})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, configset.CodeConfigInvalid, refusal.Code)
}

func TestAConfigANewerWtmWroteIsNotRewritten(t *testing.T) {
	dir := stateDir(t, strings.Replace(acme, "schema = 1", "schema = 9", 1))

	_, err := usecase(dir, operations()).Run(t.Context(), answered(t, map[string][]string{settings.PathBasePath: {"../wt"}}), domain.RunOptions{})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, configset.CodeConfigNewer, refusal.Code)
	_, newer := errors.AsType[*domain.SchemaNewer](err)
	assert.True(t, newer)
	assert.Contains(t, read(t, dir), "schema = 9")
}

func TestAnInvalidValueOnDiskIsFixedByChangingIt(t *testing.T) {
	dir := stateDir(t, strings.Replace(acme, `strategy = "example"`, `strategy = "copy"`, 1))
	u := usecase(dir, operations())

	_, err := u.Run(t.Context(), answered(t, map[string][]string{settings.PathBasePath: {"../wt"}}), domain.RunOptions{})
	require.NoError(t, err, "a problem another setting holds is no refusal of this change")
	_, err = u.Run(t.Context(), answered(t, map[string][]string{settings.PathEnvStrategy: {"main"}}), domain.RunOptions{})

	require.NoError(t, err)
	assert.Contains(t, read(t, dir), `strategy = "main"`)
}

func TestAChangeMadeOnAFileChangedSinceIsRefused(t *testing.T) {
	dir := stateDir(t, acme)
	u := usecase(dir, operations())
	facts, err := u.Observe(t.Context())
	require.NoError(t, err)
	made := facts.Settings.Saved.Digest
	theirs := strings.Replace(acme, `"../.trees"`, `"../wt"`, 1)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(theirs), 0o644))

	_, err = u.Run(t.Context(), answered(t, map[string][]string{settings.PathBasePath: {"../worktrees"}, configset.PathReplaces: {made}}), domain.RunOptions{})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, configset.CodeChangedOnDisk, refusal.Code)
	assert.Equal(t, theirs, read(t, dir))
}

func TestTheQuestionsOfAConfigANewerWtmWroteAreAsked(t *testing.T) {
	u := usecase(stateDir(t, strings.Replace(acme, "schema = 1", "schema = 9", 1)), operations())

	questions, err := u.Questions(t.Context(), configset.Request{})

	require.NoError(t, err)
	assert.Equal(t, "../.trees", questions.BasePath.Value)
}

// A blank --base-path= said "nothing to do · config.toml already holds it", which was false.
func TestABlankValueIsRefusedNotIgnored(t *testing.T) {
	_, err := configset.Request{}.Answer(settings.PathBasePath, []string{""})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	require.Len(t, refusal.Fields, 1)
	assert.Equal(t, domain.CodeRequired, refusal.Fields[0].Code)
}

// a strategy given over a config.toml that names no .env file wrote one that copies none of the repository's
func TestAProblemTheChangeCreatesOnAnotherSettingIsRefused(t *testing.T) {
	dir := stateDir(t, noFiles)
	u := usecase(dir, operations())
	u.Detect = func() project.Detected {
		return project.Detected{EnvFiles: []project.EnvFile{{Target: "apps/api/.env"}}, Dirs: detected().Dirs}
	}

	_, err := u.Run(t.Context(), answered(t, map[string][]string{settings.PathEnvStrategy: {"main"}}), domain.RunOptions{})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	require.Len(t, refusal.Fields, 1)
	assert.Equal(t, settings.PathEnvFiles, refusal.Fields[0].Path)
	assert.Equal(t, noFiles, read(t, dir))
}

// config set wrote back a remembered answer wtm cannot keep and said nothing of it
func TestARememberedAnswerKeptAsItIsWarnsAndOffersTheEdit(t *testing.T) {
	dir := stateDir(t, strings.Replace(acme, `env_strategy = "parent"`, `env_strategy = "copy"`, 1))
	u := usecase(dir, operations())
	u.Edit = &domain.FollowUp{Command: domain.CommandConfigEdit}

	preview, err := u.Run(t.Context(), answered(t, map[string][]string{settings.PathBasePath: {"../wt"}}), domain.RunOptions{DryRun: true})

	require.NoError(t, err)
	require.Len(t, preview.Warnings, 1)
	assert.Equal(t, configset.CodeRememberedKept, preview.Warnings[0].Code)
	require.NotNil(t, preview.Warnings[0].FollowUp)
	assert.Equal(t, domain.CommandConfigEdit, preview.Warnings[0].FollowUp.Command)
	forgot, err := u.Run(t.Context(), answered(t, map[string][]string{configset.PathRemembered: {}}), domain.RunOptions{DryRun: true})
	require.NoError(t, err)
	assert.Empty(t, forgot.Warnings, "forgetting them leaves none to keep")
}

// forgetting a remembered answer sent back the others as they are, one wtm cannot keep among them, and was refused
func TestForgettingAnAnswerKeepsAnotherWtmCannotKeepWithAWarning(t *testing.T) {
	dir := stateDir(t, strings.Replace(acme, `env_strategy = "parent"`, "env_strategy = \"parent\"\nisolation = \"bogus\"", 1))

	outcome, err := usecase(dir, operations()).Run(t.Context(), answered(t, map[string][]string{configset.PathRemembered: {"isolation=bogus"}}), domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, outcome.Warnings, 1)
	assert.Equal(t, configset.CodeRememberedKept, outcome.Warnings[0].Code)
	written := read(t, dir)
	assert.NotContains(t, written, `env_strategy = "parent"`)
	assert.Contains(t, written, `isolation = "bogus"`)
}

func TestAnAnswerRememberedAnewThatWtmCannotKeepIsRefused(t *testing.T) {
	dir := stateDir(t, strings.Replace(acme, `env_strategy = "parent"`, "env_strategy = \"parent\"\nisolation = \"bogus\"", 1))

	_, err := usecase(dir, operations()).Run(t.Context(), answered(t, map[string][]string{configset.PathRemembered: {"isolation=other"}}), domain.RunOptions{})

	_, ok := errors.AsType[*domain.Error](err)
	assert.True(t, ok, "%v", err)
}

func TestTheRecapListsCommandsAndPathsOnePerLine(t *testing.T) {
	dir := stateDir(t, acme)
	req := answered(t, map[string][]string{settings.PathBasePath: {"../wt"}, settings.PathEnvFiles: {"apps/api/.env"}, settings.PathOnClean: {}})

	outcome, err := usecase(dir, operations()).Run(t.Context(), req, domain.RunOptions{DryRun: true})

	require.NoError(t, err)
	stacked := map[string]bool{}
	for _, row := range outcome.Plan.Recap {
		stacked[row.Key] = row.Stacked
	}
	assert.Equal(t, map[string]bool{settings.PathBasePath: false, settings.PathEnvFiles: true, settings.PathOnClean: true}, stacked)
}

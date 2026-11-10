package cli

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfInitNamesEachSettingsFlag(t *testing.T) {
	got := runInit(t, newWorld(seed{}), "--help")
	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "init_help", got.Stdout)
}

func TestAFirstInitUnattendedWritesWhatItFoundAndNamesTheNextSteps(t *testing.T) {
	w := newWorld(seed{})
	got := runInit(t, w, "--yes")
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "first_init", got.Stdout)
	assert.Equal(t, []project.Hook{{Cmd: "docker compose down"}}, w.store.saved.Config.OnClean)
}

// Legacy ran unattended without a terminal even with no flag; next does too, as every command.
func TestWithoutATerminalInitRunsUnattended(t *testing.T) {
	w := newWorld(seed{})
	got := runInit(t, w)
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.NotNil(t, w.store.saved)
}

func TestAReInitNotesWhatItChanges(t *testing.T) {
	w := newWorld(seed{Saved: setUp()})
	got := runInit(t, w, "--yes", "--base-branch", "develop", "--on-clean", `{"cmd": "make down", "continue_on_error": true}`)
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "re_init", got.Stdout)
	assert.Equal(t, []project.Hook{{Cmd: "make down", ContinueOnError: true}}, w.store.saved.Config.OnClean)
	assert.Equal(t, []project.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}}, w.store.saved.Config.Env.Files)
}

func TestAReInitThatChangesNothingSaysSo(t *testing.T) {
	got := runInit(t, newWorld(seed{Saved: setUp()}), "--yes")
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "unchanged", got.Stdout)
}

func TestADryRunShowsThePlanAndTheLineThatAppliesIt(t *testing.T) {
	w := newWorld(seed{})
	got := runInit(t, w, "--yes", "--env-strategy", "none", "--dry-run")
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
	assert.Nil(t, w.store.saved)
}

// Legacy exited 1 on an env strategy it did not know.
func TestAnInvalidSettingIsRefusedAsAnInvalidRequestNamingItsFlag(t *testing.T) {
	got := runInit(t, newWorld(seed{}), "--yes", "--env-strategy", "copy", "--base-path", "/abs")
	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Empty(t, got.Stdout)
	formtest.Golden(t, "invalid", got.Stderr)
}

func TestAConfigThatCannotBeWrittenSetsTheExitCode(t *testing.T) {
	got := runInit(t, newWorld(seed{WriteFail: errors.New("permission denied")}), "--yes")
	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "write_failed", got.Stdout+"--- stderr\n"+got.Stderr)
}

func TestEachEnvelopeOfInitHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Seed  seed
		Words []string
		Code  int
	}{
		"first":   {Words: []string{"--yes"}, Code: uicli.ExitOK},
		"re_init": {Seed: seed{Saved: setUp()}, Words: []string{"--yes", "--on-clean", "make down"}, Code: uicli.ExitOK},
		"dry_run": {Words: []string{"--yes", "--dry-run"}, Code: uicli.ExitOK},
		"invalid": {Words: []string{"--yes", "--env-strategy", "copy"}, Code: uicli.ExitUsage},
		"stale":   {Words: []string{"--expect-fingerprint", fingerprint}, Code: uicli.ExitPrecondition},
	} {
		t.Run(name, func(t *testing.T) {
			got := runInit(t, newWorld(c.Seed), append(c.Words, "--output", "json")...)
			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestAFollowUpNamingInitIsSpelledFromItsSchema(t *testing.T) {
	assert.Equal(t, []string{"wtm-next", "init"}, Line(domain.FollowUp{Command: initialize.Name}))
	assert.Equal(t, []string{"wtm-next", "init", "--on-clean", "make down"}, Line(domain.FollowUp{Command: initialize.Name, Answers: []domain.Answered{{Path: domain.PathOnClean, Values: []string{"make down"}}}}))
}

// Review of #282: a strategy no option names, as a hand-edited config.toml holds it, was recapped "main's (was )".
func TestAStrategyNoOptionNamesIsRecappedAsItIs(t *testing.T) {
	changed := domain.RecapValue{Value: "main", Note: initialize.NoteChanged, Params: domain.Params{"was": "copy"}}
	unknown := domain.RecapValue{Value: "copy"}
	outcome := initialize.Outcome{Plan: initialize.Plan{Recapped: domain.Recapped{Recap: []domain.RecapRow{
		{Path: domain.PathEnvStrategy, Values: []domain.RecapValue{changed, unknown}},
	}}}}

	values := worded(outcome).Plan.Recap[0].Values

	assert.Equal(t, "main's (was copy)", values[0].Text)
	assert.Equal(t, "copy", values[1].Text)
}

// The owner's rule: a recap lists commands and paths one per line, in the dry run as in the outcome.
func TestARecapListsEachHookAndEachEnvFileOnALineOfItsOwn(t *testing.T) {
	for _, dryRun := range []bool{true, false} {
		args := []string{"--yes", "--on-create", "pnpm install", "--on-create", "docker compose up -d"}
		if dryRun {
			args = append(args, "--dry-run")
		}

		got := runInit(t, newWorld(seed{}), args...)

		require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
		assert.Contains(t, got.Stdout, "  .env files        apps/api/.env\n                    apps/web/.env\n", "dry run: %v", dryRun)
		assert.Contains(t, got.Stdout, "  On create         pnpm install\n                    docker compose up -d\n", "dry run: %v", dryRun)
	}
}

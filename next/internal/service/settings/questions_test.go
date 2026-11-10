package settings

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFirstInitTakesWhatTheRepositorySuggests(t *testing.T) {
	q := questions(facts(nil), Given{})

	assert.Equal(t, domain.Answer[string]{Value: project.DefaultBasePath, Origin: domain.OriginDefault}, q.BasePath)
	assert.Equal(t, domain.Answer[string]{Value: "main", Origin: domain.OriginDetected}, q.BaseBranch.Answer)
	assert.Equal(t, domain.Answer[string]{Value: string(project.EnvExample), Origin: domain.OriginDefault}, q.EnvStrategy)
	assert.Equal(t, []string{"apps/api/.env", "apps/web/.env"}, q.EnvFiles.Picked)
	assert.Equal(t, domain.OriginDetected, q.EnvFiles.Origin)
	assert.Equal(t, Hooks{Entries: []project.Hook{{Cmd: "pnpm install"}}, Origin: domain.OriginDetected, Found: detected().OnCreate}, q.OnCreate)
	assert.Equal(t, []project.Hook{{Cmd: "docker compose down"}}, q.OnClean.Entries)
	assert.Empty(t, q.Problems())
}

func TestAReInitShowsEveryFileButPicksOnlyWhatTheConfigHolds(t *testing.T) {
	q := questions(facts(saved()), Given{})

	assert.Equal(t, []string{"apps/api/.env"}, q.EnvFiles.Picked, "the file the user removed does not come back")
	assert.Equal(t, []project.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}, {Target: "apps/web/.env", Template: "apps/web/.env.example"}}, q.EnvFiles.Options)
	assert.Equal(t, saved().Config.OnCreate, q.OnCreate.Entries)
	assert.Equal(t, Hooks{Entries: []project.Hook{}, Origin: domain.OriginConfig, Found: detected().OnClean}, q.OnClean, "a list the config holds empty stays empty")
	assert.Equal(t, domain.Answer[string]{Value: "../acme.trees", Origin: domain.OriginConfig}, q.BasePath)
	assert.Equal(t, domain.Answer[string]{Value: string(project.EnvMain), Origin: domain.OriginConfig}, q.EnvStrategy)
}

func TestAReInitChangesOnlyTheSettingsGiven(t *testing.T) {
	q := questions(facts(saved()), Given{OnClean: []project.Hook{{Cmd: "make down"}}})

	rewrite := q.Rewrite()

	want := saved().Config
	want.OnClean = []project.Hook{{Cmd: "make down"}}
	assert.Equal(t, project.Rewrite{Config: want, Replaces: "abc"}, rewrite)
}

func TestAConfigThatCopiesNoEnvFileReadsAsNone(t *testing.T) {
	s := saved()
	s.Config.Env = project.Env{}

	q := questions(facts(s), Given{})

	assert.Equal(t, domain.Answer[string]{Value: EnvNone, Origin: domain.OriginConfig}, q.EnvStrategy)
	assert.Equal(t, domain.SkipNoEnv, q.EnvFiles.Skip)
	assert.Equal(t, project.Env{}, q.Rewrite().Config.Env)
}

func TestNoneWritesNeitherAStrategyNorAFile(t *testing.T) {
	q := questions(facts(nil), Given{EnvStrategy: EnvNone})

	assert.Equal(t, project.Env{}, q.Rewrite().Config.Env)
}

func TestAFileTypedByHandKeepsNoTemplate(t *testing.T) {
	q := questions(facts(nil), Given{EnvFiles: []string{"apps/web/.env", "apps/new/.env"}})

	assert.Equal(t, []project.EnvFile{{Target: "apps/web/.env", Template: "apps/web/.env.example"}, {Target: "apps/new/.env"}}, q.Rewrite().Config.Env.Files)
}

func TestTheProblemsOfTheSettingsAreTheAskingCommands(t *testing.T) {
	cases := map[string]struct {
		facts Facts
		given Given
		want  []domain.FieldError
	}{
		"base_path_absolute": {facts(nil), Given{BasePath: "/tmp/trees"}, []domain.FieldError{{Path: PathBasePath, Code: "init.base_path_absolute", Template: "{path} must be relative to the repository, not {value}", Params: domain.Params{"value": "/tmp/trees"}}}},
		"no_base_branch":     {Facts{}, Given{}, []domain.FieldError{domain.Required(PathBaseBranch)}},
		"unknown_branch":     {facts(nil), Given{BaseBranch: "mian"}, []domain.FieldError{{Path: PathBaseBranch, Code: "init.base_branch_unknown", Template: "{path} {value} is no branch of the repository", Params: domain.Params{"value": "mian"}}}},
		"env_file_outside":   {facts(nil), Given{EnvFiles: []string{"../other/.env"}}, []domain.FieldError{{Path: "env_files[0]", Code: "init.env_file_outside", Template: "{path} must be a file of the repository, not {value}", Params: domain.Params{"value": "../other/.env"}}}},
		"env_file_twice":     {facts(nil), Given{EnvFiles: []string{".env", ".env"}}, []domain.FieldError{{Path: "env_files[1]", Code: "init.env_file_twice", Template: "{path} {value} is listed twice", Params: domain.Params{"value": ".env"}}}},
		"hook_dir_outside":   {facts(nil), Given{OnClean: []project.Hook{{Cmd: "x", Cwd: "/srv"}}}, []domain.FieldError{{Path: "on_clean[0]", Code: "init.hook_dir_outside", Template: "{path} must run in a folder of the repository, not {value}", Params: domain.Params{"value": "/srv"}}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, questions(c.facts, c.given).Problems())
		})
	}
}

func TestTheFormFoldsBackIntoTheRequestItStandsFor(t *testing.T) {
	q := questions(facts(saved()), Given{})

	folded, err := domain.Fold(Given{}, domain.Form{Fields: q.Fields()}.Answers())

	require.NoError(t, err)
	assert.Equal(t, q.Given(), folded)
	for _, path := range []string{PathBasePath, PathBaseBranch, PathEnvStrategy, PathEnvFiles, PathOnCreate, PathOnClean} {
		again, err := Given{}.Answer(path, folded.Values(path))
		require.NoError(t, err)
		assert.Equal(t, folded.Values(path), again.Values(path), path)
	}
}

func TestABlankEntryOfAListIsNone(t *testing.T) {
	given, err := Given{}.Answer(PathOnCreate, []string{""})

	require.NoError(t, err)
	assert.Equal(t, []project.Hook{}, given.OnCreate)
	assert.Equal(t, []string{}, given.Values(PathOnCreate))
}

func TestOnlyTheSettingsGivenAnotherValueChange(t *testing.T) {
	was := questions(facts(saved()), Given{}).Document()

	now := questions(facts(saved()), Given{BaseBranch: "main", OnClean: []project.Hook{{Cmd: "make down"}}, EnvStrategy: EnvNone}).Document()

	assert.Equal(t, []string{PathEnvStrategy, PathEnvFiles, PathOnClean}, Changed(was, now))
	assert.Empty(t, now.EnvFiles)
}

func TestAConfigNamingNoBaseBranchReadsTheDefaultEveryCommandReads(t *testing.T) {
	s := saved()
	s.Config.Worktrees.BaseBranch = ""

	q := questions(Facts{Saved: s}, Given{})

	assert.Equal(t, domain.Answer[string]{Value: project.DefaultBaseBranch, Origin: domain.OriginDefault}, q.BaseBranch.Answer)
	assert.Empty(t, q.Problems())
}

func TestAValueOnDiskWtmCannotUseIsAProblemOfItsSetting(t *testing.T) {
	s := saved()
	s.Config.Env = project.Env{Strategy: "copy", Files: []project.EnvFile{{Target: ""}, {Target: "apps/api/.env", Template: "apps/web/.env.example"}}}
	s.Config.Remembered = map[string]string{"isolation": "both", "colour": "blue"}

	problems := questions(Facts{Saved: s}, Given{}).Problems()

	codes := map[string]string{}
	for _, problem := range problems {
		codes[problem.Path] = problem.Code
	}
	assert.Equal(t, map[string]string{
		PathEnvStrategy:        domain.CodeOneOf,
		"env_files[0]":         "init.env_file_no_target",
		"env_files[1]":         "init.env_file_template",
		"remembered.colour":    "init.remembered_unknown",
		"remembered.isolation": "init.remembered_not_allowed",
	}, codes)
}

// Review of #282: `--env-file` on a repository whose config.toml copies no .env file was dropped, "1 unchanged".
func TestEnvFilesGivenWhereConfigCopiesNoneAskForTheDefaultStrategy(t *testing.T) {
	s := saved()
	s.Config.Env = project.Env{}

	q := questions(facts(s), Given{EnvFiles: []string{"apps/api/.env"}})

	assert.Equal(t, domain.Answer[string]{Value: string(project.DefaultEnvStrategy), Origin: domain.OriginDefault}, q.EnvStrategy)
	assert.Empty(t, q.Problems())
	assert.Equal(t, []project.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}}, q.Rewrite().Config.Env.Files)
}

func TestTheStrategyAndItsFilesMustAgree(t *testing.T) {
	cases := map[string]struct {
		given Given
		path  string
		code  string
	}{
		"files_with_none":   {Given{EnvStrategy: EnvNone, EnvFiles: []string{"apps/api/.env"}}, PathEnvFiles, "init.env_files_with_none"},
		"strategy_no_files": {Given{EnvStrategy: "main", EnvFiles: []string{}}, PathEnvFiles, "init.env_files_none"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			problems := questions(facts(nil), c.given).Problems()

			require.Len(t, problems, 1)
			assert.Equal(t, c.path, problems[0].Path)
			assert.Equal(t, c.code, problems[0].Code)
		})
	}
}

// Review of #282: strategy = "none" written in config.toml is no strategy every command reads; a re-init said nothing to change.
func TestNoneWrittenInTheConfigIsAProblemOfTheStrategy(t *testing.T) {
	s := saved()
	s.Config.Env.Strategy = EnvNone

	problems := questions(facts(s), Given{}).Problems()
	fixed := questions(facts(s), Given{EnvStrategy: "main"}).Problems()

	require.Len(t, problems, 1)
	assert.Equal(t, PathEnvStrategy, problems[0].Path)
	assert.Equal(t, domain.CodeOneOf, problems[0].Code)
	assert.Empty(t, fixed)
}

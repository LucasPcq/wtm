package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheAppIsWiredOntoTheRealAdapters(t *testing.T) {
	main := configured(t)
	root := filepath.Dir(main)
	t.Setenv("HOME", t.TempDir())

	outcome, err := wire(wireParams{MainPath: main, StateDir: filepath.Join(main, ".git", "wtm"), CommonDir: filepath.Join(main, ".git")}).Create.Run(t.Context(), create.Request{Branches: []string{"feat/x"}}, domain.RunOptions{Report: func(domain.Event) {}})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status, outcome.Result.Items[0].Err)
	assert.NotEmpty(t, outcome.Result.Items[0].Detail.Metadata.CreatedAt)
	assert.Len(t, outcome.Op, 16)
	assert.DirExists(t, filepath.Join(root, ".trees", "feat-x"))
}

func TestExtractIsWiredOntoTheRealAdapters(t *testing.T) {
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	gittest.Build(t, gittest.FakeParams{Main: main, Branch: "main"})
	stateDir := filepath.Join(main, ".git", "wtm")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "config.toml"), []byte("[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(main, "notes.txt"), []byte("notes\n"), 0o644))
	t.Setenv("HOME", t.TempDir())

	outcome, err := wire(wireParams{MainPath: main, StateDir: stateDir, CommonDir: filepath.Join(main, ".git")}).Extract.Run(t.Context(), extract.Request{Source: "main", Files: []string{"notes.txt"}, To: "feat/notes"}, domain.RunOptions{Report: func(domain.Event) {}})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status, outcome.Result.Items[0].Err)
	assert.FileExists(t, filepath.Join(root, ".trees", "feat-notes", "notes.txt"))
	assert.NoFileExists(t, filepath.Join(main, "notes.txt"))
}

func TestEventsAreFiledUnderGitsCommonDirWhereverTheStateLives(t *testing.T) {
	params := wireParams{MainPath: "/repo", StateDir: "/elsewhere/state", CommonDir: "/repo/.git"}

	assert.Equal(t, "/repo/.git", params.publisherParams(domain.Operation{ID: "op-1"}).CommonDir)
}

func TestAnUndoLeftBehindIsRemovedByTheLegacyClean(t *testing.T) {
	assert.Equal(t, []string{"wtm", "clean", "feat/x", "--force"}, commandLine(wire(wireParams{}).Create.Provision.LeftBehind("feat/x")))
}

func TestAPullRequestWithNoNumberIsOpenedByItsURL(t *testing.T) {
	assert.Equal(t, []string{"gh", "pr", "view", "https://github.com/acme/acme/pull/12", "--web"}, commandLine(domain.FollowUp{URL: "https://github.com/acme/acme/pull/12"}))
}

func TestTwoExecutionsInOneProcessKeepTheirOwnRepositoryAndCorrelationID(t *testing.T) {
	t.Setenv(cli.CorrelationIDEnv, "from-env")
	t.Setenv("HOME", t.TempDir())
	t.Setenv(stateDirEnv, "")
	var published []events.PublisherParams
	creates := map[string]create.UseCase{}
	commonDirs := map[string]string{}
	for _, id := range []string{"a", "b"} {
		params, err := locate(t.Context(), configured(t))
		require.NoError(t, err)
		useCase := wire(params).Create
		useCase.Operations.Publisher = recorded{wireParams: params, published: &published}
		creates[id], commonDirs[id] = useCase, params.CommonDir
	}
	require.NotEqual(t, commonDirs["a"], commonDirs["b"])

	for _, run := range []struct{ id, branch string }{{"a", "feat/1"}, {"b", "feat/1"}, {"a", "feat/2"}} {
		outcome, err := creates[run.id].Run(t.Context(), create.Request{Branches: []string{run.branch}}, domain.RunOptions{CorrelationID: run.id, Report: func(domain.Event) {}})
		require.NoError(t, err)
		require.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status, outcome.Result.Items[0].Err)

		last := published[len(published)-1]
		assert.Equal(t, outcome.Op, last.OpID)
		assert.Equal(t, run.id, last.CorrelationID)
		assert.Equal(t, commonDirs[run.id], last.CommonDir)
	}
	assert.Len(t, published, 3)
}

func TestEachActionOfListIsTheCommandLineThatCarriesItOut(t *testing.T) {
	branch := list.Detail{Branch: "feat/a", Path: "/repo/.trees/feat-a", PR: &list.PR{Number: 12, URL: "https://github.com/acme/acme/pull/12"}}
	detached := list.Detail{Path: "/repo/.trees/detached"}
	cases := map[string]struct {
		action list.Action
		detail list.Detail
		argv   []string
	}{
		"go":          {action: list.ActionGo, detail: branch, argv: []string{"wtm", "go", "feat/a"}},
		"go_detached": {action: list.ActionGo, detail: detached, argv: []string{"cd", "/repo/.trees/detached"}},
		"open_pr":     {action: list.ActionOpenPR, detail: branch, argv: []string{"gh", "pr", "view", "12", "--web"}},
		"run_up":      {action: list.ActionRunUp, detail: branch, argv: []string{"wtm", "run", "up", "feat/a"}},
		"run_down":    {action: list.ActionRunDown, detail: branch, argv: []string{"wtm", "run", "down", "feat/a"}},
		"logs":        {action: list.ActionLogs, detail: branch, argv: []string{"wtm", "run", "logs", "feat/a"}},
		"clean":       {action: list.ActionClean, detail: branch, argv: []string{"wtm", "clean", "feat/a"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.argv, commandLine(list.FollowUpOf(c.action, c.detail)))
		})
	}
}

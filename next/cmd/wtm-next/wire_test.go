package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheAppIsWiredOntoTheRealAdapters(t *testing.T) {
	main := configured(t)
	root := filepath.Dir(main)
	ownConfigDir(t)

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
	ownConfigDir(t)

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

func TestPortsLeftUnsettledAreSettledByTheLegacyEnv(t *testing.T) {
	assert.Equal(t, []string{"wtm", "env", "feat/x"}, commandLine(wire(wireParams{}).Create.Provision.Settle("feat/x")))
}

func TestARepositoryWithNoWtmConfigIsFollowedUpByTheLegacyInit(t *testing.T) {
	uninitialized := gittest.InitRepo(t)
	ownConfigDir(t)
	for _, command := range catalogue() {
		t.Run(command.Name, func(t *testing.T) {
			got := command.Execute(t.Context(), api.Call{Dir: uninitialized})

			require.NotNil(t, got.Envelope.Err)
			assert.Equal(t, command.Name+domain.ConfigNotFound, got.Envelope.Err.Code)
			require.NotNil(t, got.Envelope.Err.FollowUp)
			assert.Equal(t, []string{"wtm", "init"}, commandLine(*got.Envelope.Err.FollowUp))
		})
	}
	source, err := openEvents(t.Context(), uninitialized)
	require.NoError(t, err)
	assert.Equal(t, initialize(), source.Repository.Init)
}

func TestTwoExecutionsInOneProcessKeepTheirOwnRepositoryAndCorrelationID(t *testing.T) {
	t.Setenv(cli.CorrelationIDEnv, "from-env")
	ownConfigDir(t)
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

func TestEveryFollowUpIsSpelledWhole(t *testing.T) {
	pr := &list.PR{Number: 12, URL: "https://github.com/acme/acme/pull/12"}
	actions := list.Actions(list.Detail{Branch: "feat/a", Path: "/repo/.trees/feat-a", PR: pr})
	dryRun := leftBehind("feat/x")
	dryRun.DryRun = true
	fixes := status.Problems(status.ProblemsParams{
		Branch:  "feat/a",
		Merged:  true,
		Missing: []status.Missing{{Target: ".env"}, {Target: ".env", From: status.FromExample}, {Target: ".env", From: status.FromMain}, {Target: ".env", From: status.FromNowhere}},
		Pending: true,
		Jobs:    []status.Job{{Name: "api", State: run.Crashed}, {Name: "db", State: run.Crashed, Owner: &status.Owner{Branch: "main"}}},
	})
	cases := map[string]struct {
		followUp domain.FollowUp
		argv     []string
	}{
		"go":          {followUp: actions[0], argv: []string{"wtm", "go", "feat/a"}},
		"open_pr":     {followUp: actions[1], argv: []string{"gh", "pr", "view", "12", "--web"}},
		"run_up":      {followUp: actions[2], argv: []string{"wtm", "run", "up", "feat/a"}},
		"run_down":    {followUp: actions[3], argv: []string{"wtm", "run", "down", "feat/a"}},
		"logs":        {followUp: actions[4], argv: []string{"wtm", "run", "logs", "feat/a"}},
		"clean":       {followUp: actions[5], argv: []string{"wtm", "clean", "feat/a"}},
		"go_detached": {followUp: list.Actions(list.Detail{Path: "/repo/.trees/detached"})[0], argv: []string{"cd", "/repo/.trees/detached"}},
		"enter":       {followUp: domain.Enter("feat/notes"), argv: []string{"wtm", "go", "feat/notes"}},
		"left_behind": {followUp: leftBehind("feat/x"), argv: []string{"wtm", "clean", "feat/x", "--force"}},
		"init":        {followUp: *initialize(), argv: []string{"wtm", "init"}},
		"settle":      {followUp: settle("feat/x"), argv: []string{"wtm", "env", "feat/x"}},
		"proxy":       {followUp: proxyStatus(), argv: []string{"wtm", "run", "proxy", "status"}},
		"dry_run":     {followUp: dryRun, argv: []string{"wtm", "clean", "feat/x", "--force", "--dry-run"}},
		"pr_url_only": {followUp: domain.FollowUp{URL: pr.URL}, argv: []string{"gh", "pr", "view", pr.URL, "--web"}},
		"merged":      {followUp: *fixes[0].Fix, argv: []string{"wtm", "clean", "feat/a"}},
		"env":         {followUp: *fixes[1].Fix, argv: []string{"wtm", "env", "feat/a"}},
		"env_example": {followUp: *fixes[2].Fix, argv: []string{"wtm", "env", "feat/a", "--from", "example"}},
		"env_main":    {followUp: *fixes[3].Fix, argv: []string{"wtm", "env", "feat/a", "--from", "main"}},
		"config_edit": {followUp: *fixes[4].Fix, argv: []string{"wtm", "config", "edit"}},
		"isolation":   {followUp: *fixes[5].Fix, argv: []string{"wtm", "env", "feat/a", "--isolation", "isolated"}},
		"run_start":   {followUp: *fixes[6].Fix, argv: []string{"wtm", "run", "start", "feat/a", "--job", "api", "--detach"}},
		"shared":      {followUp: *fixes[7].Fix, argv: []string{"wtm", "run", "start", "main", "--job", "db", "--detach"}},
		"tree_go":     {followUp: tree.Actions(tree.Detail{Branch: "feat/a", Rebasing: true}, false)[0], argv: []string{"wtm", "go", "feat/a"}},
		"tree_clean":  {followUp: tree.Actions(tree.Detail{Branch: "feat/a"}, true)[0], argv: []string{"wtm", "clean", "feat/a"}},
		"tree_sync":   {followUp: tree.Actions(tree.Detail{Branch: "feat/a", NeedsSync: true}, false)[0], argv: []string{"wtm", "sync", "feat/a"}},
		"behind":      {followUp: *status.Problems(status.ProblemsParams{Branch: "feat/a", Parent: "feat/b", Lacks: 1})[0].Fix, argv: []string{"wtm", "sync", "feat/a"}},
		"reparent":    {followUp: domain.FollowUp{Command: "reparent", Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/a"}}, {Path: "to", Values: []string{"feat/b"}}}}, argv: []string{"wtm", "reparent", "feat/a", "--to", "feat/b"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			argv, unspelled := spelled(c.followUp)

			assert.Equal(t, c.argv, argv)
			assert.Empty(t, unspelled)
		})
	}
}

func TestAnAnswerTheCommandLineCannotSpellIsReported(t *testing.T) {
	followUp := domain.Enter("feat/a")
	followUp.Answers = append(followUp.Answers, domain.Answered{Path: "profile", Values: []string{"web"}})

	_, unspelled := spelled(followUp)

	assert.Equal(t, []string{"profile"}, unspelled)
}

func TestAFollowUpWithNothingToRunIsReported(t *testing.T) {
	argv, unspelled := spelled(domain.FollowUp{Code: "list.open_pr"})

	assert.Empty(t, argv)
	assert.Equal(t, []string{"url"}, unspelled)
}

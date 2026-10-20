package main

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const commandPrefix = "github.com/LucasPcq/wtm/next/internal/command/"

func TestEachRegisteredCommandIsInTheCatalogueOrItsStreamsAndWired(t *testing.T) {
	registered := registeredCommands(t)
	var listed, streamed []string
	for _, command := range catalogue() {
		// a command's folder spells its name without dashes: command/fastforward is fast-forward
		listed = append(listed, strings.ReplaceAll(command.Name, "-", ""))
	}
	for _, stream := range streams() {
		streamed = append(streamed, stream.Name)
		assert.NotNil(t, stream.Watch, stream.Name)
	}
	// shortcut: a non-zero use case is taken as wired, not checked field by field; a test that runs each command would prove more
	wired := map[string]bool{}
	app := reflect.ValueOf(wire(wireParams{MainPath: t.TempDir()}))
	for i := range app.NumField() {
		if name, ok := strings.CutPrefix(app.Field(i).Type().PkgPath(), commandPrefix); ok && !app.Field(i).IsZero() {
			wired[name] = true
		}
	}

	require.NotEmpty(t, registered)
	assert.Equal(t, streamsListed, streamed, "catalogue.go streams a command the list of streams does not name, or the reverse")
	assert.ElementsMatch(t, registered, append(listed, streamed...), "main.go registers a command catalogue.go does not list, or the reverse")
	for _, name := range registered {
		// a stream follows every repository: wire.go builds it once, not per repository in app
		if !slices.Contains(streamsListed, name) {
			assert.True(t, wired[name], "%s is registered in main.go but wire.go builds no %s.UseCase", name, name)
		}
	}
}

func TestNoCommandAFollowUpNamesHasMigrated(t *testing.T) {
	streams, _, _, _ := output.Test()
	root := cli.NewRoot(version, streams, commands(streams, cli.Env{}))
	named := []string{leftBehind("feat/a").Command}
	for _, action := range list.Actions(list.Detail{Branch: "feat/a"}) {
		named = append(named, action.Command)
	}
	for _, detail := range []tree.Detail{{Branch: "feat/a", Rebasing: true}, {Branch: "feat/a", PR: &tree.PR{State: worktree.PullRequestMerged}}, {Branch: "feat/a", NeedsSync: true}} {
		named = append(named, tree.Actions(detail, "main")[0].Command)
	}
	fixes := status.Problems(status.ProblemsParams{Subject: "feat/a", Missing: []status.Missing{{Target: ".env"}, {Target: ".env", From: status.FromNowhere}}, Jobs: []status.Job{{Name: "api", State: run.Crashed}}})
	for _, problem := range fixes {
		named = append(named, problem.Fix.Command)
	}
	for _, command := range named {
		require.NotEmpty(t, command)
		_, listed := catalogue().Find(command)

		assert.False(t, listed || runs(root, strings.Fields(command)), "%s has migrated: check the answers its follow-ups give against its schema and spell its command line from it in commandLine", command)
	}
}

func TestTheGuardTellsARunSubcommandFromAnother(t *testing.T) {
	root, run := &cobra.Command{Use: "wtm-next"}, &cobra.Command{Use: "run"}
	run.AddCommand(&cobra.Command{Use: "init", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(run)

	assert.True(t, runs(root, []string{"run", "init"}))
	assert.False(t, runs(root, []string{"run", "up"}))
	assert.False(t, runs(root, []string{"clean"}))
}

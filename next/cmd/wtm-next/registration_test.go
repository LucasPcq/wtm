package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const commandPrefix = "github.com/LucasPcq/wtm/next/internal/command/"

func TestEachRegisteredCommandIsInTheCatalogueAndWired(t *testing.T) {
	registered := registeredCommands(t)
	var listed []string
	for _, command := range catalogue() {
		// a command's folder spells its name without dashes: command/fastforward is fast-forward
		listed = append(listed, strings.ReplaceAll(command.Name, "-", ""))
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
	assert.ElementsMatch(t, registered, listed, "main.go registers a command catalogue.go does not list, or the reverse")
	for _, name := range registered {
		assert.True(t, wired[name], "%s is registered in main.go but wire.go builds no %s.UseCase", name, name)
	}
}

func TestNoCommandListNamesInAFollowUpHasMigrated(t *testing.T) {
	streams, _, _, _ := output.Test()
	root := cli.NewRoot(version, streams, commands(streams, cli.Env{}))
	for _, action := range []list.Action{list.ActionGo, list.ActionRunUp, list.ActionRunDown, list.ActionLogs, list.ActionClean} {
		argv := listCommands(action, list.Detail{Branch: "feat/a"})
		require.GreaterOrEqual(t, len(argv), 3, action)
		require.Equal(t, "wtm", argv[0], action)
		path := argv[1 : len(argv)-1] // the words between wtm and the worktree
		_, listed := catalogue().Find(strings.Join(path, " "))

		assert.False(t, listed || runs(root, path), "%s has migrated: build list's follow-up from its schema (command + request) instead of a hard-coded argv", strings.Join(path, " "))
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

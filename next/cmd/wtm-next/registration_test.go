package main

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"

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
	registered := registeredCommands(t)
	for _, action := range []list.Action{list.ActionGo, list.ActionRunUp, list.ActionRunDown, list.ActionLogs, list.ActionClean} {
		argv := listCommands(action, list.Detail{Branch: "feat/a"})
		require.GreaterOrEqual(t, len(argv), 2, action)
		require.Equal(t, "wtm", argv[0], action)
		_, listed := catalogue().Find(argv[1])

		assert.False(t, listed || slices.Contains(registered, argv[1]), "%s has migrated: build list's follow-up from its schema (command + request) instead of a hard-coded argv", argv[1])
	}
}

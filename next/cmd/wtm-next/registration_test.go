package main

import (
	"reflect"
	"strings"
	"testing"

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

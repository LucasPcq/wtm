package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
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
		listed = append(listed, command.Name)
	}
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

// registeredCommands reads main.go for the commands it registers: the name of each command/<name>/cli whose New it calls.
func registeredCommands(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	require.NoError(t, err)
	clis := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		require.NoError(t, err)
		name, ok := strings.CutPrefix(path, commandPrefix)
		name, isCLI := strings.CutSuffix(name, "/cli")
		if ok && isCLI && spec.Name != nil {
			clis[spec.Name.Name] = name
		}
	}
	var names []string
	ast.Inspect(file, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "New" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && clis[pkg.Name] != "" {
			names = append(names, clis[pkg.Name])
		}
		return true
	})
	return names
}

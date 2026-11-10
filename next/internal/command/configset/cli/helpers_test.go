package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/configset"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func runSet(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	dir := t.TempDir()
	open := w.opener(t, dir)
	got := formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		return New(streams, w.env(), open, formtest.CommandLine)
	}, append([]string{"set"}, words...)...)
	got.Stdout = strings.ReplaceAll(got.Stdout, dir, "<state>")
	got.Stderr = strings.ReplaceAll(got.Stderr, dir, "<state>")
	configs[t.Name()] = dir
	return got
}

// configs are the state directory of each test's last run, by test name.
var configs = map[string]string{}

// written is config.toml as the last run of the test left it.
func written(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(configs[t.Name()], "config.toml"))
	require.NoError(t, err)
	return string(data)
}

// wizard starts config set's wizard on w's config.toml with the request given.
func wizard(t *testing.T, w world, given configset.Request) *formtest.Driver {
	t.Helper()
	app, err := w.opener(t, t.TempDir())(t.Context(), "/acme")
	require.NoError(t, err)
	bound := newForm(nil)
	bound.Bind(&cobra.Command{}, &uicli.Run{CommandLine: formtest.CommandLine})
	s := bound.Session(t.Context(), app, given, domain.RunOptions{})
	return formtest.Start(form.New(t.Context(), s.Script()), tea.WindowSizeMsg{Width: 100, Height: 40})
}

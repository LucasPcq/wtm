package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfResolve(t *testing.T) {
	got := runResolve(t, world{}, "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "resolve_help", got.Stdout)
}

func TestResolvePrintsThePathAloneOnStdoutEvenQuiet(t *testing.T) {
	for name, words := range map[string][]string{"text": {"feat/login"}, "quiet": {"feat/login", "-q"}, "part": {"login"}} {
		t.Run(name, func(t *testing.T) {
			got := runResolve(t, world{}, words...)

			require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
			assert.Equal(t, "/repo/.trees/feat-login\n", got.Stdout)
			assert.Empty(t, got.Stderr)
		})
	}
}

func TestWithNoWorktreeNamedAndNoTerminalTheOneYouAreInIsResolved(t *testing.T) {
	got := runResolve(t, world{Cwd: "/repo/.trees/fix-nav"})

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Equal(t, "/repo/.trees/fix-nav\n", got.Stdout)
}

func TestAWorktreeOnNoBranchIsResolvedByItsPath(t *testing.T) {
	got := runResolve(t, world{}, "/repo/.trees/detached")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Equal(t, "/repo/.trees/detached\n", got.Stdout)
}

func TestAWorktreeWhoseDirectoryIsGoneIsStillItsPath(t *testing.T) {
	got := runResolve(t, world{Gone: true}, "fix/nav", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_resolve_gone", got.Stdout)
}

func TestTheJSONOfResolveIsTheV2EnvelopeOfARead(t *testing.T) {
	got := runResolve(t, world{}, "login", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_resolve", got.Stdout)
	doc := formtest.Decoded[struct{}](t, got.Stdout)
	assert.Empty(t, doc.Op)
	assert.Equal(t, resolve.Name, doc.Command)
}

func TestARefusalLeavesStdoutEmptyAndExitsUsage(t *testing.T) {
	cases := map[string]struct {
		words  []string
		stderr string
	}{
		"not_found": {words: []string{"nope"}, stderr: "nope names no worktree"},
		"ambiguous": {words: []string{"log"}, stderr: "log matches several worktrees: feat/login, feat/logout"},
		"two_words": {words: []string{"feat", "login"}, stderr: "accepts at most 1 arg"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := runResolve(t, world{}, c.words...)

			assert.Equal(t, uicli.ExitUsage, got.Code)
			assert.Empty(t, got.Stdout)
			assert.Contains(t, got.Stderr, c.stderr)
		})
	}
}

func TestARefusalWithJSONIsTheEnvelopeOnStdout(t *testing.T) {
	cases := map[string]struct {
		w     world
		words []string
		code  int
	}{
		"not_found":        {words: []string{"nope"}, code: uicli.ExitUsage},
		"ambiguous":        {words: []string{"log"}, code: uicli.ExitUsage},
		"config_not_found": {w: world{ConfigErr: project.ErrConfigNotFound}, code: uicli.ExitConfigNotFound},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := runResolve(t, c.w, append(c.words, "--output", "json")...)

			assert.Equal(t, c.code, got.Code)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestOnATerminalWithStdoutCapturedTheWorktreeIsPicked(t *testing.T) {
	cases := map[string]struct {
		w     world
		words []string
		path  string
	}{
		"ambiguous_among_matches": {words: []string{"log"}, path: "/repo/.trees/feat-login"},
		"none_on_the_current":     {w: world{Cwd: "/repo/.trees/fix-nav"}, path: "/repo/.trees/fix-nav"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := runOn(t, captured, c.w, c.words...)

			require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
			assert.Equal(t, c.path+"\n", got.Stdout)
		})
	}
}

func TestLeavingThePickerWithStdoutCapturedIsCancelledAndPrintsNothing(t *testing.T) {
	got := runOn(t, func(streams *output.Streams) {
		captured(streams)
		streams.In = formtest.Escapes{}
	}, world{}, "log")

	assert.Equal(t, uicli.ExitCancelled, got.Code)
	assert.Empty(t, got.Stdout)
}

func TestThePickerOffersEveryWorktreeAndLeavingItCancels(t *testing.T) {
	app, err := world{}.open(t.Context(), mainPath)
	require.NoError(t, err)
	q, err := app.Questions(t.Context(), resolve.Request{Caller: domain.Caller{Cwd: "/repo/.trees/feat-logout"}})
	require.NoError(t, err)
	field, ok := q.Form().Fields[0].(domain.SelectField)
	require.True(t, ok)
	subject := &form.Subject{Field: field, Words: words(resolve.Questions{}, field)}

	d := formtest.Start(form.New(t.Context(), form.SubjectScript(subject)), tea.WindowSizeMsg{Width: 100, Height: 30})
	formtest.Golden(t, "picker", d.Screen())
	d.Keys("esc")

	assert.Equal(t, form.Cancelled, d.Wizard.Ending())
}

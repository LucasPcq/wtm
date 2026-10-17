package form

import (
	"cmp"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func TestARunAsksOnlyWhenTheUserCanSeeTheQuestionsAndAnswerThem(t *testing.T) {
	cases := map[string]struct {
		In, Err, Dumb, Yes bool
		Output, Expect     string
		Asks               bool
	}{
		"stdin and stderr terminals": {In: true, Err: true, Asks: true},
		"stdin piped":                {Err: true},
		"stderr redirected":          {In: true},
		"TERM=dumb":                  {In: true, Err: true, Dumb: true},
		"--yes":                      {In: true, Err: true, Yes: true},
		"--output json":              {In: true, Err: true, Output: uicli.OutputJSON},
		"--expect-fingerprint":       {In: true, Err: true, Expect: "print"},
	}
	for name, c := range cases {
		streams, _, _, _ := output.Test()
		streams.SetStdinTerminal(c.In)
		streams.Err.SetTerminal(c.Err)
		streams.SetDumbTerminal(c.Dumb)
		command := fakeCommand()
		command.io, command.yes, command.expect = &uicli.Run{Streams: streams, Output: cmp.Or(c.Output, uicli.OutputText)}, c.Yes, c.Expect

		assert.Equal(t, c.Asks, command.interactive(), name)
	}
}

func bound(t *testing.T, args ...string) (*Command[request, questions, outcome], *cobra.Command) {
	t.Helper()
	command, cmd := fakeCommand(), &cobra.Command{Use: "fake"}
	command.Bind(cmd, &uicli.Run{})
	require.NoError(t, cmd.ParseFlags(args))
	return command, cmd
}

func TestEachFieldIsAFlagOfItsKindAndTheSharedFlagsFollow(t *testing.T) {
	_, cmd := bound(t)
	types := map[string]string{}
	cmd.Flags().VisitAll(func(f *pflag.Flag) { types[f.Name] = f.Value.Type() })

	assert.Equal(t, map[string]string{
		"from": "string", "mode": "string", "ff": "bool", "keep": "bool", "files": "stringSlice", "name": "string",
		"remember": "stringArray", "ask": "bool", "yes": "bool", "output": "string", "dry-run": "bool", "expect-fingerprint": "string",
	}, types)
	assert.Equal(t, "The source", cmd.Flags().Lookup("from").Usage)
	assert.Equal(t, "The answers to always use in this repository: mode, ff (repeatable)", cmd.Flags().Lookup("remember").Usage)
	assert.Equal(t, "y", cmd.Flags().Lookup("yes").Shorthand)
}

func TestTheFlagsGivenAndTheArgumentsAnswerTheRequest(t *testing.T) {
	command, cmd := bound(t, "--from", "main", "--ff=false", "--files", "a.txt,b.txt", "--remember", "mode", "--ask")

	req := command.request(cmd, []string{"feat/a", " feat/b "})

	assert.Equal(t, request{"branches": {"feat/a", "feat/b"}, "from": {"main"}, "ff": {"false"}, "files": {"a.txt", "b.txt"}, domain.PathRemember: {"mode"}, domain.PathAsk: {"true"}}, req)
}

func TestNoFlagGivenIsAnEmptyRequest(t *testing.T) {
	command, cmd := bound(t)

	assert.Empty(t, command.request(cmd, nil))
}

func TestTheCommandLineSpellsThePositionalWordsThenTheFlagsSortedThenAsk(t *testing.T) {
	req := request{"branches": {"feat/a", "it's"}, "name": {"two words"}, "ff": {"false"}, "keep": {"true"}, "files": {"a.txt", "b.txt"}, "from": {"main"}, domain.PathRemember: {"mode"}, domain.PathAsk: {"true"}}

	argv := fakeCommand().Argv("wtm-next fake", req)

	assert.Equal(t, []string{"wtm-next", "fake", "feat/a", "it's", "--ff=false", "--files", "a.txt", "--files", "b.txt", "--from", "main", "--keep", "--name", "two words", "--remember", "mode", "--ask"}, argv)
	assert.Equal(t, `wtm-next fake feat/a 'it'\''s' --ff=false --files a.txt --files b.txt --from main --keep --name 'two words' --remember mode --ask`, output.Line(argv))
}

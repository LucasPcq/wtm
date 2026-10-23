package form_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAReadBindsItsFieldsAndOutputOnly(t *testing.T) {
	q, _ := fakeQuery()
	cmd := queryCommand(q, &uicli.Run{})
	types := map[string]string{}
	cmd.Flags().VisitAll(func(f *pflag.Flag) { types[f.Name] = f.Value.Type() })

	assert.Equal(t, map[string]string{
		"from": "string", "mode": "string", "ff": "bool", "keep": "bool", "files": "stringSlice", "name": "string", "output": "string",
	}, types)
}

func TestAReadPicksOnlyWhenStdinStdoutAndStderrAreTerminals(t *testing.T) {
	cases := map[string]struct {
		Out, Picks bool
		Args       []string
	}{
		"all_terminals": {Out: true, Picks: true},
		"stdout_piped":  {Out: false},
		"quiet":         {Out: true, Args: []string{"--quiet"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			q, picked := fakeQuery()
			streams, _, stdout, _ := output.Test()
			streams.SetStdinTerminal(true)
			streams.Err.SetTerminal(true)
			streams.Out.SetTerminal(c.Out)

			err := launched(t, q, &uicli.Run{Command: "fake", Streams: streams, Env: uicli.Env{Cwd: "/repo"}, CommandLine: formtest.CommandLine}, c.Args...)

			require.NoError(t, err)
			assert.Equal(t, c.Picks, *picked > 0)
			switch {
			case c.Picks:
				assert.Contains(t, stdout.String(), "→ wtm go feat/x   Go there")
			case len(c.Args) == 0:
				assert.Contains(t, stdout.String(), "read 1")
			default:
				assert.Empty(t, stdout.String())
			}
		})
	}
}

func TestAReadRefusedWithJSONIsItsEnvelopeOnStdout(t *testing.T) {
	q, _ := fakeQuery()
	q.Open = func(context.Context, string) (form.Reader[request, questions, read], error) {
		return nil, &domain.Error{Kind: domain.KindPrecondition, Code: "fake.config_not_found", Message: "no wtm config"}
	}
	streams, _, stdout, stderr := output.Test()

	err := launched(t, q, &uicli.Run{Command: "fake", Streams: streams, Env: uicli.Env{Cwd: "/repo"}, CommandLine: formtest.CommandLine}, "--output", "json")

	require.Error(t, err)
	assert.Empty(t, stderr.String())
	assert.JSONEq(t, `{"v": 2, "command": "fake", "ok": false, "follow_ups": [], "warnings": [],
		"error": {"kind": "precondition", "code": "fake.config_not_found", "message": "no wtm config"}}`, stdout.String())
}

func TestAReadThatFailedOnASubjectDrawsItAndFails(t *testing.T) {
	q, _ := fakeQuery()
	q.Open = func(context.Context, string) (form.Reader[request, questions, read], error) {
		return reader{read: read{Items: []domain.Unit[struct{}]{{Subject: "a", Status: domain.StatusFailed, Err: &domain.Error{Kind: domain.KindInternal, Code: "fake.unreadable", Message: "couldn't read a"}}}}}, nil
	}
	q.Draw = func(w io.Writer, got read, _ request, _ output.View) {
		_, _ = io.WriteString(w, "drew "+got.Items[0].Subject+"\n")
	}
	streams, _, stdout, _ := output.Test()

	err := launched(t, q, &uicli.Run{Command: "fake", Streams: streams, Env: uicli.Env{Cwd: "/repo"}, CommandLine: formtest.CommandLine})

	failed, ok := errors.AsType[*uicli.Failed](err)
	require.True(t, ok, err)
	assert.Equal(t, "fake.unreadable", failed.Err.Code)
	assert.Contains(t, stdout.String(), "drew a")
}

func TestAPickKeepsTheFailureOfTheReadInItsExitCode(t *testing.T) {
	q, _ := fakeQuery()
	q.Open = func(context.Context, string) (form.Reader[request, questions, read], error) {
		return reader{read: read{Items: []domain.Unit[struct{}]{{Subject: "a", Status: domain.StatusFailed, Err: &domain.Error{Kind: domain.KindInternal, Code: "fake.unreadable", Message: "couldn't read a"}}}}}, nil
	}
	streams, _, stdout, _ := output.Test()
	streams.SetStdinTerminal(true)
	streams.Err.SetTerminal(true)
	streams.Out.SetTerminal(true)

	err := launched(t, q, &uicli.Run{Command: "fake", Streams: streams, Env: uicli.Env{Cwd: "/repo"}, CommandLine: formtest.CommandLine})

	failed, ok := errors.AsType[*uicli.Failed](err)
	require.True(t, ok, err)
	assert.Equal(t, "fake.unreadable", failed.Err.Code)
	assert.Contains(t, stdout.String(), "→ wtm go feat/x")
}

func TestAReadPrintsAFormatOfItsOwnUnframedEvenQuiet(t *testing.T) {
	q, _ := fakeQuery()
	q.Formats = map[string]func(io.Writer, read){"mermaid": func(w io.Writer, got read) {
		_, _ = io.WriteString(w, "graph of "+got.Items[0].Subject+"\n")
	}}
	streams, _, stdout, stderr := output.Test()
	run := &uicli.Run{Command: "fake", Streams: streams, Env: uicli.Env{Cwd: "/repo"}, CommandLine: formtest.CommandLine}

	err := launched(t, q, run, "--output", "mermaid", "--quiet")

	require.NoError(t, err)
	assert.Equal(t, "graph of feat/x\n", stdout.String())
	assert.Empty(t, stderr.String())
	assert.Equal(t, "Output format: text, json or mermaid", queryCommand(q, &uicli.Run{}).Flags().Lookup("output").Usage)
}

func TestAFormatNoneOfTheReadsIsRefusedNamingThem(t *testing.T) {
	q, _ := fakeQuery()
	q.Formats = map[string]func(io.Writer, read){"mermaid": func(io.Writer, read) {}}
	streams, _, _, stderr := output.Test()

	err := launched(t, q, &uicli.Run{Command: "fake", Streams: streams, Env: uicli.Env{Cwd: "/repo"}, CommandLine: formtest.CommandLine}, "--output", "dot")

	require.Error(t, err)
	assert.Contains(t, stderr.String(), `invalid --output "dot": expected one of text, json, mermaid`)
}

func TestARefusalInAFormatOfTheReadsIsText(t *testing.T) {
	q, _ := fakeQuery()
	q.Formats = map[string]func(io.Writer, read){"mermaid": func(io.Writer, read) {}}
	q.Open = func(context.Context, string) (form.Reader[request, questions, read], error) {
		return nil, &domain.Error{Kind: domain.KindPrecondition, Code: "fake.config_not_found", Message: "no wtm config"}
	}
	streams, _, stdout, stderr := output.Test()

	err := launched(t, q, &uicli.Run{Command: "fake", Streams: streams, Env: uicli.Env{Cwd: "/repo"}, CommandLine: formtest.CommandLine}, "--output", "mermaid")

	require.Error(t, err)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "no wtm config")
}

func TestAReadAsksTheSubjectItsRequestLeavesOpenBeforeReading(t *testing.T) {
	cases := map[string]struct {
		Args     []string
		Terminal bool
		Read     string
		Asked    int
	}{
		"picked_on_a_terminal":  {Terminal: true, Read: "main", Asked: 1},
		"given":                 {Terminal: true, Args: []string{"feat/x"}, Read: "feat/x"},
		"ruled_out":             {Terminal: true, Args: []string{"--all"}},
		"unattended_left_alone": {},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			q, got, asked := subjectQuery()
			streams, _, _, _ := output.Test()
			streams.In = enters{}
			streams.SetStdinTerminal(c.Terminal)
			streams.Err.SetTerminal(c.Terminal)
			streams.Out.SetTerminal(c.Terminal)

			err := launchedSubject(t, q, &uicli.Run{Command: "fake", Streams: streams, Env: uicli.Env{Cwd: "/repo"}, CommandLine: formtest.CommandLine}, c.Args...)

			require.NoError(t, err)
			assert.Equal(t, c.Read, strings.Join((*got)["worktree"], ""))
			assert.Equal(t, c.Asked, *asked, "the questions are read only when the subject is left to ask")
		})
	}
}

func TestTheSubjectIsAskedOnItsValueAndEachOptionReadsAsItsLabel(t *testing.T) {
	field, ok := subjects{}.Form().Fields[0].(domain.SelectField)
	require.True(t, ok)
	subject := &form.Subject{Field: field}
	d := start(form.New(t.Context(), form.SubjectScript(subject)))

	assert.Contains(t, d.Screen(), "▌› main\n   feat/x (detached)")
	d.Keys("down", "enter")

	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, "feat/x", subject.Picked())
}

func TestLeavingTheSubjectCancelsTheRead(t *testing.T) {
	field, ok := subjects{}.Form().Fields[0].(domain.SelectField)
	require.True(t, ok)
	d := start(form.New(t.Context(), form.SubjectScript(&form.Subject{Field: field})))

	d.Keys("esc")

	assert.Equal(t, form.Cancelled, d.Wizard.Ending())
}

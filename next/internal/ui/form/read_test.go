package form_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
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

			err := launched(t, q, &uicli.Run{Command: "fake", Streams: streams}, c.Args...)

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
	q.Open = func(context.Context, string) (form.Reader[request, read], error) {
		return nil, &domain.Error{Kind: domain.KindPrecondition, Code: "fake.config_not_found", Message: "no wtm config"}
	}
	streams, _, stdout, stderr := output.Test()

	err := launched(t, q, &uicli.Run{Command: "fake", Streams: streams}, "--output", "json")

	require.Error(t, err)
	assert.Empty(t, stderr.String())
	assert.JSONEq(t, `{"v": 2, "command": "fake", "ok": false, "follow_ups": [], "warnings": [],
		"error": {"kind": "precondition", "code": "fake.config_not_found", "message": "no wtm config"}}`, stdout.String())
}

func TestAReadThatFailedOnASubjectDrawsItAndFails(t *testing.T) {
	q, _ := fakeQuery()
	q.Open = func(context.Context, string) (form.Reader[request, read], error) {
		return reader{read: read{Items: []domain.Unit[struct{}]{{Subject: "a", Status: domain.StatusFailed, Err: &domain.Error{Kind: domain.KindInternal, Code: "fake.unreadable", Message: "couldn't read a"}}}}}, nil
	}
	q.Draw = func(w io.Writer, got read, _ string) { _, _ = io.WriteString(w, "drew "+got.Items[0].Subject+"\n") }
	streams, _, stdout, _ := output.Test()

	err := launched(t, q, &uicli.Run{Command: "fake", Streams: streams})

	failed, ok := errors.AsType[*uicli.Failed](err)
	require.True(t, ok, err)
	assert.Equal(t, "fake.unreadable", failed.Err.Code)
	assert.Contains(t, stdout.String(), "drew a")
}

func TestAPickKeepsTheFailureOfTheReadInItsExitCode(t *testing.T) {
	q, _ := fakeQuery()
	q.Open = func(context.Context, string) (form.Reader[request, read], error) {
		return reader{read: read{Items: []domain.Unit[struct{}]{{Subject: "a", Status: domain.StatusFailed, Err: &domain.Error{Kind: domain.KindInternal, Code: "fake.unreadable", Message: "couldn't read a"}}}}}, nil
	}
	streams, _, stdout, _ := output.Test()
	streams.SetStdinTerminal(true)
	streams.Err.SetTerminal(true)
	streams.Out.SetTerminal(true)

	err := launched(t, q, &uicli.Run{Command: "fake", Streams: streams})

	failed, ok := errors.AsType[*uicli.Failed](err)
	require.True(t, ok, err)
	assert.Equal(t, "fake.unreadable", failed.Err.Code)
	assert.Contains(t, stdout.String(), "→ wtm go feat/x")
}

package cli

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfEvents(t *testing.T) {
	got := runEvents(t, run{}, "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "events_help", got.Stdout)
}

func TestEachChangeIsALineForAPerson(t *testing.T) {
	got := runEvents(t, run{Dir: app.Root, Lines: changes(t)})

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Empty(t, got.Stderr)
	formtest.Golden(t, "events_text", got.Stdout)
}

func TestTheStreamOfEveryRepositoryNamesTheRepositoryOfEachChange(t *testing.T) {
	got := runEvents(t, run{Dir: app.Root, Lines: changes(t), Registered: []domain.Repo{app}}, "--all")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "events_text_all", got.Stdout)
}

func TestWithJSONEachLineIsWrittenAsItCame(t *testing.T) {
	lines := changes(t)

	got := runEvents(t, run{Dir: app.Root, Lines: lines}, "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	written := strings.Split(strings.TrimSuffix(got.Stdout, "\n"), "\n")
	require.Len(t, written, 2+len(lines))
	assert.Equal(t, lines, written[2:])
	formtest.Golden(t, "events_json", got.Stdout)
}

func TestARefusalWithJSONIsTheEnvelopeOnStdout(t *testing.T) {
	got := runEvents(t, run{Dir: app.Root, Absent: true}, "--output", "json")

	assert.Equal(t, uicli.ExitConfigNotFound, got.Code)
	assert.Empty(t, got.Stderr)
	formtest.Golden(t, "envelope_config_not_found", got.Stdout)
}

func TestARequestThatCannotBeWatchedExitsWithTheCodeOfItsRefusal(t *testing.T) {
	for name, c := range map[string]struct {
		Words  []string
		Code   int
		Stderr string
	}{
		"repo_with_all":      {Words: []string{"--all", "--repo", app.Root}, Code: uicli.ExitUsage, Stderr: "--repo watches one repository, all of them already"},
		"repo_not_directory": {Words: []string{"--repo", "/nowhere"}, Code: uicli.ExitUsage, Stderr: "--repo /nowhere is not a directory"},
		"repo_outside":       {Words: []string{"--repo", "/tmp"}, Code: uicli.ExitNotGitRepo, Stderr: "/tmp is not a git repository"},
		"not_initialized":    {Words: nil, Code: uicli.ExitConfigNotFound, Stderr: "no wtm config found"},
	} {
		t.Run(name, func(t *testing.T) {
			got := runEvents(t, run{Dir: app.Root, Absent: name == "not_initialized"}, c.Words...)

			assert.Equal(t, c.Code, got.Code)
			assert.Contains(t, got.Stderr, c.Stderr)
			assert.Empty(t, got.Stdout)
		})
	}
}

// Legacy -q silenced the warnings with the events.
func TestQuietSilencesTheEventsButNotTheWarnings(t *testing.T) {
	got := runEvents(t, run{Dir: app.Root, Fail: []error{errors.New("connect to the event bus: refused")}, Lines: changes(t)}, "-q")

	require.Equal(t, uicli.ExitOK, got.Code)
	assert.Empty(t, got.Stdout)
	assert.Equal(t, "  ! connect to the event bus: refused\n", got.Stderr)
}

func TestALineOfANewerSchemaEndsTheStreamOnStderrWithItsOwnCode(t *testing.T) {
	got := runEvents(t, run{Dir: app.Root, Lines: []string{`{"v":3,"type":"worktree.created"}`}, Ends: true}, "--output", "json")

	assert.Equal(t, uicli.ExitStreamNewer, got.Code)
	assert.Equal(t, 2, strings.Count(got.Stdout, "\n"), "stdout carries the stream's lines alone")
	assert.Contains(t, got.Stderr, "the event stream speaks a newer schema than this wtm: upgrade wtm")
}

func TestAReaderGoneIsTheStreamEndingWell(t *testing.T) {
	assert.True(t, readerLeft(make(chan struct{}), fmt.Errorf("write: %w", syscall.EPIPE)))
	assert.False(t, readerLeft(make(chan struct{}), errors.New("disk full")))
}

func TestEventsTakesNoArgument(t *testing.T) {
	got := runEvents(t, run{Dir: app.Root}, "feat/a")

	assert.Equal(t, uicli.ExitUsage, got.Code)
}

package shared

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

func testPresenter(t *testing.T) (CLIPresenter, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&bytes.Buffer{})
	return NewPresenter(cmd, domain.OutputText), &stderr
}

// The border is what says the reader still has something to do. A note — what
// the machine made of a pass nobody has to act on — takes the plain block, or a
// yellow box ends up on the least actionable line of the run.
func TestStatus_NoteIsNotBordered(t *testing.T) {
	presenter, stderr := testPresenter(t)

	presenter.Status(flow.Notice{
		Kind:  flow.NoticeNote,
		Text:  domain.EnvOriginPortedTitle,
		Lines: []string{"written with :8080"},
	})

	got := stderr.String()
	if !strings.Contains(got, domain.EnvOriginPortedTitle) || !strings.Contains(got, "written with :8080") {
		t.Fatalf("the note lost its content: %q", got)
	}
	if strings.ContainsAny(got, "─│╭╮╰╯") {
		t.Errorf("a note must not be boxed, got %q", got)
	}
}

func TestStatus_WarningWithLinesKeepsTheBox(t *testing.T) {
	presenter, stderr := testPresenter(t)

	presenter.Status(flow.Notice{
		Kind:  flow.NoticeWarning,
		Text:  domain.EnvPortAnomaliesTitle,
		Lines: []string{"API_URL  no such key in this file"},
	})

	if got := stderr.String(); !strings.ContainsAny(got, "─│╭╮╰╯") {
		t.Errorf("what the reader has to act on stays bordered, got %q", got)
	}
}

// Everything a run says while it is still running belongs to one block: one
// leading blank, and no second one opening a frame that is already open.
func TestStatus_OpensTheMidRunBlockExactlyOnce(t *testing.T) {
	presenter, stderr := testPresenter(t)

	presenter.Status(flow.Notice{Kind: flow.NoticeSuccess, Text: "released a from b"})
	presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: "c is down"})

	got := stderr.String()
	if !strings.HasPrefix(got, "\n") {
		t.Errorf("the block opens with its own blank line, got %q", got)
	}
	if strings.Contains(got, "\n\n\n") || strings.Contains(strings.TrimPrefix(got, "\n"), "\n\n") {
		t.Errorf("consecutive status lines take no separator, got %q", got)
	}
}

// A JSON run gets the lines and nothing else: no frame, no bar, no box.
func TestStatus_MachineOutputStaysBare(t *testing.T) {
	cmd := &cobra.Command{}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	presenter := NewPresenter(cmd, domain.OutputJSON)

	presenter.Status(flow.Notice{
		Kind:  flow.NoticeNote,
		Text:  domain.EnvOriginPortedTitle,
		Lines: []string{"written with :8080"},
	})

	got := stderr.String()
	if strings.HasPrefix(got, "\n") || strings.ContainsAny(got, "─│╭╮╰╯") {
		t.Errorf("machine output takes neither frame nor box, got %q", got)
	}
}

// The hook phase is the other half of the mid-run block: what it leaves behind
// is kept, so it reads inside the same frame as the status lines, one blank
// apart from them.
func TestHookPhase_JoinsTheMidRunBlock(t *testing.T) {
	presenter, stderr := testPresenter(t)

	presenter.Status(flow.Notice{Kind: flow.NoticeSuccess, Text: "released a from b"})
	err := presenter.HookPhase(flow.HookPhaseParams{
		Title: domain.HooksTitleOnCreate,
		Run:   func(flow.HookSink) error { return nil },
	})
	if err != nil {
		t.Fatalf("HookPhase() = %v", err)
	}

	got := stderr.String()
	if !strings.HasPrefix(got, "\n") {
		t.Errorf("the block opens with its own blank line, got %q", got)
	}
	if !strings.Contains(got, domain.HooksTitleOnCreate) {
		t.Errorf("the phase lost its title: %q", got)
	}
	if strings.Count(got, "\n\n") != 1 {
		t.Errorf("the phase takes exactly one blank line off the lines above it, got %q", got)
	}
}

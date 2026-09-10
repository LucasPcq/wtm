package shared

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/output"
)

// bordered asks the style itself what a box is drawn with, so this stops
// answering "no box" the day the border set changes.
func bordered(got string) bool {
	border := lipgloss.RoundedBorder()
	for _, edge := range []string{border.TopLeft, border.TopRight, border.BottomLeft, border.BottomRight} {
		if strings.Contains(got, edge) {
			return true
		}
	}
	return false
}

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
	if bordered(got) {
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

	if got := stderr.String(); !bordered(got) {
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
	if strings.HasPrefix(got, "\n") || bordered(got) {
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

// The block a run keeps while it runs is closed by whoever writes next, and the
// blank that closes it is the blank the next one opens on. A presenter that
// remembered this itself could not know: the frame between the two is written
// by code that never sees it — `run up` around a job's output is the case that
// made this fail.
func TestStatus_TakesNoSecondBlankAfterAFrameClosedTheBlock(t *testing.T) {
	presenter, stderr := testPresenter(t)

	presenter.Status(flow.Notice{Kind: flow.NoticeSuccess, Text: "stopped services on a"})
	output.Frame(stderr, func(w io.Writer) { output.Message(w, "a job's output") })
	stderr.Reset()
	presenter.Status(flow.Notice{Kind: flow.NoticeMessage, Text: "port probes silenced for web"})

	if got := stderr.String(); strings.HasPrefix(got, "\n") {
		t.Errorf("the frame's closing blank is this block's opening one, got %q", got)
	}
}

// The same line, whether or not an unrelated status was emitted earlier in the
// run: what decided the spacing used to be an event with nothing to do with it.
func TestStatus_ReadsTheSameWhicheverRanBefore(t *testing.T) {
	first, firstErr := testPresenter(t)
	output.Frame(firstErr, func(w io.Writer) { output.Message(w, "a job's output") })
	firstErr.Reset()
	first.Status(flow.Notice{Kind: flow.NoticeMessage, Text: "silenced"})

	second, secondErr := testPresenter(t)
	second.Status(flow.Notice{Kind: flow.NoticeSuccess, Text: "stopped services on a"})
	output.Frame(secondErr, func(w io.Writer) { output.Message(w, "a job's output") })
	secondErr.Reset()
	second.Status(flow.Notice{Kind: flow.NoticeMessage, Text: "silenced"})

	if firstErr.String() != secondErr.String() {
		t.Errorf("the line reads %q after a frame alone and %q after a status and a frame", firstErr, secondErr)
	}
}

// extract reaches a hook phase through RunCreateHooksPhase while holding a
// presenter of its own, which may already have opened the block for the port
// pass. The phase joins that block instead of drawing a second, unbarred one
// beside it.
func TestDrawHookPhase_JoinsAnAlreadyOpenBlock(t *testing.T) {
	var stderr bytes.Buffer
	output.FrameStart(&stderr)
	output.Message(output.Barred(&stderr), "a port was left alone")
	stderr.Reset()

	err := DrawHookPhase(DrawHookPhaseParams{
		Stderr: &stderr,
		Human:  true,
		Title:  domain.HooksTitleOnCreate,
		Run:    func(flow.HookSink) error { return nil },
	})
	if err != nil {
		t.Fatalf("DrawHookPhase() = %v", err)
	}

	got := stderr.String()
	if !strings.HasPrefix(got, "\n") || strings.HasPrefix(got, "\n\n") {
		t.Errorf("the phase takes exactly one blank off the block it joins, got %q", got)
	}
	if !strings.Contains(got, domain.HooksTitleOnCreate) {
		t.Errorf("the phase lost its title: %q", got)
	}
}

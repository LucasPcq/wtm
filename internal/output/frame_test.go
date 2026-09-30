package output

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

func TestFrame_WrapsBodyInSingleTopAndBottomBlank(t *testing.T) {
	var buf bytes.Buffer
	Frame(&buf, func(w io.Writer) {
		Success(w, "done")
	})

	// The body line carries styled bytes; assert the frame shape rather than the
	// exact styled content: exactly one leading blank and one trailing blank.
	got := buf.String()
	if got[0] != '\n' {
		t.Errorf("expected a single leading blank line, got %q", got)
	}
	if len(got) < 2 || got[len(got)-1] != '\n' || got[len(got)-2] != '\n' {
		t.Errorf("expected a trailing blank line, got %q", got)
	}
}

func TestFrame_NoStackedBlankLines(t *testing.T) {
	var buf bytes.Buffer
	Frame(&buf, func(w io.Writer) {
		Message(w, "hello")
	})

	if got := buf.String(); containsTripleNewline(got) {
		t.Errorf("frame must not produce stacked blank lines, got %q", got)
	}
}

func TestFrameStartFrameEnd_EachEmitOneBlank(t *testing.T) {
	var buf bytes.Buffer
	FrameStart(&buf)
	FrameEnd(&buf)

	if got := buf.String(); got != "\n\n" {
		t.Errorf("expected FrameStart+FrameEnd to emit exactly two newlines, got %q", got)
	}
}

func containsTripleNewline(s string) bool {
	for i := 0; i+2 < len(s); i++ {
		if s[i] == '\n' && s[i+1] == '\n' && s[i+2] == '\n' {
			return true
		}
	}
	return false
}

// The bar is the CLI's own mark, so it never reaches a log: a buffer, a pipe or
// a redirection gets the bare text and a grep over it stays clean.
func TestBarredLeavesANonTerminalAlone(t *testing.T) {
	var buf bytes.Buffer
	if Barred(&buf) != io.Writer(&buf) {
		t.Error("a non-terminal writer was wrapped")
	}
}

// Every line of a block carries the bar, blank separators included — a gap in
// the rule reads as two blocks.
func TestBarWriterMarksEveryLineIncludingBlankOnes(t *testing.T) {
	var buf bytes.Buffer
	bar := &barWriter{w: &buf, atLineStart: true}
	if _, err := io.WriteString(bar, "one\n\ntwo\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("wrote %d lines, want 3: %q", len(lines), buf.String())
	}
	for i, line := range lines {
		if !strings.Contains(line, domain.AccentBarGlyph) {
			t.Errorf("line %d carries no bar: %q", i, line)
		}
	}
}

func TestBarredCRLFSplitAcrossWrites(t *testing.T) {
	var buf bytes.Buffer
	bar := &barWriter{w: &buf, atLineStart: true}

	_, _ = bar.Write([]byte("a\r"))
	_, _ = bar.Write([]byte("\nb\n"))

	glyph := styles.Primary.Render(domain.AccentBarGlyph)
	want := glyph + "a\r\n" + glyph + "b\n"
	if buf.String() != want {
		t.Fatalf("bar over a split CRLF = %q, want %q", buf.String(), want)
	}
}

func TestBarredCarriageReturnRemarksTheRow(t *testing.T) {
	var buf bytes.Buffer
	bar := &barWriter{w: &buf, atLineStart: true}

	_, _ = bar.Write([]byte("50%\r"))
	_, _ = bar.Write([]byte("100%\n"))

	glyph := styles.Primary.Render(domain.AccentBarGlyph)
	want := glyph + "50%\r" + glyph + "100%\n"
	if buf.String() != want {
		t.Fatalf("bar over a redrawn row = %q, want %q", buf.String(), want)
	}
}

// A surface that has to know whether it may repaint, or how wide it is, reads
// the stream and not the wrapper: a hook phase handed a barred writer would
// otherwise decide it was writing to a pipe and stream its whole output instead
// of a tail. The bars are counted rather than flagged, so a doubly wrapped
// writer does not over-report its width by a column.
func TestUnwrapStream_ReadsThroughTheBarAndCountsIt(t *testing.T) {
	var buf bytes.Buffer

	for _, tc := range []struct {
		name  string
		given io.Writer
		bars  int
	}{
		{name: "bare", given: &buf, bars: 0},
		{name: "barred", given: &barWriter{w: &buf}, bars: 1},
		{name: "barred twice", given: &barWriter{w: &barWriter{w: &buf}}, bars: 2},
	} {
		stream, bars := unwrapStream(tc.given)
		if stream != io.Writer(&buf) {
			t.Errorf("%s: unwrapStream returned %T, want the buffer underneath", tc.name, stream)
		}
		if bars != tc.bars {
			t.Errorf("%s: counted %d bars, want %d", tc.name, bars, tc.bars)
		}
	}
}

// The blank closing a block and the blank opening the next are one line on
// screen. A surface remembers which, because the block beside this one is
// written by code that never sees it.
func TestFrame_ConsecutiveBlocksShareOneBlankLine(t *testing.T) {
	var buf bytes.Buffer

	Frame(&buf, func(w io.Writer) { Message(w, "first") })
	Frame(&buf, func(w io.Writer) { Message(w, "second") })

	if got := buf.String(); containsTripleNewline(got) {
		t.Errorf("two blocks in a row must not stack their blanks, got %q", got)
	}
}

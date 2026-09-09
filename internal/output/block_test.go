package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestSuccess_ContainsCheckmarkAndMessage(t *testing.T) {
	var buf bytes.Buffer
	Success(&buf, "deployed")

	out := buf.String()
	if !strings.HasPrefix(out, Indent) {
		t.Errorf("expected output to start with Indent, got %q", out)
	}
	if !strings.Contains(out, "✓") {
		t.Error("expected output to contain ✓")
	}
	if !strings.Contains(out, "deployed") {
		t.Error("expected output to contain message")
	}
}

func TestError_ContainsCrossAndMessage(t *testing.T) {
	var buf bytes.Buffer
	Error(&buf, "failed")

	out := buf.String()
	if !strings.HasPrefix(out, Indent) {
		t.Errorf("expected output to start with Indent, got %q", out)
	}
	if !strings.Contains(out, "✗") {
		t.Error("expected output to contain ✗")
	}
	if !strings.Contains(out, "failed") {
		t.Error("expected output to contain message")
	}
}

func TestWarning_ContainsBangAndMessage(t *testing.T) {
	var buf bytes.Buffer
	Warning(&buf, "careful")

	out := buf.String()
	if !strings.HasPrefix(out, Indent) {
		t.Errorf("expected output to start with Indent, got %q", out)
	}
	if !strings.Contains(out, "!") {
		t.Error("expected output to contain !")
	}
	if !strings.Contains(out, "careful") {
		t.Error("expected output to contain message")
	}
}

func TestLoading_ContainsChevronAndMessage(t *testing.T) {
	var buf bytes.Buffer
	Loading(&buf, "working")

	out := buf.String()
	if !strings.HasPrefix(out, Indent) {
		t.Errorf("expected output to start with Indent, got %q", out)
	}
	if !strings.Contains(out, "›") {
		t.Error("expected output to contain ›")
	}
	if !strings.Contains(out, "working") {
		t.Error("expected output to contain message")
	}
}

func TestMessage_ContainsTextWithIndent(t *testing.T) {
	var buf bytes.Buffer
	Message(&buf, "hello")

	out := buf.String()
	if !strings.HasPrefix(out, Indent) {
		t.Errorf("expected output to start with Indent, got %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Error("expected output to contain message")
	}
}

func TestSectionTitle_ContainsTitleWithIndent(t *testing.T) {
	var buf bytes.Buffer
	SectionTitle(&buf, "Overview")

	out := buf.String()
	if !strings.HasPrefix(out, Indent) {
		t.Errorf("expected output to start with Indent, got %q", out)
	}
	if !strings.Contains(out, "Overview") {
		t.Error("expected output to contain title")
	}
}

func TestInfoLine_ContainsLabelAndValue(t *testing.T) {
	var buf bytes.Buffer
	InfoLine(&buf, "Branch:", "main")

	out := buf.String()
	if !strings.HasPrefix(out, Indent) {
		t.Errorf("expected output to start with Indent, got %q", out)
	}
	if !strings.Contains(out, "Branch:") {
		t.Error("expected output to contain label")
	}
	if !strings.Contains(out, "main") {
		t.Error("expected output to contain value")
	}
}

func TestBlank_EmitsNewline(t *testing.T) {
	var buf bytes.Buffer
	Blank(&buf)

	if buf.String() != "\n" {
		t.Errorf("expected single newline, got %q", buf.String())
	}
}

// A conclusion counts what happened, never what did not: a zero count would put
// "0 blocked" on every clean run and teach the reader to skip the line.
func TestTallyDropsZeroCounts(t *testing.T) {
	got := Tally(
		TallyPart{Count: 3, Label: domain.TallyApplied},
		TallyPart{Count: 0, Label: domain.TallySkipped},
		TallyPart{Count: 1, Label: domain.TallyBlocked},
	)
	if got != "3 applied · 1 blocked" {
		t.Errorf("Tally = %q, want %q", got, "3 applied · 1 blocked")
	}
	if empty := Tally(TallyPart{Count: 0, Label: domain.TallyApplied}); empty != "" {
		t.Errorf("a run that did nothing tallied %q, want nothing", empty)
	}
}

// Every hint in the CLI is one arrow and one bold command, so a reader learns
// once where to look for what to do next.
func TestNextStepIsOneArrowAndOneCommand(t *testing.T) {
	var buf bytes.Buffer
	NextStep(&buf, NextStepParams{Command: "wtm go feat/x"})

	line := strings.TrimSuffix(buf.String(), "\n")
	if strings.Count(line, domain.NextStepGlyph) != 1 || !strings.Contains(line, "wtm go feat/x") {
		t.Errorf("NextStep = %q, want one arrow and the command", line)
	}
	if strings.Contains(line, domain.NextStepGlyph+"  ") {
		t.Errorf("NextStep = %q, want a single space after the arrow", line)
	}
}

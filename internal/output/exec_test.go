package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
)

func execCode(n int) *int { return &n }

func TestConclusionAllPassedIsOneLine(t *testing.T) {
	var buf bytes.Buffer
	FormatExecConclusion(&buf, ExecConclusionParams{
		Command: "pnpm test",
		Results: []domain.ExecResult{{Branch: "a", Status: domain.ExecStatusPassed, ExitCode: execCode(0)}, {Branch: "b", Status: domain.ExecStatusPassed, ExitCode: execCode(0)}},
		Elapsed: 18200 * time.Millisecond,
	})
	got := ansi.Strip(buf.String())
	if strings.Count(got, "\n") != 1 || !strings.Contains(got, "pnpm test · 2 worktrees (18.2s)") {
		t.Fatalf("got %q", got)
	}
}

func TestConclusionExpandsEachFailureWithTailAndLog(t *testing.T) {
	var buf bytes.Buffer
	FormatExecConclusion(&buf, ExecConclusionParams{
		Command: "pnpm test",
		Results: []domain.ExecResult{
			{Branch: "a", Status: domain.ExecStatusPassed, ExitCode: execCode(0)},
			{Branch: "b", Status: domain.ExecStatusFailed, ExitCode: execCode(1), DurationMs: 3200, Tail: []string{"FAIL x.test.ts"}, Log: "/s/exec/b.log"},
			{Branch: "c", Status: domain.ExecStatusInterrupted},
		},
	})
	got := ansi.Strip(buf.String())
	for _, want := range []string{"2 of 3 worktrees failed", "b (exit 1, 3.2s)", "FAIL x.test.ts", "/s/exec/b.log", "c  interrupted", "1 passed"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestConclusionTailIsBounded(t *testing.T) {
	tail := make([]string, 20)
	for i := range tail {
		tail[i] = "line" + string(rune('a'+i))
	}
	var buf bytes.Buffer
	FormatExecConclusion(&buf, ExecConclusionParams{Command: "x", Results: []domain.ExecResult{{Branch: "b", Status: domain.ExecStatusFailed, ExitCode: execCode(1), Tail: tail}}})
	got := ansi.Strip(buf.String())
	if strings.Contains(got, "linea") || !strings.Contains(got, "linet") {
		t.Fatalf("expected only the last %d lines:\n%s", domain.ExecConclusionTailLines, got)
	}
}

func TestPrintShowsEveryOutputInSelectionOrder(t *testing.T) {
	var buf bytes.Buffer
	FormatExecPrint(&buf, []domain.ExecResult{{Branch: "b", Output: "bee\n"}, {Branch: "a", Output: "ay\n"}})
	got := ansi.Strip(buf.String())
	if strings.Index(got, "bee") > strings.Index(got, "ay") || !strings.Contains(got, "b") {
		t.Fatalf("got %q", got)
	}
}

func TestExecJSONEnvelope(t *testing.T) {
	var buf bytes.Buffer
	err := WriteExecJSON(&buf, ExecJSONParams{Command: "x", Results: []domain.ExecResult{
		{Branch: "a", Status: domain.ExecStatusPassed, ExitCode: execCode(0)},
		{Branch: "b", Status: domain.ExecStatusNotStarted},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Command string           `json:"command"`
		Results []map[string]any `json:"results"`
		Failed  []string         `json:"failed"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Command != "x" || len(doc.Results) != 2 || len(doc.Failed) != 1 || doc.Failed[0] != "b" {
		t.Fatalf("doc = %+v", doc)
	}
	if _, ok := doc.Results[0]["exit_code"]; !ok {
		t.Error("a passing exit code 0 must be written")
	}
	if _, ok := doc.Results[1]["exit_code"]; ok {
		t.Error("a command that never ran has no exit code")
	}
}

func TestExecViewRepaintsInPlaceAndClears(t *testing.T) {
	var buf bytes.Buffer
	view := NewExecView(ExecViewParams{W: &buf, Branches: []string{"a", "b"}})
	view.OnBeat(domain.ExecBeat{Index: 0, Started: true})
	view.OnBeat(domain.ExecBeat{Index: 0, Result: domain.ExecResult{Branch: "a", Status: domain.ExecStatusPassed, ExitCode: execCode(0)}})
	view.Close()
	got := buf.String()
	if !strings.Contains(got, domain.AnsiClearBelow) || !strings.Contains(ansi.Strip(got), "b  "+domain.ExecQueuedLabel) {
		t.Fatalf("got %q", got)
	}
}

func TestExecResultLineMarksPassAndFailure(t *testing.T) {
	var buf bytes.Buffer
	ExecResultLine(&buf, domain.ExecResult{Branch: "a", Status: domain.ExecStatusPassed, ExitCode: execCode(0), DurationMs: 1200})
	ExecResultLine(&buf, domain.ExecResult{Branch: "b", Status: domain.ExecStatusFailed, ExitCode: execCode(1), DurationMs: 300})
	got := ansi.Strip(buf.String())
	if !strings.Contains(got, domain.GlyphSuccess+" a (1.2s)") || !strings.Contains(got, domain.GlyphFailure+" b (exit 1, 300ms)") {
		t.Fatalf("got %q", got)
	}
}

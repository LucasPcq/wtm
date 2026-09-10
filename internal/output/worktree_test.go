package output

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func init() {
	os.Setenv("NO_COLOR", "1")
}

func TestFormatWorktreeListEmpty(t *testing.T) {
	got := ansi.Strip(FormatWorktreeList(FormatWorktreeListParams{}))
	if got != UnchangedLine(domain.NoWorktreesMessage) {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestFormatWorktreeListSingleParent(t *testing.T) {
	got := FormatWorktreeList(FormatWorktreeListParams{
		Statuses: []domain.WorktreeStatus{
			{Branch: "main", IsParent: true, IsDirty: false, CommitsAhead: 0, CreatedAt: time.Now()},
		},
	})

	if !strings.Contains(got, "main") {
		t.Error("expected output to contain 'main'")
	}
	if !strings.Contains(got, "(parent)") {
		t.Error("expected output to contain '(parent)'")
	}
	if !strings.Contains(got, "clean") {
		t.Error("expected output to contain 'clean'")
	}
}

func TestFormatWorktreeListDirtyAndAhead(t *testing.T) {
	got := FormatWorktreeList(FormatWorktreeListParams{
		Statuses: []domain.WorktreeStatus{
			{Branch: "main", IsParent: true, IsDirty: false, CreatedAt: time.Now()},
			{Branch: "feature-auth", IsParent: false, IsDirty: true, CommitsAhead: 3, CreatedAt: time.Now()},
		},
	})

	if !strings.Contains(got, "dirty") {
		t.Error("expected output to contain 'dirty'")
	}
	if !strings.Contains(got, "base ↑3") {
		t.Error("expected output to contain 'base ↑3'")
	}
}

func TestFormatWorktreeListSingleCommitAhead(t *testing.T) {
	got := FormatWorktreeList(FormatWorktreeListParams{
		Statuses: []domain.WorktreeStatus{
			{Branch: "fix", IsParent: false, IsDirty: false, CommitsAhead: 1, CreatedAt: time.Now()},
		},
	})

	if !strings.Contains(got, "base ↑1") {
		t.Error("expected output to contain 'base ↑1'")
	}
}

func TestFormatWorktreeListActiveIndicator(t *testing.T) {
	got := FormatWorktreeList(FormatWorktreeListParams{
		Statuses: []domain.WorktreeStatus{
			{Branch: "main", IsParent: true, CreatedAt: time.Now()},
			{Branch: "feature/auth", IsParent: false, CreatedAt: time.Now()},
		},
		ActiveBranch: "feature/auth",
	})

	if !strings.Contains(got, "active") {
		t.Error("expected active indicator on focused worktree")
	}
}

func TestPrintableLen(t *testing.T) {
	if printableLen("hello") != 5 {
		t.Error("plain string length wrong")
	}
	if printableLen("\x1b[1mhello\x1b[0m") != 5 {
		t.Error("ANSI string length wrong")
	}
}

func TestFormatTagParentOnly(t *testing.T) {
	got := formatTag(true, false)
	if !strings.Contains(got, "(parent)") {
		t.Error("expected output to contain '(parent)'")
	}
	if strings.Contains(got, "active") {
		t.Error("expected output to NOT contain 'active'")
	}
}

func TestFormatTagActiveOnly(t *testing.T) {
	got := formatTag(false, true)
	if !strings.Contains(got, "active") {
		t.Error("expected output to contain 'active'")
	}
	if strings.Contains(got, "(parent)") {
		t.Error("expected output to NOT contain '(parent)'")
	}
}

func TestFormatTagBoth(t *testing.T) {
	got := formatTag(true, true)
	if !strings.Contains(got, "(parent)") {
		t.Error("expected output to contain '(parent)'")
	}
	if !strings.Contains(got, "active") {
		t.Error("expected output to contain 'active'")
	}
}

func TestFormatTagNeither(t *testing.T) {
	got := formatTag(false, false)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatAheadZero(t *testing.T) {
	got := formatAhead(0)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatAheadOne(t *testing.T) {
	got := formatAhead(1)
	if !strings.Contains(got, "base ↑1") {
		t.Errorf("expected 'base ↑1', got %q", got)
	}
}

func TestFormatAheadMultiple(t *testing.T) {
	got := formatAhead(5)
	if !strings.Contains(got, "base ↑5") {
		t.Errorf("expected 'base ↑5', got %q", got)
	}
}

func TestFormatOriginStates(t *testing.T) {
	cases := []struct {
		name  string
		s     domain.WorktreeStatus
		want  string
		empty bool
	}{
		{name: "unknown", s: domain.WorktreeStatus{OriginState: domain.DivergenceUnknown}, empty: true},
		{name: "up-to-date", s: domain.WorktreeStatus{OriginState: domain.DivergenceUpToDate}, empty: true},
		{name: "behind", s: domain.WorktreeStatus{OriginState: domain.DivergenceBehind, OriginBehind: 5}, want: "origin ↓5"},
		{name: "ahead", s: domain.WorktreeStatus{OriginState: domain.DivergenceAhead, OriginAhead: 2}, want: "origin ↑2"},
		{name: "diverged", s: domain.WorktreeStatus{OriginState: domain.DivergenceDiverged, OriginAhead: 2, OriginBehind: 5}, want: "origin ↑2 ↓5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatOrigin(tc.s)
			if tc.empty {
				if got != "" {
					t.Errorf("expected empty, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestPrintableLenEmpty(t *testing.T) {
	if printableLen("") != 0 {
		t.Error("expected printableLen of empty string to be 0")
	}
}

func TestAnsiOverheadPlainString(t *testing.T) {
	if ansiOverhead("hello world") != 0 {
		t.Error("expected ansiOverhead of plain string to be 0")
	}
}

// The port pass rides on the env line as a count. It is the whole of what create
// says about it: the values are in the .env the run just wrote, and `wtm env` is
// the command they belong to.
func TestFormatCreateResultCarriesThePortPassAsANote(t *testing.T) {
	render := func(note string) string {
		var buf bytes.Buffer
		FormatCreateResult(&buf, CreateResultParams{
			Branch:      "feat/x",
			From:        "main",
			EnvStrategy: "main",
			EnvNote:     note,
			Path:        ".worktrees/feat-x",
			GoCommand:   "wtm go feat/x",
		})
		return buf.String()
	}

	with := render("4 port(s) shifted (+10)")
	if !strings.Contains(with, "main") || !strings.Contains(with, "4 port(s) shifted (+10)") {
		t.Errorf("the env line dropped its note:\n%s", with)
	}
	if strings.Count(with, "\n") != strings.Count(render(""), "\n") {
		t.Errorf("the note cost the recap a line:\n%s", with)
	}
	if strings.Contains(render(""), domain.EnvRecapNoteSeparator) {
		t.Errorf("a run that moved nothing still printed a separator:\n%s", render(""))
	}
}

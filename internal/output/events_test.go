package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
)

func eventLine(t *testing.T, event domain.Event) string {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteEventLine(&buf, event); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(ansi.Strip(buf.String()))
}

func TestEachEventReadsAsOneLine(t *testing.T) {
	ordinal := 3
	identity := &domain.WorktreeIdentity{Branch: "feat/a", Path: "/wt/feat-a", Parent: "main", Ordinal: &ordinal, Isolation: domain.IsolationIsolated}
	repo := &domain.EventRepo{Root: "/repo", CommonDir: "/repo/.git"}

	cases := []struct {
		event domain.Event
		want  string
	}{
		{domain.Event{Type: domain.EventSnapshot, Repo: repo, Worktrees: []domain.WorktreeIdentity{*identity, *identity}}, "= 2 worktrees · /repo"},
		{domain.Event{Type: domain.EventReady}, "= watching for changes"},
		{domain.Event{Type: domain.EventWorktreeCreated, Worktree: identity}, "✓ created feat/a  /wt/feat-a"},
		{domain.Event{Type: domain.EventWorktreeRemoved, Worktree: identity}, "✓ removed feat/a"},
		{domain.Event{Type: domain.EventWorktreeRelocated, Worktree: identity, FromPath: "/old"}, "~ relocated feat/a  /old → /wt/feat-a"},
		{domain.Event{Type: domain.EventWorktreeReparented, Worktree: identity, FromParent: "feat/x"}, "~ reparented feat/a  feat/x → main"},
		{domain.Event{Type: domain.EventWorktreeUpdated, Worktree: identity, Changed: []domain.IdentityField{domain.IdentityIsolation, domain.IdentityOrdinal}}, "~ updated feat/a  isolation=isolated, ordinal=3"},
	}
	for _, c := range cases {
		if got := eventLine(t, c.event); got != c.want {
			t.Errorf("%s: got %q, want %q", c.event.Type, got, c.want)
		}
	}
}

func TestAnUnallocatedOrdinalReadsAsNone(t *testing.T) {
	identity := &domain.WorktreeIdentity{Branch: "feat/a"}
	got := eventLine(t, domain.Event{Type: domain.EventWorktreeUpdated, Worktree: identity, Changed: []domain.IdentityField{domain.IdentityOrdinal}})
	if got != "~ updated feat/a  ordinal=none" {
		t.Fatalf("got %q", got)
	}
}

func TestAnUnknownTypeWritesNothing(t *testing.T) {
	if got := eventLine(t, domain.Event{Type: "job.started"}); got != "" {
		t.Fatalf("got %q, want nothing", got)
	}
}

func TestAJSONEventIsOneCompactLine(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteEventJSONLine(&buf, domain.Event{V: 1, Type: domain.EventWorktreeCreated, TS: "t", Worktree: &domain.WorktreeIdentity{Branch: "a&b"}}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if strings.Count(got, "\n") != 1 || !strings.HasSuffix(got, "\n") || strings.Contains(got, "  ") || !strings.Contains(got, `"branch":"a&b"`) {
		t.Fatalf("got %q", got)
	}
}

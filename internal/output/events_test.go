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

func TestAJSONEventIsWrittenAsReceivedOnOneLine(t *testing.T) {
	var buf bytes.Buffer
	raw := []byte(`{"v":1,"type":"worktree.created","worktree":{"branch":"a&b"},"extra":1}`)
	if err := WriteEventJSONLine(&buf, raw); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != string(raw)+"\n" {
		t.Fatalf("got %q", got)
	}
}

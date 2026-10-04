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
	passed, failed, exitCode, five := true, false, 3, 5

	cases := []struct {
		event domain.Event
		want  string
	}{
		{domain.Event{Type: domain.EventSnapshot, Repo: repo, Worktrees: []domain.WorktreeIdentity{*identity, *identity}}, "= 2 worktrees · /repo"},
		{domain.Event{Type: domain.EventSnapshot, Repo: repo, Worktrees: []domain.WorktreeIdentity{*identity}}, "= 1 worktree · /repo"},
		{domain.Event{Type: domain.EventReady}, "= watching for changes"},
		{domain.Event{Type: domain.EventWorktreeCreated, Worktree: identity}, "✓ created feat/a  /wt/feat-a"},
		{domain.Event{Type: domain.EventWorktreeRemoved, Worktree: identity}, "✓ removed feat/a"},
		{domain.Event{Type: domain.EventWorktreeRelocated, Worktree: identity, FromPath: "/old"}, "~ relocated feat/a  /old → /wt/feat-a"},
		{domain.Event{Type: domain.EventWorktreeReparented, Worktree: identity, FromParent: "feat/x"}, "~ reparented feat/a  feat/x → main"},
		{domain.Event{Type: domain.EventWorktreeUpdated, Worktree: identity, Changed: []domain.IdentityField{domain.IdentityIsolation, domain.IdentityOrdinal}}, "~ updated feat/a  isolation=isolated, ordinal=3"},
		{domain.Event{Type: domain.EventWorktreeProvisioned, Worktree: identity, OK: &passed}, "✓ provisioned feat/a"},
		{domain.Event{Type: domain.EventWorktreeProvisioned, Worktree: identity, OK: &failed, Hook: "pnpm install", ExitCode: &exitCode}, "✗ on_create failed for feat/a  pnpm install (exit 3)"},
		{domain.Event{Type: domain.EventWorktreeProvisioned, Worktree: identity, OK: &failed}, "✗ on_create failed for feat/a"},
		{domain.Event{Type: domain.EventWorktreeDeprovisioned, Worktree: identity, OK: &failed, Hook: "exit 5", ExitCode: &five}, "✗ on_clean failed for feat/a, kept  exit 5 (exit 5)"},
		{domain.Event{Type: domain.EventRepoAdded, Repo: repo}, "~ watching /repo"},
		{domain.Event{Type: domain.EventRepoRemoved, Repo: repo}, "~ no longer watching /repo"},
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

// removed follows straight away, and says the same thing.
func TestASuccessfulDeprovisioningWritesNothing(t *testing.T) {
	ok := true
	if got := eventLine(t, domain.Event{Type: domain.EventWorktreeDeprovisioned, Worktree: &domain.WorktreeIdentity{Branch: "feat/a"}, OK: &ok}); got != "" {
		t.Fatalf("got %q", got)
	}
}

// Several repositories share one global stream: a worktree line names its own.
func TestAGlobalLineNamesTheRepositoryOfAWorktree(t *testing.T) {
	repo := &domain.EventRepo{Root: "/code/app", CommonDir: "/code/app/.git"}
	identity := &domain.WorktreeIdentity{Branch: "feat/a", Path: "/wt/feat-a"}
	cases := []struct {
		event domain.Event
		want  string
	}{
		{domain.Event{Type: domain.EventWorktreeCreated, Repo: repo, Worktree: identity}, "✓ app · created feat/a  /wt/feat-a"},
		{domain.Event{Type: domain.EventWorktreeRemoved, Repo: repo, Worktree: identity}, "✓ app · removed feat/a"},
		{domain.Event{Type: domain.EventReady}, "= watching for changes"},
		{domain.Event{Type: domain.EventRepoAdded, Repo: repo}, "~ watching /code/app"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if err := WriteGlobalEventLine(&buf, c.event); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(ansi.Strip(buf.String())); got != c.want {
			t.Errorf("%s: got %q, want %q", c.event.Type, got, c.want)
		}
	}
}

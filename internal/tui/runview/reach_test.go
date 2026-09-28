package runview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

func reachHarness(t *testing.T) *testHarness {
	t.Helper()
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("compose")},
		Streams: []string{"compose"},
	})
	h.emit(t, runlogs.Event{Phase: runlogs.PhaseStarted, Job: "compose", Step: 1, Steps: 1, Ports: map[string]int{
		"REDIS_PORT": 6379, "MINIO_PORT": 9000,
	}})
	return h
}

// The title says what the pane shows and where to reach it in one fragment;
// the ports themselves are the block's.
func TestThePaneTitleCarriesOneAddressFragment(t *testing.T) {
	h := reachHarness(t)

	frame := ansi.Strip(h.model.View())
	if !strings.Contains(frame, "compose · running · 2 ports") {
		t.Fatalf("frame = %q, want the title to count the ports", frame)
	}
	if strings.Contains(frame, "REDIS_PORT") {
		t.Errorf("frame = %q, want no port list in the title", frame)
	}
}

func TestTheReachKeyShowsTheBlockAndEscGoesBack(t *testing.T) {
	h := reachHarness(t)

	h.press(t, key(domain.RunViewReachKey))
	frame := ansi.Strip(h.model.View())
	if !strings.Contains(frame, domain.ReachTitle) || !strings.Contains(frame, "redis :6379") {
		t.Fatalf("frame = %q, want the block with each port by name", frame)
	}

	h.press(t, namedKey(tea.KeyEsc))
	if strings.Contains(ansi.Strip(h.model.View()), domain.ReachTitle) {
		t.Error("esc left the block on screen")
	}
}

func sharedIn(view runlogs.JobView, worktree string) runlogs.JobView {
	view.Status = domain.JobStatusAttached
	view.SharedIn = worktree
	return view
}

// A shared service this worktree only holds sits under its own jobs, below a
// line saying where it runs — and its title names that worktree, not this one.
func TestASharedServiceIsSetApartAndNamedByWhereItRuns(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views: []runlogs.JobView{
			inWorktree(sharedIn(running("postgres"), "main"), "/work/feat", "feat/x"),
			inWorktree(running("compose"), "/work/feat", "feat/x"),
		},
		Streams: []string{"postgres", "compose"},
	})

	frame := ansi.Strip(h.model.View())
	compose, shared, postgres := strings.Index(frame, "compose"), strings.Index(frame, "shared · main"), strings.Index(frame, "postgres")
	if compose < 0 || shared < 0 || !(compose < shared && shared < postgres) {
		t.Fatalf("frame = %q, want compose, then the shared line, then postgres", frame)
	}

	h.press(t, key("j"))
	frame = ansi.Strip(h.model.View())
	if !strings.Contains(frame, "postgres · attached to main") || strings.Contains(frame, "postgres · feat/x") {
		t.Errorf("frame = %q, want the title to say where postgres runs", frame)
	}
}

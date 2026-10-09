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
	if !strings.Contains(frame, domain.ReachTitle) || !strings.Contains(frame, "redis  :6379") {
		t.Fatalf("frame = %q, want the block with each port by name", frame)
	}

	h.press(t, namedKey(tea.KeyEsc))
	if strings.Contains(ansi.Strip(h.model.View()), domain.ReachTitle) {
		t.Error("esc left the block on screen")
	}
}

func sharedIn(view runlogs.JobView, worktree string) runlogs.JobView {
	view.Status = domain.JobStatusJoined
	view.SharedIn = worktree
	return view
}

// A shared service is set apart above the worktree's own jobs, under a line
// saying where it runs — and its title names that worktree, not this one.
func TestASharedServiceIsSetApartAndNamedByWhereItRuns(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views: []runlogs.JobView{
			inWorktree(running("compose"), "/work/feat", "feat/x"),
			inWorktree(sharedIn(running("postgres"), "main"), "/work/feat", "feat/x"),
		},
		Streams: []string{"postgres", "compose"},
	})

	frame := ansi.Strip(h.model.View())
	shared, postgres := strings.Index(frame, "shared"), strings.Index(frame, "◈  postgres")
	heading, compose := strings.Index(frame, "feat/x"), strings.Index(frame, "●  compose")
	if shared < 0 || !(shared < postgres && postgres < heading && heading < compose) {
		t.Fatalf("frame = %q, want the shared line and postgres above the worktree and its jobs", frame)
	}

	if !strings.Contains(frame, "postgres · joined, running in main") || strings.Contains(frame, "postgres · feat/x") {
		t.Errorf("frame = %q, want the title to say where postgres runs", frame)
	}
}

// Held by two worktrees, a shared service is still one process: one row in the
// list, and one entry in the block, each worktree's namespace under it.
func TestASharedServiceHeldByTwoWorktreesIsListedOnce(t *testing.T) {
	main := inWorktree(running("postgres"), "/work/main", "main")
	main.Shared, main.Namespace = true, "app_main"
	feat := inWorktree(sharedIn(running("postgres"), "main"), "/work/feat", "feat")
	feat.Shared, feat.Namespace = true, "app_feat"
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{main, inWorktree(running("web"), "/work/main", "main"), feat},
		Streams: []string{"postgres", "web"},
	})

	if count := strings.Count(ansi.Strip(h.model.renderSidebar(h.model.layout())), "postgres"); count != 1 {
		t.Errorf("postgres is listed %d times, want once", count)
	}
	if visible := h.model.visible(); len(visible) != 2 || viewKey(visible[0]) != viewKey(main) {
		t.Errorf("visible = %+v, want main's postgres first, then web", visible)
	}

	h.press(t, key(domain.RunViewReachKey))
	frame := ansi.Strip(h.model.View())
	if strings.Count(frame, "postgres") != 2 || !strings.Contains(frame, "main  app_main") || !strings.Contains(frame, "feat  app_feat") {
		t.Errorf("frame = %q, want postgres once with each worktree's namespace under it", frame)
	}
}

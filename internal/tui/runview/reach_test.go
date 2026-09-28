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

package runview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

func renderedLines(t *testing.T, store *paneStore, job jobKey) []string {
	t.Helper()
	entry, held := store.entry(job)
	if !held {
		t.Fatalf("no pane for %s", job)
	}
	var lines []string
	for _, line := range strings.Split(entry.pane.Render(), "\n") {
		if trimmed := strings.TrimRight(line, " "); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// Une task sort sur un pipe : ses LF nus faisaient descendre le curseur sans le
// ramener en colonne zéro, et chaque ligne s'affichait un cran plus à droite
// que la précédente (LUC-208).
func TestSequenceOutputOfATaskIsNotStaircased(t *testing.T) {
	store := newPaneStore(PaneSize{Cols: 60, Rows: 10})
	emitter := sink{panes: store, msgs: make(chan tea.Msg, 1), done: make(chan struct{})}

	emitter.Emit(runlogs.Event{
		Phase: runlogs.PhaseOutput,
		Job:   "migrate",
		Kind:  domain.JobKindTask,
		Chunk: []byte("running migrations\nseeding users\ndone\n"),
	})

	for _, line := range renderedLines(t, store, "migrate") {
		if strings.HasPrefix(line, " ") {
			t.Errorf("line %q starts indented: le LF n'a pas ramené le curseur en colonne zéro", line)
		}
	}
}

// Un service passe par un PTY, dont la discipline de ligne a déjà produit des
// CRLF : en ajouter un second casserait l'affichage dans l'autre sens.
func TestSequenceOutputOfAServiceIsLeftAlone(t *testing.T) {
	store := newPaneStore(PaneSize{Cols: 60, Rows: 10})
	emitter := sink{panes: store, msgs: make(chan tea.Msg, 1), done: make(chan struct{})}

	emitter.Emit(runlogs.Event{
		Phase: runlogs.PhaseOutput,
		Job:   "api",
		Kind:  domain.JobKindService,
		Chunk: []byte("listening on 3000\r\nready\r\n"),
	})

	lines := renderedLines(t, store, "api")
	if len(lines) != 2 || lines[0] != "listening on 3000" || lines[1] != "ready" {
		t.Errorf("lines = %q", lines)
	}
}

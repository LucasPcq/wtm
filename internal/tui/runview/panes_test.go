package runview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func paneText(t *testing.T, store *paneStore, job jobKey) string {
	t.Helper()
	entry, held := store.entry(job)
	if !held {
		t.Fatalf("no pane for %s", job)
	}
	return ansi.Strip(entry.pane.Render())
}

// A pane holds bytes from one source only. The daemon replays what a job
// already printed when a stream is attached, so appending to a pane that was
// filled from the log file would show that history twice.
func TestPaneStoreRebuildsWhenTheSourceChanges(t *testing.T) {
	store := newPaneStore(PaneSize{Cols: 40, Rows: 5})
	store.writeLines(writeLinesParams{Key: "api", Lines: []string{"from the log file"}})

	stream := runlogstest.NewStream()
	t.Cleanup(func() { stream.Close() })
	pane := store.attach(attachPaneParams{Key: "api", Stream: stream})
	pane.Write([]byte("from the stream\r\n"))

	text := paneText(t, store, "api")
	if strings.Contains(text, "from the log file") {
		t.Fatalf("pane = %q, want the log file's lines gone once the stream took over", text)
	}
	if !strings.Contains(text, "from the stream") {
		t.Fatalf("pane = %q, want the stream's bytes", text)
	}
}

// The other way round too, and the subscription goes with it: a pane refilled
// from the log file is no longer reading the job.
func TestPaneStoreRebuildsWhenTheLogFileTakesOver(t *testing.T) {
	store := newPaneStore(PaneSize{Cols: 40, Rows: 5})
	stream := runlogstest.NewStream()
	store.attach(attachPaneParams{Key: "api", Stream: stream}).Write([]byte("from the stream\r\n"))

	store.writeLines(writeLinesParams{Key: "api", Lines: []string{"from the log file"}})

	text := paneText(t, store, "api")
	if strings.Contains(text, "from the stream") {
		t.Fatalf("pane = %q, want the stream's bytes gone once the log file took over", text)
	}
	if !stream.Closed() {
		t.Fatal("the pane was rebuilt while its subscription was left open")
	}
	if store.hasStream() {
		t.Fatal("the store still holds a subscription for a pane it replaced")
	}
}

func TestPaneStoreKeepsAppendingWhileTheSourceHolds(t *testing.T) {
	store := newPaneStore(PaneSize{Cols: 40, Rows: 5})
	store.writeLines(writeLinesParams{Key: "api", Lines: []string{"first"}})
	store.writeLines(writeLinesParams{Key: "api", Lines: []string{"second"}})

	text := paneText(t, store, "api")
	if !strings.Contains(text, "first") || !strings.Contains(text, "second") {
		t.Fatalf("pane = %q, want both reads of the same source", text)
	}
}

func TestPaneStoreReleaseDropsThePaneWhateverFedIt(t *testing.T) {
	store := newPaneStore(PaneSize{Cols: 40, Rows: 5})
	stream := runlogstest.NewStream()
	store.attach(attachPaneParams{Key: "api", Stream: stream})
	store.writeLines(writeLinesParams{Key: "migrate", Lines: []string{"done"}})

	store.release("api")
	if _, held := store.entry("api"); held {
		t.Fatal("the attached pane was kept, holding a styled cell per written column")
	}
	if !stream.Closed() {
		t.Fatal("releasing a pane left its subscription open")
	}

	store.release("migrate")
	if _, held := store.entry("migrate"); held {
		t.Fatal("a pane read from the log file was kept for the life of the view")
	}
}

func TestPaneStoreAttachReplacesAnEarlierSubscription(t *testing.T) {
	store := newPaneStore(PaneSize{Cols: 40, Rows: 5})
	first := runlogstest.NewStream()
	second := runlogstest.NewStream()
	t.Cleanup(func() { second.Close() })

	store.attach(attachPaneParams{Key: "api", Stream: first})
	store.attach(attachPaneParams{Key: "api", Stream: second})

	if !first.Closed() {
		t.Fatal("a second attach left the first subscription open: the job would be read twice")
	}
	if got := store.stream("api"); got != second {
		t.Fatal("the store did not keep the newest subscription")
	}
}

// Only a pane that actually changed size is worth a round-trip to the daemon;
// x/vt never reflows, so the job's own process is what has to redraw.
func TestPaneStoreResizeReportsOnlyTheStreamsThatMoved(t *testing.T) {
	store := newPaneStore(PaneSize{Cols: 40, Rows: 5})
	stream := runlogstest.NewStream()
	t.Cleanup(func() { stream.Close() })
	store.attach(attachPaneParams{Key: "api", Stream: stream})
	store.writeLines(writeLinesParams{Key: "migrate", Lines: []string{"done"}})

	changed := store.resize(PaneSize{Cols: 80, Rows: 10})
	if len(changed) != 1 || changed[0] != stream {
		t.Fatalf("resize reported %v, want the one attached stream", changed)
	}
	if entry, _ := store.entry("migrate"); entry.pane.Size() != (PaneSize{Cols: 80, Rows: 10}) {
		t.Fatal("a pane with no stream was left at its old size")
	}

	if again := store.resize(PaneSize{Cols: 80, Rows: 10}); len(again) != 0 {
		t.Fatalf("resize reported %v for a size that did not change", again)
	}
}

func TestPaneStoreCloseAllEndsEverySubscription(t *testing.T) {
	store := newPaneStore(PaneSize{Cols: 40, Rows: 5})
	first, second := runlogstest.NewStream(), runlogstest.NewStream()
	store.attach(attachPaneParams{Key: "api", Stream: first})
	store.attach(attachPaneParams{Key: "web", Stream: second})

	store.closeAll()

	if !first.Closed() || !second.Closed() {
		t.Fatal("a subscription survived the view being left")
	}
	if store.hasStream() {
		t.Fatal("the store still reports something feeding a pane")
	}
	if _, held := store.entry("api"); held {
		t.Fatal("the panes were kept after the view was left")
	}
}

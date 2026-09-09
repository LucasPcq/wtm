package output

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

func runHookView(t *testing.T, logPath string, run func(view *HookView)) string {
	t.Helper()
	var buf bytes.Buffer
	view := NewHookView(HookViewParams{W: &buf, LogPath: logPath})
	run(view)
	view.Close()
	return buf.String()
}

// What a hook leaves on screen is its result, not its output: the tail is drawn
// while it runs and taken back when it ends.
func TestHookViewLeavesOnlyTheResultLine(t *testing.T) {
	out := runHookView(t, "", func(view *HookView) {
		view.OnHook(domain.HookBeat{Cmd: "pnpm install", Started: true})
		_, _ = view.Write([]byte("resolving\nfetching\n"))
		view.OnHook(domain.HookBeat{Cmd: "pnpm install", Duration: 12400 * time.Millisecond})
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "pnpm install (12.4s)") || !strings.Contains(last, domain.HookGlyphDone) {
		t.Errorf("the phase ended on %q, want the hook's result line", last)
	}
	if strings.Count(out, domain.AnsiClearBelow) == 0 {
		t.Error("the tail was printed and never taken back")
	}
}

// A hook that failed is the one time the output matters: the tail it died on
// stays, and so does the way to the whole of it.
func TestHookViewKeepsTheTailOfAFailingHook(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "on_create"+domain.HooksLogFileExt)
	out := runHookView(t, logPath, func(view *HookView) {
		view.OnHook(domain.HookBeat{Cmd: "pnpm install", Started: true})
		_, _ = view.Write([]byte("ERR_PNPM_NO_LOCKFILE\n"))
		view.OnHook(domain.HookBeat{
			Cmd:      "pnpm install",
			Duration: 2 * time.Second,
			Err:      "exit status 1",
			Stderr:   "lockfile is absent",
		})
	})

	for _, want := range []string{domain.HookGlyphFailed, "ERR_PNPM_NO_LOCKFILE", "lockfile is absent", logPath} {
		if !strings.Contains(out, want) {
			t.Errorf("a failed hook never showed %q:\n%s", want, out)
		}
	}
}

// The log holds what the tail could not, whether or not anything went wrong.
func TestHookViewLogsTheWholeStream(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "on_create"+domain.HooksLogFileExt)
	runHookView(t, logPath, func(view *HookView) {
		view.OnHook(domain.HookBeat{Cmd: "seq", Started: true})
		for i := 0; i < domain.HookViewTailLines*3; i++ {
			_, _ = view.Write([]byte("line\n"))
		}
		view.OnHook(domain.HookBeat{Cmd: "seq"})
	})

	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if got := strings.Count(string(body), "line\n"); got != domain.HookViewTailLines*3 {
		t.Errorf("log holds %d lines, want every one of the %d written", got, domain.HookViewTailLines*3)
	}
}

// A progress bar rewrites one row: it must be drawn without waiting for a
// newline that never comes, and what it ended on is the one line the record
// keeps — not one line per frame it drew.
func TestHookViewRewritesTheRowAProgressBarRedraws(t *testing.T) {
	var buf bytes.Buffer
	view := NewHookView(HookViewParams{W: &buf})
	view.OnHook(domain.HookBeat{Cmd: "docker pull", Started: true})

	_, _ = view.Write([]byte("10%\r"))
	if !strings.Contains(buf.String(), "10%") {
		t.Fatal("a row rewritten with no newline was never drawn")
	}

	_, _ = view.Write([]byte("50%\r100%\r"))
	buf.Reset()
	view.OnHook(domain.HookBeat{Cmd: "docker pull", Err: "boom"})
	view.Close()

	record := buf.String()
	if strings.Contains(record, "10%") || strings.Contains(record, "50%") {
		t.Errorf("the record kept every frame the bar drew:\n%s", record)
	}
	if !strings.Contains(record, "100%") {
		t.Errorf("the record lost what the bar ended on:\n%s", record)
	}
}

// The repaint moves the cursor back by the number of rows it last printed. If
// the two ever disagree the block eats the output above it, so the pair is
// pinned here rather than trusted.
func TestHookViewMovesBackExactlyTheRowsItPrinted(t *testing.T) {
	var buf bytes.Buffer
	view := NewHookView(HookViewParams{W: &buf})
	view.OnHook(domain.HookBeat{Cmd: "seq", Started: true})

	drawn := strings.Count(buf.String(), "\n")
	for i := 0; i < domain.HookViewTailLines+3; i++ {
		buf.Reset()
		_, _ = view.Write([]byte("a line of hook output\n"))

		up := 0
		if _, err := fmt.Sscanf(buf.String(), domain.AnsiCursorUpFmt, &up); err != nil {
			t.Fatalf("repaint %d moved no cursor: %q", i, buf.String())
		}
		if up != drawn {
			t.Fatalf("repaint %d moved back %d rows, want the %d it had printed", i, up, drawn)
		}
		drawn = strings.Count(buf.String(), "\n")
	}
}

// A tab is a jump to the next stop, so a line holding one is wider than its
// runes say — and a line drawn wider than the terminal wraps, which is what
// makes the cursor arithmetic above wrong.
func TestHookViewMeasuresATabAsTheColumnsItTakes(t *testing.T) {
	got := styles.ExpandTabs("a\tb")
	if strings.Contains(got, "\t") || styles.VisibleWidth(got) != 2+domain.TabWidth {
		t.Errorf("ExpandTabs(%q) = %q, want the tab spelled out", "a\tb", got)
	}
}

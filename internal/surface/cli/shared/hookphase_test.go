package shared

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// A hook writes where the command was told to write. A surface that leaves the
// sink nil sends it to os.Stderr instead — which under --quiet is the one stream
// the run was asked not to use.
func TestDrawHookPhaseAlwaysHandsTheSinkAStream(t *testing.T) {
	for _, human := range []bool{true, false} {
		var buf bytes.Buffer
		var got io.Writer
		err := DrawHookPhase(DrawHookPhaseParams{
			Stderr: &buf,
			Human:  human,
			Title:  domain.HooksTitleOnCreate,
			Run: func(sink flow.HookSink) error {
				got = sink.Output
				return nil
			},
		})
		if err != nil {
			t.Fatalf("DrawHookPhase(human=%v) = %v", human, err)
		}
		if got == nil {
			t.Fatalf("human=%v: the phase ran with no sink, so the hook falls back to os.Stderr", human)
		}
		if _, writeErr := io.WriteString(got, "from the hook\n"); writeErr != nil {
			t.Fatalf("human=%v: write to sink: %v", human, writeErr)
		}
		if !strings.Contains(buf.String(), "from the hook") {
			t.Errorf("human=%v: the hook's output did not reach the command's own stream", human)
		}
	}
}

// The record of a hook is kept for the reader who could not watch it: a pipe, a
// CI log, a JSON run, a quiet one. Those are precisely the runs a terminal-only
// log would leave with nothing.
func TestDrawHookPhaseKeepsTheLogOnANonTerminal(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "on_create"+domain.HooksLogFileExt)
	var buf bytes.Buffer

	err := DrawHookPhase(DrawHookPhaseParams{
		Stderr:  &buf,
		Human:   true,
		Title:   domain.HooksTitleOnCreate,
		LogPath: logPath,
		Run: func(sink flow.HookSink) error {
			_, writeErr := io.WriteString(sink.Output, "resolving\nfetching\n")
			return writeErr
		},
	})
	if err != nil {
		t.Fatalf("DrawHookPhase() = %v", err)
	}

	body, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read log: %v", readErr)
	}
	if !strings.Contains(string(body), "fetching") {
		t.Errorf("log holds %q, want the whole stream", body)
	}
}

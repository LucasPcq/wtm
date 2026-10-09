package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// The user opening a second worktree wants one thing from the start line: the
// port their copy is on.
func TestRunPrinterReportsBoundPorts(t *testing.T) {
	var out, errOut bytes.Buffer
	printer := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut})

	printer.Emit(runlogs.Event{
		Phase: runlogs.PhaseStarted,
		Job:   "web",
		Ports: map[string]int{"PORT": 3010},
	})

	if !strings.Contains(out.String(), "web started · :3010") {
		t.Errorf("got %q", out.String())
	}
}

func TestRunPrinterLeavesAPortlessJobAlone(t *testing.T) {
	var out, errOut bytes.Buffer
	printer := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut})

	printer.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "web"})

	if strings.Contains(out.String(), "·") {
		t.Errorf("nothing to say should say nothing, got %q", out.String())
	}
}

func TestRunPrinterShowsURL(t *testing.T) {
	var out, errOut bytes.Buffer
	p := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut})
	p.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "web", Ports: map[string]int{"PORT": 3010}, URL: "http://localhost:3010"})

	if !strings.Contains(out.String(), "http://localhost:3010") {
		t.Errorf("started line must carry the URL, got %q", out.String())
	}
	if strings.Contains(out.String(), "\x1b]8;;") {
		t.Errorf("hyperlinks are off by default, got %q", out.String())
	}
}

func TestRunPrinterReportsMissingDevOrigins(t *testing.T) {
	var out, errOut bytes.Buffer
	p := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut})
	p.Emit(runlogs.Event{
		Phase: runlogs.PhaseStarted,
		Job:   "web",
		URL:   "http://web.feat.myapp.localhost:4000",
		DevOrigins: []domain.DevOriginFix{{
			Job:    "web",
			Config: "apps/web/next.config.ts",
			Line:   "web: add allowedDevOrigins to apps/web/next.config.ts",
		}},
	})

	// A finding about a job that started fine goes where the ports report goes:
	// on stderr, below the line that says it is up.
	if !strings.Contains(errOut.String(), "allowedDevOrigins") {
		t.Errorf("stderr = %q, want the missing line named", errOut.String())
	}
	if strings.Contains(out.String(), "allowedDevOrigins") {
		t.Errorf("stdout = %q, want the finding kept off the machine-readable stream", out.String())
	}
}

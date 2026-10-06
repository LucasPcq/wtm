package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

func (d *fakeDaemon) requestsOf(action process.RequestAction) []process.Request {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []process.Request
	for _, req := range d.requests {
		if req.Action == action {
			out = append(out, req)
		}
	}
	return out
}

// The daemon publishes a job's events long after `run up` returned: the
// repository and correlation id they carry travel with the start.
func TestRunUpHandsTheDaemonWhatItsJobEventsCarry(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)
	enterWorktree(t, os.Getenv(domain.EnvProjectDir))
	t.Setenv(domain.EnvCorrelationID, "popup-9")

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run up: %v", err)
	}

	starts := daemon.requestsOf(process.ActionStart)
	if len(starts) == 0 {
		t.Fatal("nothing was started")
	}
	commonDir, err := filepath.EvalSymlinks(filepath.Join(os.Getenv(domain.EnvProjectDir), ".git"))
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range starts {
		if req.Origin == nil || req.Origin.CorrelationID != "popup-9" || req.Origin.Repo.CommonDir != commonDir {
			t.Fatalf("start of %s carries origin %+v, want popup-9 in %s", req.Job.Name, req.Origin, commonDir)
		}
	}
}

func TestRunDownHandsTheDaemonItsCorrelationID(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	runningHere(t, daemon, "api")
	fakeTTY(t, false)
	t.Setenv(domain.EnvCorrelationID, "popup-10")

	if _, _, err := runCmd(t, domain.CmdDown, "--"+domain.FlagProfile, "dev", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("run down: %v", err)
	}

	stops := daemon.requestsOf(process.ActionStop)
	if len(stops) != 1 || stops[0].Origin == nil || stops[0].Origin.CorrelationID != "popup-10" {
		t.Fatalf("stops = %+v, want one carrying popup-10", stops)
	}
}

func TestRunStopHandsTheDaemonItsCorrelationID(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	runningHere(t, daemon, "api")
	fakeTTY(t, false)
	t.Setenv(domain.EnvCorrelationID, "popup-11")

	if _, _, err := runCmd(t, domain.CmdStop, "--"+domain.FlagJob, "api", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("run stop: %v", err)
	}

	stops := daemon.requestsOf(process.ActionStop)
	if len(stops) != 1 || stops[0].Origin == nil || stops[0].Origin.CorrelationID != "popup-11" {
		t.Fatalf("stops = %+v, want one carrying popup-11", stops)
	}
}

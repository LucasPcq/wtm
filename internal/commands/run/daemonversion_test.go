package run

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// TestClientRefusesADaemonOfAnotherBuild covers the failure that looks like
// success: the daemon runs the jobs, so an older one keeps applying its own
// behaviour while the new client prints what it believes.
func TestClientRefusesADaemonOfAnotherBuild(t *testing.T) {
	startFakeDaemon(t, &fakeDaemon{Version: "0.27.0"})

	_, err := process.NewClient(process.SocketPath()).Send(t.Context(), process.Request{Action: process.ActionStart, Job: &apiJob})

	if !errors.Is(err, domain.ErrDaemonVersionMismatch) {
		t.Fatalf("error = %v, want a version mismatch", err)
	}
	for _, want := range []string{"0.27.0", domain.Version, "wtm run daemon restart"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not name %q", err, want)
		}
	}
}

// TestClientReadsAnUnstampedAnswerAsOlder covers the daemon predating the
// handshake itself: it cannot announce a version, and its silence is the
// divergence.
func TestClientReadsAnUnstampedAnswerAsOlder(t *testing.T) {
	startFakeDaemon(t, &fakeDaemon{Version: "none"})

	_, err := process.NewClient(process.SocketPath()).Send(t.Context(), process.Request{Action: process.ActionStart, Job: &apiJob})

	if !errors.Is(err, domain.ErrDaemonVersionMismatch) {
		t.Fatalf("error = %v, want a version mismatch", err)
	}
	if !strings.Contains(err.Error(), domain.DaemonVersionUnknown) {
		t.Errorf("message %q does not stand in for the missing version", err)
	}
}

func TestClientAcceptsADaemonOfTheSameBuild(t *testing.T) {
	startFakeDaemon(t, &fakeDaemon{})

	if _, err := process.NewClient(process.SocketPath()).Send(t.Context(), process.Request{Action: process.ActionList}); err != nil {
		t.Fatalf("the nominal path must cost nothing: %v", err)
	}
}

// An older daemon is still the one holding the jobs: listing it is how the user
// learns what a restart would cost, so `run ps` shows them and says why they
// may behave differently.
func TestPsListsAnOlderDaemonAndSaysSo(t *testing.T) {
	setupStartProject(t, &fakeDaemon{Version: "none", Jobs: []domain.JobInfo{{Name: "api", Status: domain.JobStatusRunning, WorkDir: "/w"}}})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdPs)
	if err != nil {
		t.Fatalf("run ps: %v", err)
	}
	for _, want := range []string{"api", domain.DaemonVersionUnknown, "wtm run daemon restart"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("run ps does not say %q:\n%s", want, stdout)
		}
	}
}

// Replacing a daemon that holds jobs would stop the foreground ones: a start is
// refused, naming how many and the way out.
func TestRunStartRefusesAnOlderDaemonHoldingJobs(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{Version: "0.27.1", Jobs: []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/elsewhere"}}})
	fakeTTY(t, false)

	_, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api", "--"+domain.FlagDetach)
	if !errors.Is(err, domain.ErrDaemonVersionMismatch) {
		t.Fatalf("error = %v, want a version mismatch", err)
	}
	for _, want := range []string{"0.27.1", "1 job", "wtm run daemon restart"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not say %q", err, want)
		}
	}
	if started := daemon.startedJobs(); len(started) != 0 {
		t.Errorf("started %v on an older daemon", started)
	}
}

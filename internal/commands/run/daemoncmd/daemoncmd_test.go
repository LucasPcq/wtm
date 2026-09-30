package daemoncmd_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/commands/run/daemoncmd"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := daemoncmd.NewCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.Execute()
	return stdout.String(), err
}

func decodeStatus(t *testing.T, stdout string) domain.DaemonStatus {
	t.Helper()
	var status domain.DaemonStatus
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("parse JSON: %v\n%s", err, stdout)
	}
	return status
}

func shutdowns(daemon *processtest.Daemon) int {
	count := 0
	for _, action := range daemon.Actions() {
		if strings.HasPrefix(action, "shutdown:") {
			count++
		}
	}
	return count
}

var mixed = []domain.JobInfo{
	{Name: "api", Status: domain.JobStatusRunning, WorkDir: "/w"},
	{Name: "db", Status: domain.JobStatusDetached, WorkDir: "/w"},
	{Name: "migrate", Status: domain.JobStatusStopped, WorkDir: "/w"},
}

func TestStatusWithNoDaemonSaysItIsNotRunning(t *testing.T) {
	globaldir.Isolate(t)

	stdout, err := execute(t, domain.CmdStatus, "--output", domain.OutputJSON)

	if err != nil {
		t.Fatalf("status: %v", err)
	}
	status := decodeStatus(t, stdout)
	if status.Running || status.Version != domain.Version || status.SocketPath == "" {
		t.Errorf("status = %+v, want not running, this build, the socket named", status)
	}
}

// The status is the daemon's own count of what it holds, split by what a stop
// would cost: a job that has stopped costs nothing.
func TestStatusCountsWhatTheDaemonHolds(t *testing.T) {
	processtest.Serve(t, mixed)

	stdout, err := execute(t, domain.CmdStatus, "--output", domain.OutputJSON)

	if err != nil {
		t.Fatalf("status: %v", err)
	}
	status := decodeStatus(t, stdout)
	if !status.Running || status.Foreground != 1 || status.Detached != 1 {
		t.Errorf("status = %+v, want running with one foreground and one detached", status)
	}
	if status.PID == 0 {
		t.Error("pid = 0, want the process holding the socket")
	}
}

// A daemon from another build is reported, not refused: saying so is what the
// command exists for.
func TestStatusReportsADaemonFromAnotherBuild(t *testing.T) {
	daemon := processtest.Serve(t, nil)
	daemon.Version = "v0.27.0"

	stdout, err := execute(t, domain.CmdStatus, "--output", domain.OutputJSON)

	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status := decodeStatus(t, stdout); status.DaemonVersion != "v0.27.0" || status.Version != domain.Version {
		t.Errorf("status = %+v, want both builds named", status)
	}
}

// Nothing was stopped, so nothing succeeded: the line is a non-event.
func TestStopWithNoDaemonSaysSoAndSucceeds(t *testing.T) {
	globaldir.Isolate(t)

	stdout, err := execute(t, domain.CmdStop)

	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !strings.Contains(stdout, domain.DaemonAlreadyStopped) {
		t.Errorf("stdout = %q, want %q", stdout, domain.DaemonAlreadyStopped)
	}
}

// A daemon holding only detached stacks costs nothing to stop: they outlive it.
func TestStopOfADaemonHoldingNothingForegroundAsksNothing(t *testing.T) {
	daemon := processtest.Serve(t, []domain.JobInfo{{Name: "db", Status: domain.JobStatusDetached, WorkDir: "/w"}})

	stdout, err := execute(t, domain.CmdStop, "--output", domain.OutputJSON)

	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if shutdowns(daemon) != 1 {
		t.Errorf("requests = %v, want one shutdown", daemon.Actions())
	}
	if decodeStatus(t, stdout).Running {
		t.Error("the document says the daemon still runs after its stop")
	}
}

// Foreground services die with the daemon: nobody can be asked, so the stop is
// refused naming --yes, and the daemon is left alone.
func TestStopThatWouldKillForegroundServicesRequiresYes(t *testing.T) {
	daemon := processtest.Serve(t, mixed)

	_, err := execute(t, domain.CmdStop, "--output", domain.OutputJSON)

	if err == nil || !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Fatalf("err = %v, want --%s named", err, domain.FlagYes)
	}
	if shutdowns(daemon) != 0 {
		t.Errorf("requests = %v, want the daemon left alone", daemon.Actions())
	}
}

func TestStopWithYesStopsADaemonHoldingForegroundServices(t *testing.T) {
	daemon := processtest.Serve(t, mixed)

	stdout, err := execute(t, domain.CmdStop, "--"+domain.FlagYes)

	if err != nil {
		t.Fatalf("stop --yes: %v", err)
	}
	if shutdowns(daemon) != 1 {
		t.Errorf("requests = %v, want one shutdown", daemon.Actions())
	}
	if !strings.Contains(stdout, domain.DaemonStopped) {
		t.Errorf("stdout = %q, want %q", stdout, domain.DaemonStopped)
	}
}

// A restart stops before it starts: refused, it has touched nothing.
func TestRestartThatWouldKillForegroundServicesRequiresYes(t *testing.T) {
	daemon := processtest.Serve(t, mixed)

	_, err := execute(t, domain.CmdRestart, "--output", domain.OutputJSON)

	if err == nil || !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Fatalf("err = %v, want --%s named", err, domain.FlagYes)
	}
	if shutdowns(daemon) != 0 {
		t.Errorf("requests = %v, want the daemon left alone", daemon.Actions())
	}
}

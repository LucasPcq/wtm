package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// startRealDaemon runs the daemon in-process on the socket path the commands
// compute, so EnsureDaemon finds it already up. The forked binary the real
// EnsureDaemon starts is the test binary here, which has no `daemon` command.
func startRealDaemon(t *testing.T) {
	t.Helper()
	shortHome(t)

	sock := process.SocketPath()
	go func() { _ = process.RunDaemon(process.DaemonParams{SocketPath: sock}) }()
	t.Cleanup(func() { _, _, _ = runCmd(t, domain.CmdDown) })

	deadline := time.Now().Add(5 * time.Second)
	for !process.IsDaemonRunning(sock) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !process.IsDaemonRunning(sock) {
		t.Fatal("daemon did not come up")
	}
}

// gatedJobs is the shape LUC-208 was reported on: a task the next service
// depends on. The service records whether the task had run by the time it
// started, which is the only thing that tells an ordered run from a truncated
// one.
func gatedJobs(work string) []domain.JobConfig {
	marker := filepath.Join(work, "marker")
	result := filepath.Join(work, "result")
	return []domain.JobConfig{
		{Name: "migrate", Kind: domain.JobKindTask, Cmd: "touch " + marker, Cwd: work},
		{Name: "api", Kind: domain.JobKindService, Cwd: work, Cmd: fmt.Sprintf(
			"if [ -f %s ]; then echo READY > %s; else echo MISSING > %s; fi; sleep 5",
			marker, result, result)},
	}
}

func gateResult(t *testing.T, work string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(filepath.Join(work, "result")); err == nil {
			return strings.TrimSpace(string(data))
		}
		time.Sleep(20 * time.Millisecond)
	}
	return ""
}

// Sans profil, `run up` ne gardait que les services : la task disparaissait du
// run, le compteur affichait [1/1], et le service démarrait sur une base
// qu'aucune migration n'avait touchée (LUC-208).
func TestRunUpStartsTasksWhenNoProfileIsDeclared(t *testing.T) {
	startRealDaemon(t)
	stateDir := setupTestProject(t)
	work := t.TempDir()
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: gatedJobs(work)})

	out, _, err := runCmd(t, domain.CmdUp, "-d")
	if err != nil {
		t.Fatalf("run up: %v", err)
	}

	if !strings.Contains(out, "[1/2] migrate") {
		t.Errorf("output does not run the task as step 1 of 2:\n%s", out)
	}
	if got := gateResult(t, work); got != "READY" {
		t.Errorf("service saw %q, want READY: la task doit passer avant lui", got)
	}
}

func TestRunUpNamesTheProfileItStarted(t *testing.T) {
	startRealDaemon(t)
	stateDir := setupTestProject(t)
	work := t.TempDir()
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs:     gatedJobs(work),
		Profiles: []domain.ProfileConfig{{Name: "backend", Jobs: []string{"migrate", "api"}, Default: true}},
	})

	out, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagProfile, "backend", "-d")
	if err != nil {
		t.Fatalf("run up: %v", err)
	}

	if !strings.Contains(out, "backend") {
		t.Errorf("nothing names the profile that was started:\n%s", out)
	}
	if got := gateResult(t, work); got != "READY" {
		t.Errorf("service saw %q, want READY", got)
	}
}

package process

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// A job's cmd is a shell line, so what a terminal would do with it is what the
// job does: quotes group, operators chain, and a declared port reaches a server
// that only takes it as a CLI flag.
func TestManagerStartTask_RunsCmdThroughTheShell(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		env  map[string]string
		want string
	}{
		{name: "quotes", cmd: `echo "one two"`, want: "one two"},
		{name: "operators", cmd: "echo first && echo second", want: "second"},
		{name: "pipe", cmd: "echo piped | tr a-z A-Z", want: "PIPED"},
		{
			name: "declared port expands",
			cmd:  "echo --port ${PORT}",
			env:  map[string]string{domain.EnvPortOffset: "10"},
			want: "--port 3010",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newManager()
			var buf bytes.Buffer
			job := domain.JobConfig{
				Name:  "probe",
				Kind:  domain.JobKindTask,
				Cmd:   c.cmd,
				Ports: map[string]int{"PORT": 3000},
			}

			if err := m.Start(StartParams{Job: job, WorkDir: t.TempDir(), Env: c.env, Streamer: &buf}); err != nil {
				t.Fatalf("start task: %v", err)
			}
			if !strings.Contains(buf.String(), c.want) {
				t.Errorf("output %q does not contain %q", buf.String(), c.want)
			}
		})
	}
}

func TestManagerStartTask_BlankCmdRejected(t *testing.T) {
	m := newManager()
	job := domain.JobConfig{Name: "empty", Kind: domain.JobKindTask, Cmd: "   "}

	err := m.Start(StartParams{Job: job, WorkDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected a blank cmd to be refused")
	}
	if !strings.Contains(err.Error(), "empty cmd") {
		t.Errorf("got %v, want an empty-cmd error", err)
	}
}

// A stop command is a shell line too, and runs with the ports its cmd ran with.
func TestManagerStop_RunsStopThroughTheShell(t *testing.T) {
	m := newManager()
	dir := t.TempDir()
	out := filepath.Join(dir, "stopped")

	job := domain.JobConfig{
		Name:  "compose",
		Kind:  domain.JobKindService,
		Cmd:   "echo up",
		Stop:  "echo ${PORT} > " + out,
		Ports: map[string]int{"PORT": 3000},
	}
	if err := m.Start(StartParams{Job: job, WorkDir: dir, Env: map[string]string{domain.EnvPortOffset: "10"}}); err != nil {
		t.Fatalf("start detached: %v", err)
	}
	if err := m.Stop(JobRef{Name: "compose", WorkDir: dir}); err != nil {
		t.Fatalf("stop: %v", err)
	}

	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("stop command did not redirect: %v", err)
	}
	if got := strings.TrimSpace(string(written)); got != "3010" {
		t.Errorf("stop ran with %q, want the resolved port 3010", got)
	}
}

// A stop command that is only whitespace is not a command: the service must
// still be signalled, not silently marked stopped.
func TestManagerStop_BlankStopFallsBackToSignal(t *testing.T) {
	m := newManager()
	dir := t.TempDir()

	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "sleep 30", Stop: "   "}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start service: %v", err)
	}
	if err := m.Stop(JobRef{Name: "server", WorkDir: dir}); err != nil {
		t.Fatalf("stop: %v", err)
	}

	jobs := m.List()
	if len(jobs) != 1 || jobs[0].Status != domain.JobStatusStopped {
		t.Fatalf("expected the job stopped, got %+v", jobs)
	}
	if jobs[0].ExitCode == nil {
		t.Error("expected the process to have been reaped, meaning it was really signalled")
	}
}

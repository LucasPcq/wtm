package run

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// A runner binds nothing itself: the addresses a reader came for are its
// apps', and `run ps` names them under its row and in its JSON.
func setupRunnerProject(t *testing.T) {
	t.Helper()
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Addressing: domain.AddressingPorts,
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "turbo run dev", Runs: []string{"web"}, BindsNoPort: true},
			{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
		},
	})
	daemon := startFakeDaemon(t, &fakeDaemon{})
	runningHere(t, daemon, "dev")
}

func TestRunPsJSONNamesTheAppsARunnerHolds(t *testing.T) {
	setupRunnerProject(t)
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdPs, "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("run ps: %v", err)
	}
	var rows []domain.RunningJob
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(rows) != 1 || len(rows[0].Held) != 1 {
		t.Fatalf("rows = %+v, want dev holding web", rows)
	}
	if held := rows[0].Held[0]; held.Job != "web" || !strings.Contains(held.URL, "3000") {
		t.Errorf("held = %+v, want web on port 3000", held)
	}
}

func TestRunPsListsTheAppsUnderTheRunnerRow(t *testing.T) {
	setupRunnerProject(t)
	fakeTTY(t, true)

	stdout, _, err := runCmd(t, domain.CmdPs)
	if err != nil {
		t.Fatalf("run ps: %v", err)
	}
	lines := strings.Split(stdout, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "turbo") && strings.Contains(line, " dev ") && i+1 < len(lines) {
			if next := lines[i+1]; !strings.Contains(next, "web") || !strings.Contains(next, "3000") {
				t.Errorf("the line under dev is %q, want web and its address\n%s", next, stdout)
			}
			return
		}
	}
	t.Errorf("no dev row:\n%s", stdout)
}

func TestRunPsListsOneWorktreeAtATime(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{apiJob}})
	daemon := startFakeDaemon(t, &fakeDaemon{})
	daemon.setJobs([]domain.JobInfo{
		{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/b"},
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: "/wt/a"},
		{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/a"},
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: "/wt/b"},
	})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdPs, "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("run ps: %v", err)
	}
	var rows []domain.RunningJob
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	var got []string
	for _, row := range rows {
		got = append(got, row.Path+" "+row.Name)
	}
	if strings.Join(got, ",") != "/wt/a api,/wt/a web,/wt/b api,/wt/b web" {
		t.Errorf("rows in order %v, want worktree by worktree", got)
	}
}

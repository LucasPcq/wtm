package run

import (
	"encoding/json"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// A name run.toml does not declare is refused with exit code 14 before anything
// is asked of the daemon: nothing is started, woken or stopped on a typo.
func TestUndeclaredNamesAreRefusedBeforeTheDaemon(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"up profile", []string{domain.CmdUp, "--" + domain.FlagProfile, "nope", "--" + domain.FlagYes, "--" + domain.FlagOutput, domain.OutputJSON}},
		{"down profile", []string{domain.CmdDown, "--" + domain.FlagProfile, "nope", "--" + domain.FlagYes, "--" + domain.FlagOutput, domain.OutputJSON}},
		{"start job", []string{domain.CmdStart, "--" + domain.FlagJob, "nope", "--" + domain.FlagYes, "--" + domain.FlagOutput, domain.OutputJSON}},
		{"stop job", []string{domain.CmdStop, "--" + domain.FlagJob, "nope", "--" + domain.FlagYes, "--" + domain.FlagOutput, domain.OutputJSON}},
		{"logs job", []string{domain.CmdLogs, "--" + domain.FlagJob, "nope", "--" + domain.FlagYes, "--" + domain.FlagOutput, domain.OutputJSON}},
		{"url job", []string{domain.CmdURL, "--" + domain.FlagJob, "nope", "--" + domain.FlagOutput, domain.OutputJSON}},
		{"open job", []string{domain.CmdOpen, "--" + domain.FlagJob, "nope", "--" + domain.FlagYes, "--" + domain.FlagOutput, domain.OutputJSON}},
		{"export profile", []string{domain.CmdExport, "--" + domain.FlagProfile, "nope"}},
		{"job rm", []string{domain.CmdJob, domain.CmdRm, "nope", "--" + domain.FlagYes, "--" + domain.FlagOutput, domain.OutputJSON}},
		{"profile rm", []string{domain.CmdProfile, domain.CmdRm, "nope", "--" + domain.FlagYes, "--" + domain.FlagOutput, domain.OutputJSON}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			daemon := setupStartProject(t, &fakeDaemon{})
			fakeTTY(t, false)
			stubOpener(t)

			_, _, err := runCmd(t, tc.args...)
			if got := rules.ExitCode(err); got != domain.ExitCodeNotDeclared {
				t.Fatalf("exit code = %d (%v), want %d", got, err, domain.ExitCodeNotDeclared)
			}
			if actions := daemon.actions(); len(actions) != 0 {
				t.Errorf("the daemon was contacted: %v", actions)
			}
		})
	}
}

func TestRunPsJSONNamesTheWorktreeAndItsProject(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	main := runningHere(t, daemon, "api")
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdPs, "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("run ps: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %v, want api", rows)
	}
	row := rows[0]
	if row["name"] != "api" || row["branch"] != "main" || row["path"] != main || row["project"] == "" || row["project"] == nil {
		t.Errorf("row = %v, want api in main, by branch, path and project", row)
	}
	if _, stale := row["work_dir"]; stale {
		t.Errorf("row still carries work_dir: %v", row)
	}
}

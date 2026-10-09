package run

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

var listed = domain.RunConfig{
	Jobs: []domain.JobConfig{
		{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		{Name: "build", Kind: domain.JobKindTask, Cmd: "make"},
	},
	Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"api"}, Default: true}},
}

// A listing nobody can pick from is a listing: under --yes, or with no
// terminal, the table is the whole answer and nothing is asked.
func TestTheCRUDListsPrintTheTableWhenNobodyCanPick(t *testing.T) {
	cases := map[string]struct {
		args []string
		tty  bool
		want []string
	}{
		"job list --yes":        {args: []string{domain.CmdJob, domain.CmdList, "--" + domain.FlagYes}, tty: true, want: []string{"api", "build"}},
		"job list, no terminal": {args: []string{domain.CmdJob, domain.CmdList}, want: []string{"api", "build"}},
		"profile list --yes":    {args: []string{domain.CmdProfile, domain.CmdList, "--" + domain.FlagYes}, tty: true, want: []string{"dev"}},
		"profile list, no tty":  {args: []string{domain.CmdProfile, domain.CmdList}, want: []string{"dev"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stateDir := setupTestProject(t)
			writeRunTOML(t, stateDir, listed)
			fakeTTY(t, c.tty)

			stdout, _, err := runCmd(t, c.args...)

			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			for _, want := range c.want {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout is missing %q\n%s", want, stdout)
				}
			}
		})
	}
}

// A run.toml with jobs and no profile still answers the profile listing: an
// empty one says so rather than printing nothing.
func TestAnEmptyProfileListingSaysSo(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: listed.Jobs})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdProfile, domain.CmdList)

	if err != nil {
		t.Fatalf("profile list: %v", err)
	}
	if !strings.Contains(stdout, domain.RunProfilesEmpty) {
		t.Errorf("stdout = %q, want %q", stdout, domain.RunProfilesEmpty)
	}
}

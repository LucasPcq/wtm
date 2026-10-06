package wt

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

const dbPassword = "db-s3cr3t"

// databaseURLRepo links a URL holding a password to the web port, so the
// value wtm rewrites is one a report must not print whole.
func databaseURLRepo(t *testing.T) string {
	t.Helper()
	globaldir.Isolate(t)
	dir := isolationRepo(t)
	if err := config.WriteRun(config.WriteRunParams{StateDir: filepath.Join(dir, ".git", "wtm"), Force: true, Config: domain.RunConfig{
		Jobs: []domain.JobConfig{{
			Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev",
			Ports: map[string]int{"PORT": 3000},
		}},
		EnvPorts: []domain.EnvPortLink{
			{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"},
			{File: ".env", Key: "DATABASE_URL", Job: "web", Port: "PORT"},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	writeEnvFile(t, filepath.Join(dir, ".env"), mainEnv+"DATABASE_URL=postgres://app:"+dbPassword+"@localhost:3000/db\n")
	return dir
}

func TestCreateJSONMasksThePasswordOfAPortLinkedURL(t *testing.T) {
	databaseURLRepo(t)

	stdout, _, err := runWtCmd(t, domain.CmdCreate, "feat/a", "--from", "main", "--yes", "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if strings.Contains(stdout, dbPassword) {
		t.Errorf("create JSON carries the password:\n%s", stdout)
	}
	if !strings.Contains(stdout, "postgres://app:***@localhost:3010/db") {
		t.Errorf("create JSON lacks the rewritten URL with its password masked:\n%s", stdout)
	}
}

func TestEnvMasksThePasswordOfAPortLinkedURLUnlessShowValues(t *testing.T) {
	dir := databaseURLRepo(t)
	envCreate("feat/a", "--from", "main", "--yes")(t, dir)
	json := []string{"--" + domain.FlagOutput, domain.OutputJSON}
	check := "--" + domain.FlagCheck

	for name, args := range map[string][]string{
		"check text": {"feat/a", check},
		"check json": append([]string{"feat/a", check}, json...),
	} {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, _ := runWtCmd(t, append([]string{domain.CmdEnv}, args...)...)
			if strings.Contains(stdout+stderr, dbPassword) {
				t.Errorf("output carries the password:\n%s%s", stdout, stderr)
			}
			if name == "check json" && !strings.Contains(stdout, `"current_value": "postgres://app:***@localhost:3010/db"`) {
				t.Errorf("JSON lacks the URL with its password masked:\n%s", stdout)
			}
		})
	}

	stdout, _, _ := runWtCmd(t, append([]string{domain.CmdEnv, "feat/a", check, "--" + domain.FlagShowValues}, json...)...)
	if !strings.Contains(stdout, "postgres://app:"+dbPassword+"@localhost:3010/db") {
		t.Errorf("--show-values lacks the full URL:\n%s", stdout)
	}
}

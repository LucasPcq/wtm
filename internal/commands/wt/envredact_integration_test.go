package wt

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
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
	return linkedSecretRepo(t, map[string]string{"DATABASE_URL": "postgres://app:" + dbPassword + "@localhost:3000/db"})
}

// linkedSecretRepo links every given key to the web port, each value holding
// a password on the 3000 it follows.
func linkedSecretRepo(t *testing.T, values map[string]string) string {
	t.Helper()
	globaldir.Isolate(t)
	dir := isolationRepo(t)
	if err := config.WriteRun(config.WriteRunParams{StateDir: filepath.Join(dir, ".git", "wtm"), Force: true, Config: domain.RunConfig{
		Jobs: []domain.JobConfig{{
			Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev",
			Ports: map[string]int{"PORT": 3000},
		}},
		EnvPorts:  envPortLinks(values),
		EnvValues: []domain.EnvValueLink{{File: ".env", Key: "REALM", Job: "web", Value: "app-{worktree}"}},
	}}); err != nil {
		t.Fatal(err)
	}
	body := mainEnv
	for _, key := range slices.Sorted(maps.Keys(values)) {
		body += key + "=" + values[key] + "\n"
	}
	writeEnvFile(t, filepath.Join(dir, ".env"), body)
	return dir
}

func envPortLinks(values map[string]string) []domain.EnvPortLink {
	links := []domain.EnvPortLink{{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"}}
	for _, key := range slices.Sorted(maps.Keys(values)) {
		links = append(links, domain.EnvPortLink{File: ".env", Key: key, Job: "web", Port: "PORT"})
	}
	return links
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

func jsonArgs(args ...string) []string {
	return append(args, "--yes", "--"+domain.FlagOutput, domain.OutputJSON)
}

func assertNoSecret(t *testing.T, label, out string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(out, secret) {
			t.Errorf("%s carries %q:\n%s", label, secret, out)
		}
	}
}

// LUC-274 F1: a switch to verbatim reported the source's URL it put back
// with its password.
func TestEnvVerbatimJSONMasksTheRestoredPassword(t *testing.T) {
	dir := databaseURLRepo(t)
	envCreate("feat/a", "--from", "main", "--yes")(t, dir)

	stdout, stderr, err := runWtCmd(t, jsonArgs(domain.CmdEnv, "feat/a", "--"+domain.FlagIsolation, string(domain.IsolationVerbatim))...)
	if err != nil {
		t.Fatalf("env --isolation verbatim: %v\n%s", err, stderr)
	}

	assertNoSecret(t, "verbatim JSON", stdout+stderr, dbPassword)
	if !strings.Contains(stdout, `"to": "postgres://app:***@localhost:3000/db"`) {
		t.Errorf("verbatim JSON lacks the restored URL with its password masked:\n%s", stdout)
	}
}

// LUC-274 F2: the current value of an [[env]] key is whatever the user left
// there before wtm writes its own.
func TestEnvCheckJSONMasksThePasswordOfAnOwnedKey(t *testing.T) {
	const ownedPassword = "owned-s3cr3t"
	dir := databaseURLRepo(t)
	envCreate("feat/a", "--from", "main", "--yes")(t, dir)
	path := worktreeEnvPath(dir, "feat/a")
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeEnvFile(t, path, strings.Replace(string(current), "REALM=app-feat-a", "REALM=postgres://app:"+ownedPassword+"@localhost:5432/app", 1))

	stdout, stderr, _ := runWtCmd(t, jsonArgs(domain.CmdEnv, "feat/a", "--"+domain.FlagCheck)...)

	assertNoSecret(t, "check JSON", stdout+stderr, ownedPassword)
	if !strings.Contains(stdout, "postgres://app:***@localhost:5432/app") {
		t.Errorf("check JSON lacks the owned value with its password masked:\n%s", stdout)
	}
}

// LUC-274 F3: the shapes net/url cannot read leaked from every surface that
// prints a port-linked value — create JSON, env JSON, and the text conflict line.
func TestEveryEnvSurfaceMasksThePasswordsNetURLCannotRead(t *testing.T) {
	const mongoPassword, dsnPassword, hashPassword = "mongo-s3cr3t", "dsn-s3cr3t", "hash-s3cr3t"
	dir := linkedSecretRepo(t, map[string]string{
		"MONGO_URL":    "mongodb://app:" + mongoPassword + "@localhost:3000,localhost:27018/db",
		"PG_DSN":       "host=localhost port=3000 password=" + dsnPassword + " dbname=app",
		"DATABASE_URL": "postgres://app:pa#" + hashPassword + "@localhost:3000/db",
	})
	secrets := []string{mongoPassword, dsnPassword, hashPassword}

	stdout, stderr, err := runWtCmd(t, jsonArgs(domain.CmdCreate, "feat/a", "--from", "main")...)
	if err != nil {
		t.Fatalf("create: %v\n%s", err, stderr)
	}
	assertNoSecret(t, "create JSON", stdout+stderr, secrets...)

	path := worktreeEnvPath(dir, "feat/a")
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeEnvFile(t, path, strings.ReplaceAll(string(current), "s3cr3t", "s3cr3t-local"))

	refresh := []string{domain.CmdEnv, "feat/a", "--" + domain.FlagCheck, "--" + domain.FlagMode, string(domain.EnvModeRefresh)}
	stdout, stderr, _ = runWtCmd(t, jsonArgs(refresh...)...)
	assertNoSecret(t, "check JSON", stdout+stderr, secrets...)

	stdout, stderr, _ = runWtCmd(t, refresh...)
	assertNoSecret(t, "check text", stdout+stderr, secrets...)
	if !strings.Contains(stdout, "conflict") {
		t.Errorf("check text shows no conflict line to test:\n%s", stdout)
	}

	stdout, _, _ = runWtCmd(t, jsonArgs(append(refresh, "--"+domain.FlagShowValues)...)...)
	for _, secret := range secrets {
		if !strings.Contains(stdout, secret) {
			t.Errorf("--show-values lacks %q:\n%s", secret, stdout)
		}
	}
}

// LUC-274: a verbatim switch's row for an origin list was elided from the left
// down to its last origin, the same on both sides, hiding the port that moved.
func TestEnvVerbatimTextShowsTheMoveInAList(t *testing.T) {
	dir := linkedSecretRepo(t, map[string]string{"ORIGINS": "http://localhost:3000,http://a:" + dbPassword + "@localhost:3001"})
	envCreate("feat/a", "--from", "main", "--yes")(t, dir)

	stdout, stderr, err := runWtCmd(t, domain.CmdEnv, "feat/a", "--yes", "--"+domain.FlagIsolation, string(domain.IsolationVerbatim))
	if err != nil {
		t.Fatalf("env --isolation verbatim: %v\n%s", err, stderr)
	}

	assertNoSecret(t, "verbatim text", stdout+stderr, dbPassword)
	if !strings.Contains(stdout, `back to the source's "http://localhost:3000,`) || !strings.Contains(stdout, `(was "http://localhost:3010,`) {
		t.Errorf("verbatim text hides the move:\n%s", stdout)
	}
}

// LUC-274: a check whose only finding was a hand-edited [[env]] key named
// nothing.
func TestEnvCheckTextNamesAHandEditedOwnedKey(t *testing.T) {
	dir := databaseURLRepo(t)
	envCreate("feat/a", "--from", "main", "--yes")(t, dir)
	path := worktreeEnvPath(dir, "feat/a")
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeEnvFile(t, path, strings.Replace(string(current), "REALM=app-feat-a", "REALM=edited", 1))

	stdout, _, _ := runWtCmd(t, domain.CmdEnv, "feat/a", "--"+domain.FlagCheck)

	if !strings.Contains(stdout, `REALM  would be set to wtm's value "app-feat-a"`) {
		t.Errorf("check text does not name REALM:\n%s", stdout)
	}
}

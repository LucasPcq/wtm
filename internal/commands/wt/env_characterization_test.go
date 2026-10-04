package wt

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

// These goldens pin `wtm env` as it behaved before its move to internal/flow
// (LUC-239). They are not edited by the move: a diff here is a behaviour change.

var updateEnvGolden = flag.Bool("update-env-golden", false, "rewrite the wtm env golden files")

type envGoldenCase struct {
	name string
	// setup prepares the repository once isolationRepo has; it returns nothing
	// and fails the test on its own.
	setup func(t *testing.T, dir string)
	args  []string
	// branch is the worktree whose .env and isolation the golden records.
	branch string
}

func TestEnvCharacterization(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range envGoldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			got := runEnvGolden(t, tc)
			path := filepath.Join(wd, "testdata", "envgolden", tc.name+".golden")
			if *updateEnvGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update-env-golden to create it): %v", err)
			}
			if got != string(want) {
				t.Errorf("wtm env output drifted from %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
			}
		})
	}
}

func runEnvGolden(t *testing.T, tc envGoldenCase) string {
	t.Helper()
	globaldir.Isolate(t)
	dir := isolationRepo(t)
	if tc.setup != nil {
		tc.setup(t, dir)
	}

	stdout, stderr, err := runWtCmd(t, append([]string{domain.CmdEnv}, tc.args...)...)

	var b strings.Builder
	b.WriteString("args: " + strings.Join(tc.args, " ") + "\n")
	b.WriteString("err: ")
	if err != nil {
		b.WriteString(err.Error())
	}
	b.WriteString(fmt.Sprintf("\nexit: %d", rules.ExitCode(err)))
	b.WriteString("\n--- stdout ---\n" + stdout)
	b.WriteString("--- stderr ---\n" + stderr)
	if tc.branch != "" {
		b.WriteString("--- .env ---\n" + readEnvOf(t, dir, tc.branch))
		b.WriteString("--- isolation ---\n" + string(recordedIsolation(dir, tc.branch)) + "\n")
	}
	return normalizeEnvGolden(dir, b.String())
}

func readEnvOf(t *testing.T, dir, branch string) string {
	t.Helper()
	path := worktreeEnvPath(dir, branch)
	if branch == "main" {
		path = filepath.Join(dir, ".env")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "(unreadable: " + err.Error() + ")\n"
	}
	return string(body)
}

func normalizeEnvGolden(dir, text string) string {
	var paths []string
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		paths = append(paths, resolved)
	}
	paths = append(paths, dir)
	for _, p := range paths {
		text = strings.ReplaceAll(text, filepath.Dir(p)+"/.trees", "<trees>")
		text = strings.ReplaceAll(text, p, "<repo>")
	}
	return text
}

func envCreate(args ...string) func(t *testing.T, dir string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		if _, _, err := runWtCmd(t, append([]string{domain.CmdCreate}, args...)...); err != nil {
			t.Fatalf("create %v: %v", args, err)
		}
	}
}

func writeEnvFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// driftSetup leaves feat/a with every kind of drift against main: a key main
// gained (missing), a value that diverged (conflict) and a key main never had
// (orphan).
func driftSetup(t *testing.T, dir string) {
	t.Helper()
	envCreate("feat/a", "--from", "main", "--yes")(t, dir)
	writeEnvFile(t, filepath.Join(dir, ".env"), mainEnv+"NEW_KEY=1\nSHARED=from-main\n")
	wt := readEnvOf(t, dir, "feat/a")
	writeEnvFile(t, worktreeEnvPath(dir, "feat/a"), wt+"SHARED=from-worktree\nORPHAN=1\n")
}

func noFilesSetup(t *testing.T, dir string) {
	t.Helper()
	if err := setupMinimalConfig(t, filepath.Join(dir, ".git", "wtm")); err != nil {
		t.Fatal(err)
	}
}

func envGoldenCases() []envGoldenCase {
	yes := "--" + domain.FlagYes
	json := []string{"--" + domain.FlagOutput, domain.OutputJSON}
	isolation := "--" + domain.FlagIsolation
	return []envGoldenCase{
		{name: "no-drift", setup: envCreate("feat/a", "--from", "main", "--yes"), args: []string{"feat/a", yes}, branch: "feat/a"},
		{name: "no-drift-json", setup: envCreate("feat/a", "--from", "main", "--yes"), args: append([]string{"feat/a", yes}, json...), branch: "feat/a"},
		{name: "drift-add", setup: driftSetup, args: []string{"feat/a", yes}, branch: "feat/a"},
		{name: "drift-add-json", setup: driftSetup, args: append([]string{"feat/a", yes}, json...), branch: "feat/a"},
		{name: "check", setup: driftSetup, args: []string{"feat/a", "--" + domain.FlagCheck}, branch: "feat/a"},
		{name: "check-clean", setup: envCreate("feat/a", "--from", "main", "--yes"), args: []string{"feat/a", "--" + domain.FlagCheck}, branch: "feat/a"},
		{name: "check-json", setup: driftSetup, args: append([]string{"feat/a", "--" + domain.FlagCheck}, json...), branch: "feat/a"},
		{name: "prune", setup: driftSetup, args: []string{"feat/a", yes, "--" + domain.FlagPrune}, branch: "feat/a"},
		{name: "refresh-overwrite", setup: driftSetup, args: []string{"feat/a", yes, "--" + domain.FlagMode, "refresh", "--" + domain.FlagOnConflict, "overwrite"}, branch: "feat/a"},
		{name: "refresh-keep-json", setup: driftSetup, args: append([]string{"feat/a", yes, "--" + domain.FlagMode, "refresh"}, json...), branch: "feat/a"},
		{name: "from-parent", setup: func(t *testing.T, dir string) {
			envCreate("feat/a", "--from", "main", "--yes")(t, dir)
			writeEnvFile(t, worktreeEnvPath(dir, "feat/a"), readEnvOf(t, dir, "feat/a")+"FROM_A=1\n")
			envCreate("feat/b", "--from", "feat/a", "--yes")(t, dir)
			writeEnvFile(t, filepath.Join(dir, ".env"), mainEnv+"FROM_MAIN=1\n")
		}, args: []string{"feat/b", yes, "--" + domain.FlagFrom, "parent"}, branch: "feat/b"},
		{name: "warning-invalid-run-toml", setup: func(t *testing.T, dir string) {
			driftSetup(t, dir)
			writeEnvFile(t, filepath.Join(dir, ".git", "wtm", domain.RunFileName), "bogus_key = 1\n")
		}, args: []string{"feat/a", yes}, branch: "feat/a"},
		{name: "main", setup: func(t *testing.T, dir string) {
			writeEnvFile(t, filepath.Join(dir, ".env"), "WEB_PORT=9999\nREALM=other\n")
		}, args: []string{"main", yes}, branch: "main"},
		{name: "isolation-changed", setup: envCreate("feat/v", "--from", "main", "--yes", "--"+domain.FlagIsolation, "verbatim"), args: append([]string{"feat/v", yes, isolation, "isolated"}, json...), branch: "feat/v"},
		{name: "isolation-changed-human", setup: envCreate("feat/v", "--from", "main", "--yes", "--"+domain.FlagIsolation, "verbatim"), args: []string{"feat/v", yes, isolation, "isolated"}, branch: "feat/v"},
		{name: "isolation-unchanged", setup: envCreate("feat/i", "--from", "main", "--yes"), args: append([]string{"feat/i", yes, isolation, "isolated"}, json...), branch: "feat/i"},
		{name: "isolation-verbatim", setup: envCreate("feat/i", "--from", "main", "--yes"), args: []string{"feat/i", yes, isolation, "verbatim"}, branch: "feat/i"},
		{name: "err-json-without-yes", setup: envCreate("feat/a", "--from", "main", "--yes"), args: append([]string{"feat/a"}, json...)},
		{name: "err-worktree-required", args: []string{yes}},
		{name: "err-worktree-required-check", args: []string{"--" + domain.FlagCheck}},
		{name: "err-not-found", args: []string{"nope", yes}},
		{name: "err-no-files", setup: noFilesSetup, args: []string{"main", yes}},
		{name: "err-bad-mode", args: []string{"main", yes, "--" + domain.FlagMode, "bogus"}},
		{name: "err-bad-from", args: []string{"main", yes, "--" + domain.FlagFrom, "bogus"}},
		{name: "err-bad-on-conflict", args: []string{"main", yes, "--" + domain.FlagOnConflict, "bogus"}},
		{name: "err-bad-isolation", args: []string{"main", yes, isolation, "bogus"}},
		{name: "err-isolation-with-check", args: []string{"main", "--" + domain.FlagCheck, isolation, "verbatim"}},
		{name: "err-prune-with-check", args: []string{"main", "--" + domain.FlagCheck, "--" + domain.FlagPrune}},
		{name: "err-on-conflict-in-add", args: []string{"main", yes, "--" + domain.FlagOnConflict, "overwrite"}},
	}
}

package wt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// extractTestRepo sets up a git repo + minimal wtm config and returns the main
// worktree dir and state dir.
func extractTestRepo(t *testing.T) (dir, stateDir string) {
	t.Helper()
	dir = gittest.InitRepo(t)
	stateDir = filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	return dir, stateDir
}

func createWorktree(t *testing.T, branch string) domain.CreateResult {
	t.Helper()
	out, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("create %s: %v", branch, err)
	}
	var res domain.CreateResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decode create result: %v\n%s", err, out)
	}
	return res
}

// TestExtractRequiresSourceNonInteractive verifies that, aligned with sync, a
// non-interactive extract without a source argument errors instead of defaulting
// to the current worktree.
func TestExtractRequiresSourceNonInteractive(t *testing.T) {
	_, _ = extractTestRepo(t)

	_, _, err := runWtCmd(t, domain.CmdExtract, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if !errors.Is(err, domain.ErrExtractSourceRequired) {
		t.Fatalf("expected ErrExtractSourceRequired, got %v", err)
	}
}

// TestExtractUnknownSource verifies a source argument that is not a worktree
// branch is rejected.
func TestExtractUnknownSource(t *testing.T) {
	_, _ = extractTestRepo(t)

	_, _, err := runWtCmd(t, domain.CmdExtract, "does-not-exist", "--files", "x", "--to", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("expected an error for an unknown source worktree")
	}
}

// TestExtractYesRequiresSelectionFlags verifies the hardened bypass taxonomy: under
// --yes a missing required selection errors naming the flag (no picker fallback),
// while --yes with every selection supplied runs unattended.
func TestExtractYesRequiresSelectionFlags(t *testing.T) {
	_, _ = extractTestRepo(t)
	src := createWorktree(t, "src")
	createWorktree(t, "dst")

	if err := os.WriteFile(filepath.Join(src.Path, "f.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	// --yes without --files errors (files are a required selection with no default).
	if _, _, err := runWtCmd(t, domain.CmdExtract, "src", "--to", "dst", "--yes"); !errors.Is(err, domain.ErrExtractFilesRequired) {
		t.Fatalf("expected ErrExtractFilesRequired, got %v", err)
	}
	// --yes without --to errors (target is a required selection with no default).
	if _, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "f.txt", "--yes"); !errors.Is(err, domain.ErrExtractTargetRequired) {
		t.Fatalf("expected ErrExtractTargetRequired, got %v", err)
	}
	// --yes with source + --files + --to runs unattended (text output, no picker).
	if _, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "f.txt", "--to", "dst", "--yes"); err != nil {
		t.Fatalf("extract --yes with full flags: %v", err)
	}
}

// TestExtractWithSourceArgJSON drives the full non-interactive extraction: a
// source worktree is named by argument, its uncommitted file is moved to the
// target worktree, and the JSON result reports the source branch.
func TestExtractWithSourceArgJSON(t *testing.T) {
	_, _ = extractTestRepo(t)

	src := createWorktree(t, "src")
	dst := createWorktree(t, "dst")

	srcFile := filepath.Join(src.Path, "extracted.txt")
	if err := os.WriteFile(srcFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	out, _, err := runWtCmd(t, domain.CmdExtract, "src",
		"--files", "extracted.txt", "--to", "dst", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	var res domain.ExtractResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decode extract result: %v\n%s", err, out)
	}
	if res.SourceBranch != "src" {
		t.Errorf("source_branch = %q, want \"src\"", res.SourceBranch)
	}
	if res.TargetBranch != "dst" {
		t.Errorf("target_branch = %q, want \"dst\"", res.TargetBranch)
	}

	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Errorf("expected the file to be moved out of the source, stat: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst.Path, "extracted.txt")); err != nil {
		t.Errorf("expected the file in the target worktree: %v", err)
	}
}

// TestExtractPicksOneFileOutOfANewDirectory covers the enumeration fix: files of a
// brand-new directory are listed one by one, so --files can name a single one
// instead of taking the whole tree.
func TestExtractPicksOneFileOutOfANewDirectory(t *testing.T) {
	_, _ = extractTestRepo(t)
	src := createWorktree(t, "src")
	dst := createWorktree(t, "dst")

	writeWorktreeFile(t, src.Path, "newmod/x.go", "package newmod\n")
	writeWorktreeFile(t, src.Path, "newmod/sub/y.go", "package sub\n")

	out, _, err := runWtCmd(t, domain.CmdExtract, "src",
		"--files", "newmod/x.go", "--to", "dst", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	var res domain.ExtractResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decode extract result: %v\n%s", err, out)
	}
	if len(res.Files) != 1 || res.Files[0].Path != "newmod/x.go" {
		t.Fatalf("files = %+v, want just newmod/x.go", res.Files)
	}
	if _, err := os.Stat(filepath.Join(dst.Path, "newmod/x.go")); err != nil {
		t.Errorf("expected newmod/x.go in the target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(src.Path, "newmod/sub/y.go")); err != nil {
		t.Errorf("the sibling must stay in the source: %v", err)
	}
}

// TestExtractFilesAcceptsDirectoryPrefix keeps the pre-uall spelling working:
// naming a directory takes every change below it.
func TestExtractFilesAcceptsDirectoryPrefix(t *testing.T) {
	_, _ = extractTestRepo(t)
	src := createWorktree(t, "src")
	dst := createWorktree(t, "dst")

	writeWorktreeFile(t, src.Path, "newmod/x.go", "package newmod\n")
	writeWorktreeFile(t, src.Path, "newmod/sub/y.go", "package sub\n")

	out, _, err := runWtCmd(t, domain.CmdExtract, "src",
		"--files", "newmod/", "--to", "dst", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	var res domain.ExtractResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decode extract result: %v\n%s", err, out)
	}
	if len(res.Files) != 2 {
		t.Fatalf("files = %+v, want both files under newmod/", res.Files)
	}
	for _, rel := range []string{"newmod/x.go", "newmod/sub/y.go"} {
		if _, err := os.Stat(filepath.Join(dst.Path, rel)); err != nil {
			t.Errorf("expected %s in the target: %v", rel, err)
		}
	}
}

// TestExtractQuotedPathViaFilesFlag covers the path-quoting fix: a name git would
// quote under the default porcelain format is selectable verbatim.
func TestExtractQuotedPathViaFilesFlag(t *testing.T) {
	_, _ = extractTestRepo(t)
	src := createWorktree(t, "src")
	dst := createWorktree(t, "dst")

	writeWorktreeFile(t, src.Path, "a b.txt", "spaced\n")

	if _, _, err := runWtCmd(t, domain.CmdExtract, "src",
		"--files", "a b.txt", "--to", "dst", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst.Path, "a b.txt")); err != nil {
		t.Errorf("expected \"a b.txt\" in the target: %v", err)
	}
}

// TestExtractUntrackedCollisionFollowsOnConflict verifies an untracked file that
// already exists in the target sits on the --on-conflict axis: abort by default
// under --yes, conflict markers with --on-conflict resolve.
func TestExtractUntrackedCollisionFollowsOnConflict(t *testing.T) {
	_, _ = extractTestRepo(t)
	src := createWorktree(t, "src")
	dst := createWorktree(t, "dst")

	writeWorktreeFile(t, src.Path, "c.txt", "from source\n")
	writeWorktreeFile(t, dst.Path, "c.txt", "already here\n")

	_, _, err := runWtCmd(t, domain.CmdExtract, "src",
		"--files", "c.txt", "--to", "dst", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if !errors.Is(err, domain.ErrExtractConflict) {
		t.Fatalf("expected ErrExtractConflict under the safe default, got %v", err)
	}

	out, _, err := runWtCmd(t, domain.CmdExtract, "src",
		"--files", "c.txt", "--to", "dst", "--"+domain.FlagOnConflict, domain.OnConflictResolve,
		"--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("extract --on-conflict resolve: %v", err)
	}

	var res domain.ExtractResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decode extract result: %v\n%s", err, out)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0] != "c.txt" {
		t.Fatalf("conflicts = %v, want [c.txt]", res.Conflicts)
	}

	merged, err := os.ReadFile(filepath.Join(dst.Path, "c.txt"))
	if err != nil {
		t.Fatalf("read merged file: %v", err)
	}
	if !strings.Contains(string(merged), "<<<<<<<") {
		t.Errorf("target c.txt missing conflict markers:\n%s", merged)
	}
}

func writeWorktreeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// TestExtractSettlesEnvPortsOnTheCreatedTarget guards the isolation of a
// worktree extract creates: its .env has to carry the ports this worktree
// binds, not the ones it was copied from. create has settled them since
// LUC-99; extract never did.
func TestExtractSettlesEnvPortsOnTheCreatedTarget(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := config.WriteProject(config.WriteProjectParams{
		StateDir: stateDir,
		Answers: domain.InitProjectAnswers{
			BasePath:    "../.trees",
			BaseBranch:  "main",
			EnvStrategy: domain.EnvStrategyMain,
			EnvFiles:    []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
		},
	}); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	if err := config.WriteRun(config.WriteRunParams{
		StateDir: stateDir,
		Force:    true,
		Config: domain.RunConfig{
			Jobs: []domain.JobConfig{{
				Name: "web", Kind: domain.JobKindService, Cmd: "true",
				Ports: map[string]int{"PORT": 3000},
			}},
			EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"}},
		},
	}); err != nil {
		t.Fatalf("write run config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte("WEB_PORT=3000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".env.example")
	gittest.Git(t, dir, "commit", "-m", "env template")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("WEB_PORT=3000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "moved.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runWtCmd(t, domain.CmdExtract, "main",
		"--"+domain.FlagTo, "feat/target",
		"--"+domain.FlagFrom, "main",
		"--"+domain.FlagFiles, "moved.txt",
		"--"+domain.FlagYes,
		"--output", domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	var res domain.ExtractResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decode extract result: %v\n%s", err, out)
	}

	body, err := os.ReadFile(filepath.Join(res.TargetPath, ".env"))
	if err != nil {
		t.Fatalf("read target .env: %v", err)
	}
	if strings.Contains(string(body), "WEB_PORT=3000") {
		t.Errorf("target .env kept the base port, isolation is broken: %s", body)
	}
}

// G1: extract creating its target goes ahead over a run.toml the port pass
// refuses — hooks run, the .env stays as copied, and the JSON says why.
func TestExtractCreatesItsTargetOverAnInvalidRunToml(t *testing.T) {
	dir, stateDir := extractTestRepo(t)
	if err := os.WriteFile(filepath.Join(stateDir, domain.RunFileName), []byte("bogus_key = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "moved.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runWtCmd(t, domain.CmdExtract, "main",
		"--"+domain.FlagTo, "feat/target",
		"--"+domain.FlagFrom, "main",
		"--"+domain.FlagFiles, "moved.txt",
		"--"+domain.FlagYes,
		"--output", domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("extract must not fail over run.toml: %v", err)
	}

	var res domain.ExtractResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decode extract result: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(res.TargetPath, "moved.txt")); err != nil {
		t.Errorf("the file was not extracted: %v", err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "bogus_key") {
		t.Errorf("warnings = %v, want the refused run.toml named", res.Warnings)
	}
}

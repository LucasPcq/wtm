package wt

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// These tests pin `wtm extract` from the outside — flags in, streams, errors and
// the state of both worktrees out — before its move to internal/flow (LUC-240).
// They are not edited by the migration: a diff here is a behaviour change.

type extractRepo struct {
	dir      string
	stateDir string
}

func newExtractRepo(t *testing.T) extractRepo {
	t.Helper()
	dir, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	return extractRepo{dir: dir, stateDir: stateDir}
}

func (r extractRepo) writeConfig(t *testing.T, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.stateDir, domain.ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func (r extractRepo) normalize(s string) string {
	trees := filepath.Join(filepath.Dir(r.dir), ".trees")
	return strings.ReplaceAll(s, trees, "<trees>")
}

// extractFixture is a source worktree carrying a.txt (modified), b.txt
// (untracked) and gone.txt (deleted), next to an empty target worktree.
type extractFixture struct {
	repo extractRepo
	src  domain.CreateResult
	dst  domain.CreateResult
}

func newExtractFixture(t *testing.T) extractFixture {
	t.Helper()
	repo := newExtractRepo(t)
	writeWorktreeFile(t, repo.dir, "a.txt", "one\n")
	writeWorktreeFile(t, repo.dir, "gone.txt", "bye\n")
	gittest.Git(t, repo.dir, "add", "a.txt", "gone.txt")
	gittest.Git(t, repo.dir, "commit", "-m", "tracked files")

	src := createWorktree(t, "src")
	dst := createWorktree(t, "dst")
	writeWorktreeFile(t, src.Path, "a.txt", "one\ntwo\n")
	writeWorktreeFile(t, src.Path, "b.txt", "new\n")
	if err := os.Remove(filepath.Join(src.Path, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	return extractFixture{repo: repo, src: src, dst: dst}
}

// printedBeyondUsage ignores the usage cobra prints on an error under this
// harness, which the real root silences.
func printedBeyondUsage(stdout string) bool {
	return stdout != "" && !strings.HasPrefix(stdout, "Usage:")
}

func fileContent(t *testing.T, path string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data), true
}

func TestCharacterizeExtractRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"json without --yes", []string{"src", "--files", "a.txt", "--to", "dst", "--output", "json"}, "--output json requires --yes (prompts cannot run in JSON mode)"},
		{"no source under --yes", []string{"--files", "a.txt", "--to", "dst", "--yes"}, domain.ErrExtractSourceRequired.Error()},
		{"no source without a terminal", []string{"--files", "a.txt", "--to", "dst"}, domain.ErrExtractSourceRequired.Error()},
		{"no --files under --yes", []string{"src", "--to", "dst", "--yes"}, domain.ErrExtractFilesRequired.Error()},
		{"no --files without a terminal", []string{"src", "--to", "dst"}, domain.ErrExtractFilesRequired.Error()},
		{"no --to under --yes", []string{"src", "--files", "a.txt", "--yes"}, domain.ErrExtractTargetRequired.Error()},
		{"an invalid --on-conflict", []string{"src", "--files", "a.txt", "--to", "dst", "--on-conflict", "merge", "--yes"}, `invalid --on-conflict value "merge": use abort or resolve`},
		{"an invalid --isolation", []string{"src", "--files", "a.txt", "--to", "dst", "--isolation", "loose", "--yes"}, ""},
		{"an unknown source", []string{"nope", "--files", "a.txt", "--to", "dst", "--yes"}, `source worktree "nope": `},
		{"a file that is not a change", []string{"src", "--files", "zzz.txt", "--to", "dst", "--yes"}, ""},
		{"a source as its own target", []string{"src", "--files", "a.txt", "--to", "src", "--yes"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newExtractFixture(t)
			stdout, _, err := runWtCmd(t, append([]string{domain.CmdExtract}, c.args...)...)
			if err == nil {
				t.Fatalf("extract %v succeeded, want a refusal", c.args)
			}
			if c.want != "" && !strings.HasPrefix(err.Error(), c.want) {
				t.Errorf("error = %q, want it to start with %q", err.Error(), c.want)
			}
			if printedBeyondUsage(stdout) {
				t.Errorf("stdout = %q, want nothing on a refusal", stdout)
			}
			if got, _ := fileContent(t, filepath.Join(fx.src.Path, "a.txt")); got != "one\ntwo\n" {
				t.Errorf("the source changed on a refusal: a.txt = %q", got)
			}
		})
	}
}

func TestCharacterizeExtractRefusalMessages(t *testing.T) {
	newExtractFixture(t)

	_, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "zzz.txt", "--to", "dst", "--yes")
	if err == nil || !strings.Contains(err.Error(), "zzz.txt") {
		t.Errorf("a file that is not a change must be named, got %v", err)
	}

	_, _, err = runWtCmd(t, domain.CmdExtract, "src", "--files", "a.txt", "--to", "src", "--yes")
	if !errors.Is(err, domain.ErrSameWorktree) {
		t.Errorf("a source as its own target = %v, want ErrSameWorktree", err)
	}

	_, _, err = runWtCmd(t, domain.CmdExtract, "src", "--files", "a.txt", "--to", "dst", "--isolation", "loose", "--yes")
	if want := `unknown isolation "loose" (expected "isolated" or "verbatim")`; err == nil || err.Error() != want {
		t.Errorf("an invalid --isolation = %v, want %q", err, want)
	}
}

func TestCharacterizeExtractMoveToAnExistingWorktree(t *testing.T) {
	fx := newExtractFixture(t)

	stdout, stderr, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "a.txt,b.txt,gone.txt", "--to", "dst", "--yes")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want := "\n" +
		"  ✓ Moved 3 files to dst\n" +
		"\n" +
		"      mod  a.txt\n" +
		"      new  b.txt\n" +
		"      del  gone.txt\n" +
		"\n" +
		"  source  src · clean\n" +
		"  worktree  <trees>/dst\n" +
		"\n" +
		"  → wtm go dst\n" +
		"\n"
	if got := fx.repo.normalize(stdout); got != want {
		t.Errorf("stdout:\n%q\nwant:\n%q", got, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}

	if got, _ := fileContent(t, filepath.Join(fx.dst.Path, "a.txt")); got != "one\ntwo\n" {
		t.Errorf("target a.txt = %q", got)
	}
	if got, _ := fileContent(t, filepath.Join(fx.dst.Path, "b.txt")); got != "new\n" {
		t.Errorf("target b.txt = %q", got)
	}
	if _, present := fileContent(t, filepath.Join(fx.dst.Path, "gone.txt")); present {
		t.Error("the deletion was not carried to the target")
	}
	if got, _ := fileContent(t, filepath.Join(fx.src.Path, "a.txt")); got != "one\n" {
		t.Errorf("a move must restore the source: a.txt = %q", got)
	}
	if _, present := fileContent(t, filepath.Join(fx.src.Path, "b.txt")); present {
		t.Error("a move must remove the untracked file from the source")
	}
	if _, present := fileContent(t, filepath.Join(fx.src.Path, "gone.txt")); !present {
		t.Error("a move must restore the deleted file in the source")
	}
}

func TestCharacterizeExtractCopyKeepsTheSource(t *testing.T) {
	fx := newExtractFixture(t)

	stdout, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "a.txt", "--to", "dst", "--keep", "--yes")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want := "\n" +
		"  ✓ Copied 1 file to dst\n" +
		"\n" +
		"      mod  a.txt\n" +
		"\n" +
		"  source  src · kept\n" +
		"  worktree  <trees>/dst\n" +
		"\n" +
		"  → wtm go dst\n" +
		"\n"
	if got := fx.repo.normalize(stdout); got != want {
		t.Errorf("stdout:\n%q\nwant:\n%q", got, want)
	}
	if got, _ := fileContent(t, filepath.Join(fx.src.Path, "a.txt")); got != "one\ntwo\n" {
		t.Errorf("a copy must keep the source: a.txt = %q", got)
	}
	if got, _ := fileContent(t, filepath.Join(fx.dst.Path, "a.txt")); got != "one\ntwo\n" {
		t.Errorf("target a.txt = %q", got)
	}
}

func TestCharacterizeExtractJSON(t *testing.T) {
	fx := newExtractFixture(t)

	stdout, stderr, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "a.txt,b.txt", "--to", "dst", "--output", "json", "--yes")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	want := map[string]any{
		"source_branch": "src",
		"target_branch": "dst",
		"target_path":   fx.dst.Path,
		"kept":          false,
		"isolation":     "isolated",
		"files": []any{
			map[string]any{"path": "a.txt", "status": "modified"},
			map[string]any{"path": "b.txt", "status": "untracked"},
		},
	}
	assertJSONSubset(t, got, want)
}

func assertJSONSubset(t *testing.T, got, want map[string]any) {
	t.Helper()
	for key, value := range want {
		gotJSON, _ := json.Marshal(got[key])
		wantJSON, _ := json.Marshal(value)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s = %s, want %s", key, gotJSON, wantJSON)
		}
	}
}

func TestCharacterizeExtractKeysOfTheJSONDocument(t *testing.T) {
	newExtractFixture(t)

	stdout, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "a.txt", "--to", "dst", "--output", "json", "--yes")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(got))
	for key := range got {
		keys = append(keys, key)
	}
	joined := strings.Join(sortedStrings(keys), ",")
	if joined != "conflicts,files,isolation,kept,source_branch,target_branch,target_path" {
		t.Errorf("keys = %s", joined)
	}
}

func sortedStrings(values []string) []string {
	sorted := append([]string(nil), values...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}

func TestCharacterizeExtractAConflictAbortsUnderYes(t *testing.T) {
	fx := newExtractFixture(t)
	writeWorktreeFile(t, fx.dst.Path, "b.txt", "already here\n")

	stdout, stderr, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "b.txt", "--to", "dst", "--yes")
	if !errors.Is(err, domain.ErrExtractConflict) {
		t.Fatalf("error = %v, want ErrExtractConflict", err)
	}
	if printedBeyondUsage(stdout) || stderr != "Error: "+err.Error()+"\n" {
		t.Errorf("an aborted conflict prints nothing itself: stdout %q, stderr %q", stdout, stderr)
	}
	if got, _ := fileContent(t, filepath.Join(fx.dst.Path, "b.txt")); got != "already here\n" {
		t.Errorf("the target changed on an abort: %q", got)
	}
	if got, _ := fileContent(t, filepath.Join(fx.src.Path, "b.txt")); got != "new\n" {
		t.Errorf("the source changed on an abort: %q", got)
	}
}

func TestCharacterizeExtractAConflictWithoutATerminalAborts(t *testing.T) {
	fx := newExtractFixture(t)
	writeWorktreeFile(t, fx.dst.Path, "b.txt", "already here\n")

	_, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "b.txt", "--to", "dst")
	if !errors.Is(err, domain.ErrExtractConflict) {
		t.Fatalf("error = %v, want ErrExtractConflict", err)
	}
}

func TestCharacterizeExtractResolveWritesMarkers(t *testing.T) {
	fx := newExtractFixture(t)
	writeWorktreeFile(t, fx.dst.Path, "b.txt", "already here\n")

	stdout, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "a.txt,b.txt", "--to", "dst", "--on-conflict", "resolve", "--yes")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want := "\n" +
		"  ! Applied to dst with conflicts\n" +
		"\n" +
		"  Conflicts to resolve in dst\n" +
		"    b.txt\n" +
		"\n" +
		"  The other files were applied cleanly.\n" +
		"\n" +
		"  Nothing was removed from src — your changes are safe there.\n" +
		"  • Finish the split: resolve the conflicts in dst, then discard the same files in src.\n" +
		"  • Undo: discard the applied changes in dst — src stays untouched.\n" +
		"\n" +
		"  worktree  <trees>/dst\n" +
		"\n"
	if got := fx.repo.normalize(stdout); got != want {
		t.Errorf("stdout:\n%q\nwant:\n%q", got, want)
	}
	merged, _ := fileContent(t, filepath.Join(fx.dst.Path, "b.txt"))
	if !strings.Contains(merged, "<<<<<<<") {
		t.Errorf("target b.txt carries no markers:\n%s", merged)
	}
	if got, _ := fileContent(t, filepath.Join(fx.src.Path, "a.txt")); got != "one\ntwo\n" {
		t.Errorf("a conflicted run removes nothing from the source: a.txt = %q", got)
	}
}

func TestCharacterizeExtractASourceWithoutChanges(t *testing.T) {
	newExtractRepo(t)
	createWorktree(t, "src")
	createWorktree(t, "dst")

	stdout, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "a.txt", "--to", "dst", "--yes")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if want := "\n  no uncommitted changes to extract\n\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	stdout, _, err = runWtCmd(t, domain.CmdExtract, "src", "--files", "a.txt", "--to", "dst", "--output", "json", "--yes")
	if err != nil {
		t.Fatalf("extract json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if files, ok := got["files"].([]any); !ok || len(files) != 0 {
		t.Errorf("files = %v, want an empty list", got["files"])
	}
}

func TestCharacterizeExtractCreatesItsTarget(t *testing.T) {
	fx := newExtractFixture(t)

	stdout, stderr, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "b.txt", "--to", "feat/new", "--yes")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want := "\n" +
		"  ✓ Moved 1 file to feat/new\n" +
		"\n" +
		"      new  b.txt\n" +
		"\n" +
		"  source  src · clean\n" +
		"  worktree  <trees>/feat-new\n" +
		"\n" +
		"  → wtm go feat/new\n" +
		"\n"
	if got := fx.repo.normalize(stdout); got != want {
		t.Errorf("stdout:\n%q\nwant:\n%q", got, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
	if got, _ := fileContent(t, filepath.Join(fx.repo.dir, "..", ".trees", "feat-new", "b.txt")); got != "new\n" {
		t.Errorf("the created target does not carry b.txt: %q", got)
	}
}

// The source's recorded parent is the default parent of a target extract
// creates, ahead of the configured base branch.
func TestCharacterizeExtractRecordsTheSourcesParentOnItsTarget(t *testing.T) {
	repo := newExtractRepo(t)
	createWorktree(t, "base")
	out, _, err := runWtCmd(t, domain.CmdCreate, "stacked", "--from", "base", "--output", "json", "--yes")
	if err != nil {
		t.Fatalf("create stacked: %v", err)
	}
	stacked := decodeCreated(t, out)
	writeWorktreeFile(t, stacked.Path, "x.txt", "x\n")

	stdout, _, err := runWtCmd(t, domain.CmdExtract, "stacked", "--files", "x.txt", "--to", "split", "--output", "json", "--yes")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	var res domain.ExtractResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatal(err)
	}
	meta, err := os.ReadFile(filepath.Join(repo.stateDir, "worktrees", "split", "meta.json"))
	if err != nil {
		t.Fatalf("read the target's meta: %v", err)
	}
	if !strings.Contains(string(meta), `"base"`) {
		t.Errorf("the target's parent should be the source's (base), meta: %s", meta)
	}
}

func TestCharacterizeExtractWithFromStacksTheTarget(t *testing.T) {
	repo := newExtractRepo(t)
	src := createWorktree(t, "src")
	createWorktree(t, "other")
	writeWorktreeFile(t, src.Path, "x.txt", "x\n")

	if _, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "x.txt", "--to", "split", "--from", "other", "--output", "json", "--yes"); err != nil {
		t.Fatalf("extract: %v", err)
	}
	meta, err := os.ReadFile(filepath.Join(repo.stateDir, "worktrees", "split", "meta.json"))
	if err != nil {
		t.Fatalf("read the target's meta: %v", err)
	}
	if !strings.Contains(string(meta), `"other"`) {
		t.Errorf("--from must be the target's parent, meta: %s", meta)
	}
}

func TestCharacterizeExtractOntoAnExistingBranch(t *testing.T) {
	repo := newExtractRepo(t)
	src := createWorktree(t, "src")
	writeWorktreeFile(t, src.Path, "x.txt", "x\n")
	gitCommitOn(t, repo.dir, "feat/old", "kept commit")

	_, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "x.txt", "--to", "feat/old", "--yes")
	if err == nil {
		t.Fatal("an existing branch without --from must be refused")
	}
	if want := "feat/old already exists locally: pass --from to record its parent branch (it can't be inferred, and `wtm sync` needs it)"; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}

	stdout, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "x.txt", "--to", "feat/old", "--from", "main", "--output", "json", "--yes")
	if err != nil {
		t.Fatalf("extract with --from: %v", err)
	}
	var res domain.ExtractResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatal(err)
	}
	log := gitOutput(t, res.TargetPath, "log", "--format=%s", "-1")
	if strings.TrimSpace(log) != "kept commit" {
		t.Errorf("the reused branch lost its commits, head = %q", log)
	}
}

func TestCharacterizeExtractFastForwardsTheParentWithFF(t *testing.T) {
	work := repoWithRemote(t)
	dir, err := filepath.EvalSymlinks(work)
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatal(err)
	}
	behindBranch(t, dir, "parent")
	src := createWorktree(t, "src")
	writeWorktreeFile(t, src.Path, "x.txt", "x\n")

	before := revParse(t, dir, "parent")
	if _, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "x.txt", "--to", "split", "--from", "parent", "--output", "json", "--yes"); err != nil {
		t.Fatalf("extract without --ff: %v", err)
	}
	if got := revParse(t, dir, "parent"); got != before {
		t.Error("without --ff the parent must be left alone under --yes")
	}

	writeWorktreeFile(t, src.Path, "y.txt", "y\n")
	if _, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "y.txt", "--to", "split2", "--from", "parent", "--ff", "--output", "json", "--yes"); err != nil {
		t.Fatalf("extract --ff: %v", err)
	}
	if got := revParse(t, dir, "parent"); got != revParse(t, dir, "origin/parent") {
		t.Error("--ff must fast-forward the parent to origin")
	}
}

func TestCharacterizeExtractWarnsAboutAnIgnoredIsolation(t *testing.T) {
	newExtractFixture(t)

	stdout, stderr, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "a.txt", "--to", "dst", "--isolation", "verbatim", "--output", "json", "--yes")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	var res domain.ExtractResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "dst") {
		t.Errorf("warnings = %v, want the ignored --isolation named", res.Warnings)
	}
	if !strings.Contains(stderr, res.Warnings[0]) {
		t.Errorf("stderr = %q, want the warning said as it happens", stderr)
	}

	_, stderr, err = runWtCmd(t, domain.CmdExtract, "src", "--files", "b.txt", "--to", "dst", "--isolation", "verbatim", "--yes")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if want := "\n  ! --isolation verbatim ignored: dst already exists and stays isolated — switch it with `wtm env dst --isolation verbatim`\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestCharacterizeExtractRunsTheCreateHooksOnANewTarget(t *testing.T) {
	fx := newExtractFixture(t)
	fx.repo.writeConfig(t, `[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[hooks]
on_create = ["echo hooked > hooked.txt"]
`)

	stdout, stderr, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "b.txt", "--to", "feat/hooked", "--yes")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	target := filepath.Join(fx.repo.dir, "..", ".trees", "feat-hooked")
	if got, _ := fileContent(t, filepath.Join(target, "hooked.txt")); got != "hooked\n" {
		t.Errorf("the on_create hook did not run in the target: %q", got)
	}
	if !strings.Contains(stdout, "✓ Moved 1 file to feat/hooked") {
		t.Errorf("stdout = %q", stdout)
	}
	for _, want := range []string{"Hooks · On Create", "✓ echo hooked > hooked.txt ("} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want %q: the hook phase belongs to stderr", stderr, want)
		}
	}
	if strings.Contains(stdout, "echo hooked") {
		t.Errorf("stdout = %q: the hook phase leaked onto the result", stdout)
	}

	if _, err := os.Stat(filepath.Join(fx.repo.stateDir, "hooks", "on_create-feat%2Fhooked.log")); err != nil {
		t.Errorf("the hook phase left no log: %v", err)
	}
}

// Hooks never run on an existing target: extract did not create it.
func TestCharacterizeExtractRunsNoHookOnAnExistingTarget(t *testing.T) {
	fx := newExtractFixture(t)
	fx.repo.writeConfig(t, `[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[hooks]
on_create = ["echo hooked > hooked.txt"]
`)
	if _, _, err := runWtCmd(t, domain.CmdExtract, "src", "--files", "b.txt", "--to", "dst", "--yes"); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if _, present := fileContent(t, filepath.Join(fx.dst.Path, "hooked.txt")); present {
		t.Error("a hook ran on a target extract did not create")
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

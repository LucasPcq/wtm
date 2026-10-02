package checkout

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/ghtest"
)

// These tests pin `wtm checkout` from the outside — flags in, streams and git
// state out — so they hold whatever drives the command underneath (LUC-237).

const (
	prFresh  = 42
	prFork   = 9
	prReused = 51
)

type checkoutRepo struct {
	work     string
	stateDir string
}

func newCheckoutRepo(t *testing.T) checkoutRepo {
	t.Helper()
	work := repoWithRemote(t)
	result := loadResult(t, work)
	ghtest.Stub(t, ghtest.StubParams{Details: []ghtest.PRDetail{
		{Number: prFresh, Title: "Add the thing", Author: "octocat", Branch: "feat/thing", Base: "main"},
		{Number: prFork, Title: "From a fork", Author: "stranger", Branch: "patch-1", Base: "main", Fork: true},
		{Number: prReused, Title: "Reused", Author: "octocat", Branch: "feat/reused", Base: "main"},
	}})
	return checkoutRepo{work: work, stateDir: result.StateDir}
}

func (r checkoutRepo) pushBranch(t *testing.T, branch string) {
	t.Helper()
	git(t, r.work, "branch", branch)
	git(t, r.work, "push", "origin", branch)
	git(t, r.work, "branch", "-D", branch)
}

func (r checkoutRepo) writeConfig(t *testing.T, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.stateDir, domain.ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func (r checkoutRepo) normalize(s string) string {
	trees := filepath.Join(filepath.Dir(r.work), ".trees")
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(r.work)); err == nil {
		s = strings.ReplaceAll(s, filepath.Join(resolved, ".trees"), "<trees>")
	}
	s = strings.ReplaceAll(s, trees, "<trees>")
	return strings.ReplaceAll(s, r.work, "<work>")
}

func runCheckoutCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := NewCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func decodeCheckoutJSON(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode checkout JSON %q: %v", stdout, err)
	}
	return got
}

func TestCharacterizeRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"a number that is not one", []string{"abc", "--yes"}, `invalid PR number "abc"`},
		{"a number that is not positive", []string{"0", "--yes"}, `invalid PR number "0"`},
		{"json without --yes", []string{"42", "--output", "json"}, "--output json requires --yes (prompts cannot run in JSON mode)"},
		{"no number under --yes", []string{"--yes"}, "PR number required without an interactive terminal (or when --yes is set)"},
		{"no number without a terminal", nil, "PR number required without an interactive terminal (or when --yes is set)"},
		{"a fork", []string{"9", "--yes"}, "PR #9 is from a fork — wtm doesn't check out fork PRs by design; use `gh pr checkout 9`"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			newCheckoutRepo(t)
			stdout, _, err := runCheckoutCmd(t, c.args...)
			if err == nil {
				t.Fatalf("checkout %v succeeded, want %q", c.args, c.want)
			}
			if err.Error() != c.want {
				t.Errorf("error = %q, want %q", err.Error(), c.want)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing on a refusal", stdout)
			}
		})
	}
}

func TestCharacterizeAnUnknownPRIsAFetchError(t *testing.T) {
	newCheckoutRepo(t)
	_, _, err := runCheckoutCmd(t, "77", "--yes")
	if err == nil || !strings.HasPrefix(err.Error(), "fetch PR: get PR: ") {
		t.Fatalf("error = %v, want it prefixed by the fetch", err)
	}
}

func TestCharacterizeHumanCheckout(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")

	stdout, stderr, err := runCheckoutCmd(t, "42", "--yes")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	want := "\n" +
		"  ✓ Checked out PR #42 (feat/thing)\n" +
		"\n" +
		"  path  <trees>/feat-thing\n" +
		"\n" +
		"  → wtm go feat/thing\n" +
		"\n"
	if got := repo.normalize(stdout); got != want {
		t.Errorf("stdout =\n%q\nwant\n%q", got, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing without hooks", stderr)
	}
	if parent := worktree.ParentBranch(worktree.ParentBranchParams{StateDir: repo.stateDir, Branch: "feat/thing"}); parent != "main" {
		t.Errorf("recorded parent = %q, want the PR base", parent)
	}
}

func TestCharacterizeFromRecordsTheParent(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")
	git(t, repo.work, "branch", "develop")

	if _, _, err := runCheckoutCmd(t, "42", "--yes", "--from", "develop", "--output", "json"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if parent := worktree.ParentBranch(worktree.ParentBranchParams{StateDir: repo.stateDir, Branch: "feat/thing"}); parent != "develop" {
		t.Errorf("recorded parent = %q, want --from", parent)
	}
}

func TestCharacterizeEnvFromIsRecorded(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")

	if _, _, err := runCheckoutCmd(t, "42", "--yes", "--env-from", "main", "--output", "json"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	meta, ok := worktree.Metadata(worktree.ParentBranchParams{StateDir: repo.stateDir, Branch: "feat/thing"})
	if !ok {
		t.Fatal("no metadata recorded")
	}
	if meta.EnvStrategy != domain.EnvStrategyMain {
		t.Errorf("env strategy = %q, want --env-from", meta.EnvStrategy)
	}
}

func TestCharacterizeJSONCheckout(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")

	stdout, stderr, err := runCheckoutCmd(t, "42", "--yes", "--output", "json")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
	want := `{"author":"octocat","branch":"feat/thing","existing_branch":false,"is_draft":false,"isolation":"isolated","number":42,"path":"<trees>/feat-thing","url":"https://github.com/test/test/pull/42"}`
	var normalized bytes.Buffer
	encoder := json.NewEncoder(&normalized)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(decodeCheckoutJSON(t, repo.normalize(stdout))); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(normalized.String()) != want {
		t.Errorf("json =\n%s\nwant\n%s", normalized.String(), want)
	}
}

func TestCharacterizeReusedBranchKeepsItsCommits(t *testing.T) {
	repo := newCheckoutRepo(t)
	git(t, repo.work, "branch", "feat/reused")
	git(t, repo.work, "push", "origin", "feat/reused")
	git(t, repo.work, "checkout", "feat/reused")
	git(t, repo.work, "commit", "--allow-empty", "-m", "local-only")
	tip := revParse(t, repo.work, "feat/reused")
	git(t, repo.work, "checkout", "main")

	stdout, _, err := runCheckoutCmd(t, "51", "--yes")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	want := "\n" +
		"  ✓ Checked out PR #51 (feat/reused)\n" +
		"\n" +
		"  path  <trees>/feat-reused\n" +
		"\n" +
		"  Reused your existing local branch feat/reused.\n" +
		"\n" +
		"  → wtm go feat/reused\n" +
		"\n"
	if got := repo.normalize(stdout); got != want {
		t.Errorf("stdout =\n%q\nwant\n%q", got, want)
	}
	if head := revParse(t, repo.work, "feat/reused"); head != tip {
		t.Errorf("branch moved to %s, want its local tip %s", head, tip)
	}
}

func TestCharacterizeReusedBranchBehindIsLeftAlone(t *testing.T) {
	repo := newCheckoutRepo(t)
	git(t, repo.work, "branch", "feat/reused")
	git(t, repo.work, "push", "origin", "feat/reused")
	local := revParse(t, repo.work, "feat/reused")
	git(t, repo.work, "commit", "--allow-empty", "-m", "server-commit")
	git(t, repo.work, "push", "origin", "main:feat/reused")

	stdout, _, err := runCheckoutCmd(t, "51", "--yes", "--output", "json")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if head := revParse(t, repo.work, "feat/reused"); head != local {
		t.Errorf("branch moved to %s under --yes, want %s", head, local)
	}
	got := decodeCheckoutJSON(t, stdout)
	if got["existing_branch"] != true || got["origin_state"] != domain.DivergenceLabelBehind {
		t.Errorf("json = %v, want the reuse reported behind", got)
	}
}

func TestCharacterizeABranchHeldElsewhereIsRefused(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")
	if _, _, err := runCheckoutCmd(t, "42", "--yes", "--output", "json"); err != nil {
		t.Fatalf("first checkout: %v", err)
	}
	if _, _, err := runCheckoutCmd(t, "42", "--yes", "--output", "json"); err == nil {
		t.Fatal("second checkout of the same PR succeeded, want a refusal")
	}
}

func TestCharacterizeHooksRunAsTheirOwnPhase(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")
	repo.writeConfig(t, `#:schema ./schemas/project.schema.json
[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[hooks]
on_create = ["echo hooked > hook-ran"]
`)

	stdout, stderr, err := runCheckoutCmd(t, "42", "--yes")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if !strings.Contains(stdout, "✓ Checked out PR #42 (feat/thing)") {
		t.Errorf("stdout = %q, want the conclusion", stdout)
	}
	if !strings.Contains(stderr, domain.HooksTitleOnCreate) {
		t.Errorf("stderr = %q, want the hook phase titled", stderr)
	}
	if _, err := os.Stat(filepath.Join(repo.work, "..", ".trees", "feat-thing", "hook-ran")); err != nil {
		t.Errorf("hook did not run: %v", err)
	}
	logPath := filepath.Join(repo.stateDir, "hooks")
	if entries, err := os.ReadDir(logPath); err != nil || len(entries) == 0 {
		t.Errorf("no hook log under %s: %v", logPath, err)
	}
}

func TestCharacterizeHooksFailureFailsTheCheckout(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")
	repo.writeConfig(t, `#:schema ./schemas/project.schema.json
[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[hooks]
on_create = ["exit 3"]
`)

	stdout, _, err := runCheckoutCmd(t, "42", "--yes", "--output", "json")
	if err == nil {
		t.Fatal("checkout succeeded over a failing hook")
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want no JSON for a failed checkout", stdout)
	}
}

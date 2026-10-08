package status_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/status"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

const runToml = `addressing = "ports"

[[job]]
  name = "api"
  kind = "service"
  cmd = "true"
  [job.ports]
    PORT = 3000
  [job.url]
    port = "PORT"

[[job]]
  name = "worker"
  kind = "service"
  cmd = "true"
`

const secret = "s3cr3t-value"

type fixture struct {
	repo   string
	linked string
	ctx    flow.Context
}

func setup(t *testing.T) fixture {
	t.Helper()
	globaldir.Isolate(t)
	repo, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(repo, ".git", "wtm")
	write(t, filepath.Join(stateDir, domain.RunFileName), runToml)
	write(t, filepath.Join(repo, ".env"), "TOKEN="+secret+"\n")
	linked := filepath.Join(t.TempDir(), "feat")
	gittest.Git(t, repo, "worktree", "add", "-b", "feat/x", linked)
	linked, err = filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{
		repo:   repo,
		linked: linked,
		ctx: flow.Context{
			ProjectDir: repo,
			StateDir:   stateDir,
			Config: domain.Config{Project: domain.ProjectConfig{Env: domain.EnvConfig{Strategy: domain.EnvStrategyMain, Files: []domain.EnvFile{
				{Target: ".env"},
				{Target: "apps/e2e/.env", Template: "apps/e2e/.env.example"},
			}}}},
		},
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func jobs(infos ...domain.JobInfo) func() []domain.JobInfo {
	return func() []domain.JobInfo { return infos }
}

func codes(doc domain.StatusDocument) []domain.StatusProblemCode {
	got := []domain.StatusProblemCode{}
	for _, problem := range doc.Problems {
		got = append(got, problem.Code)
	}
	return got
}

func TestStatusReadsTheCurrentWorktreeWithTheStateOfEveryDeclaredJob(t *testing.T) {
	f := setup(t)
	exit := 1

	doc, err := runStatus(t, status.Params{
		Context: f.ctx,
		Request: status.Request{Cwd: f.repo},
		Jobs: jobs(
			domain.JobInfo{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusRunning, WorkDir: f.repo},
			domain.JobInfo{Name: "worker", Kind: domain.JobKindService, Status: domain.JobStatusCrashed, ExitCode: &exit, WorkDir: f.repo},
			domain.JobInfo{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusRunning, WorkDir: f.linked},
		),
	})
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if doc.Branch != "main" || !doc.Main || doc.Path != f.repo || !doc.RunConfig {
		t.Errorf("doc = %+v, want the main checkout with its run.toml", doc)
	}
	if doc.Addressing == nil || *doc.Addressing != domain.AddressingPorts || doc.Offset == nil || *doc.Offset != 0 {
		t.Errorf("addressing = %v, offset = %v, want ports at +0", doc.Addressing, doc.Offset)
	}
	if len(doc.Jobs) != 2 || doc.Jobs[0].State != domain.JobStateRunning || doc.Jobs[1].State != domain.JobStateCrashed {
		t.Fatalf("jobs = %+v, want api running and worker crashed, the linked worktree's api left out", doc.Jobs)
	}
	if !strings.HasSuffix(doc.Jobs[0].URL, ":3000") {
		t.Errorf("api url = %q, want its port", doc.Jobs[0].URL)
	}
	if doc.Env.Declared != 2 || len(doc.Env.Missing) != 1 || doc.Env.Missing[0] != "apps/e2e/.env" {
		t.Errorf("env = %+v, want 2 declared and apps/e2e/.env missing", doc.Env)
	}
	want := []domain.StatusProblemCode{domain.StatusProblemEnvMissing, domain.StatusProblemJobCrashed}
	if got := codes(doc); strings.Join(toStrings(got), ",") != strings.Join(toStrings(want), ",") {
		t.Errorf("problems = %v, want %v", got, want)
	}
}

func toStrings(codes []domain.StatusProblemCode) []string {
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		out = append(out, string(code))
	}
	return out
}

func TestStatusNeverPutsAnEnvValueInTheDocument(t *testing.T) {
	f := setup(t)

	doc, err := runStatus(t, status.Params{Context: f.ctx, Request: status.Request{Cwd: f.repo}, Jobs: jobs()})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secret) {
		t.Errorf("document %s carries an env value", data)
	}
}

func TestANamedWorktreeMissingAnEnvFileIsToldToRunEnv(t *testing.T) {
	f := setup(t)

	doc, err := runStatus(t, status.Params{Context: f.ctx, Request: status.Request{Worktree: "feat/x", Cwd: f.repo}, Jobs: jobs()})
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if doc.Branch != "feat/x" || doc.Main || doc.Path != f.linked {
		t.Fatalf("doc = %+v, want feat/x", doc)
	}
	if len(doc.Problems) == 0 || doc.Problems[0].Fix != "wtm env feat/x --yes" {
		t.Errorf("problems = %+v, want .env rebuilt by wtm env from main, the project's strategy", doc.Problems)
	}
}

// Reading a worktree's addresses must not be what numbers it: the ordinal is
// allocated by the run that needs it, and published as worktree.updated.
func TestStatusNeverNumbersTheWorktreeItReads(t *testing.T) {
	f := setup(t)

	doc, err := runStatus(t, status.Params{Context: f.ctx, Request: status.Request{Cwd: f.linked}, Jobs: jobs()})
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if doc.Offset != nil {
		t.Errorf("offset = %d, want none for a worktree no run has numbered", *doc.Offset)
	}
	identity, err := worktree.Identity(t.Context(), worktree.WorktreeRef{ProjectDir: f.repo, StateDir: f.ctx.StateDir, Branch: "feat/x"})
	if err != nil {
		t.Fatal(err)
	}
	if identity.Ordinal != nil {
		t.Errorf("ordinal = %d after a status, want none", *identity.Ordinal)
	}
	if doc.Jobs[0].URL != "" {
		t.Errorf("api url = %q, want none without an offset", doc.Jobs[0].URL)
	}
}

func TestStatusAllReadsEveryWorktree(t *testing.T) {
	f := setup(t)

	docs, err := status.RunAll(t.Context(), unattended(status.Params{Context: f.ctx, Request: status.Request{Cwd: f.repo}, Jobs: jobs()}))
	if err != nil {
		t.Fatalf("status --all: %v", err)
	}

	if len(docs) != 2 || docs[0].Branch != "main" || docs[1].Branch != "feat/x" {
		t.Fatalf("docs = %+v, want main then feat/x", docs)
	}
}

func TestWithoutRunTomlTheJobsAndAddressesAreEmpty(t *testing.T) {
	f := setup(t)
	if err := os.Remove(filepath.Join(f.ctx.StateDir, domain.RunFileName)); err != nil {
		t.Fatal(err)
	}

	doc, err := runStatus(t, status.Params{Context: f.ctx, Request: status.Request{Cwd: f.repo}, Jobs: jobs()})
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if doc.RunConfig || doc.Addressing != nil || doc.Offset != nil || len(doc.Jobs) != 0 || doc.Jobs == nil {
		t.Errorf("doc = %+v, want no run part and an empty jobs list", doc)
	}
}

func unattended(params status.Params) status.Params {
	params.Prompter = flow.Unattended{}
	params.Presenter = &flowtest.Recorder{}
	return params
}

// runStatus is a run nobody can be asked in: no terminal, JSON or --quiet.
func runStatus(t *testing.T, params status.Params) (domain.StatusDocument, error) {
	outcome, err := status.Run(t.Context(), unattended(params))
	return outcome.Document, err
}

// Without a positional, a fully interactive run asks which worktree with the
// run module's own picker, opened on the one it was launched from.
func TestTheInteractiveRunPicksTheWorktreeOpenedOnTheCurrentOne(t *testing.T) {
	f := setup(t)
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{target.KeyWorktree: f.linked}}

	outcome, err := status.Run(t.Context(), status.Params{Context: f.ctx, Request: status.Request{Cwd: f.repo}, Prompter: prompter, Presenter: &flowtest.Recorder{}, Jobs: jobs()})
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if prompter.AskedKeys() != target.KeyWorktree {
		t.Errorf("asked %s, want the worktree picker alone", prompter.AskedKeys())
	}
	if start := prompter.Content[target.KeyWorktree].Start; start != f.repo {
		t.Errorf("picker opens on %q, want the current worktree %q", start, f.repo)
	}
	if outcome.Document.Branch != "feat/x" {
		t.Errorf("branch = %q, want the picked worktree", outcome.Document.Branch)
	}
}

func TestAPositionalAnswersThePickerWithoutAsking(t *testing.T) {
	f := setup(t)
	prompter := &flowtest.ScriptedPrompter{}

	outcome, err := status.Run(t.Context(), status.Params{Context: f.ctx, Request: status.Request{Worktree: "feat/x", Cwd: f.repo}, Prompter: prompter, Presenter: &flowtest.Recorder{}, Jobs: jobs()})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(prompter.Asked) != 0 || outcome.Document.Branch != "feat/x" {
		t.Errorf("asked %v, branch %q; want feat/x and no question", prompter.Asked, outcome.Document.Branch)
	}
}

func TestBackingOutOfThePickerReadsNothing(t *testing.T) {
	f := setup(t)
	recorder := &flowtest.Recorder{}

	outcome, err := status.Run(t.Context(), status.Params{Context: f.ctx, Request: status.Request{Cwd: f.repo}, Prompter: &flowtest.ScriptedPrompter{Abort: true}, Presenter: recorder, Jobs: jobs()})
	if err != nil || !outcome.Aborted {
		t.Fatalf("outcome = %+v, err = %v; want an abort", outcome, err)
	}
	if len(recorder.Stages) != 0 {
		t.Errorf("stages = %v, want nothing read after the abort", recorder.Stages)
	}
}

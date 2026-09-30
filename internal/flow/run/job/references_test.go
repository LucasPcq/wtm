package job_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	jobflow "github.com/LucasPcq/wtm/internal/flow/run/job"
	"github.com/LucasPcq/wtm/internal/rules"
)

func removeUnattended(t *testing.T, ctx flow.Context, request jobflow.RemoveRequest) (*recorder, jobflow.Outcome, error) {
	t.Helper()
	presenter := &recorder{}
	outcome, err := jobflow.Remove(jobflow.RemoveParams{
		Context: ctx, Request: request, Prompter: flow.Unattended{}, Presenter: presenter,
	})
	return presenter, outcome, err
}

// The refusal called every reference a profile, runners and touchers included.
// Each kind is named as what it is, so the reader knows where to look.
func TestRemoveRefusalNamesEachKindOfReference(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "db", Kind: domain.JobKindService, Cmd: "true", Ports: map[string]int{"PG": 5432}},
			{Name: "dev", Kind: domain.JobKindService, Cmd: "turbo dev", Runs: []string{"db"}},
			{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate", Touches: []string{"db"}},
		},
		Profiles: []domain.ProfileConfig{{Name: "full", Jobs: []string{"db", "dev"}}},
		EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "DATABASE_URL", Job: "db", Port: "PG"}},
	}

	_, _, err := removeUnattended(t, flow.Context{StateDir: t.TempDir()}, jobflow.RemoveRequest{Name: "db", Config: cfg})
	if err == nil {
		t.Fatal("a referenced job was removed without --force")
	}
	for _, want := range []string{"profiles full", "runs of dev", "touches of migrate", "[[env_port]] DATABASE_URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to name %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "profile(s): full, dev") {
		t.Errorf("err = %v, still calls a runner a profile", err)
	}
}

// A job only the .env links named was removed without a word, taking the links
// with it.
func TestRemoveRefusesAJobOnlyEnvLinksName(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs:      []domain.JobConfig{{Name: "db", Kind: domain.JobKindService, Cmd: "true", Scope: domain.JobScopeShared, Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "createdb"}}},
		EnvValues: []domain.EnvValueLink{{File: ".env", Key: "DB_NAME", Job: "db", Value: "{namespace}"}},
	}

	_, _, err := removeUnattended(t, flow.Context{StateDir: t.TempDir()}, jobflow.RemoveRequest{Name: "db", Config: cfg})
	if err == nil || !strings.Contains(err.Error(), "[[env]] DB_NAME") {
		t.Fatalf("err = %v, want a refusal naming the [[env]] link", err)
	}
}

func writeMeta(t *testing.T, stateDir, branch string, namespaces ...string) {
	t.Helper()
	dir := rules.WorktreeMetaDir(stateDir, branch)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(domain.WorktreeMetadata{SourceBranch: "main", Namespaces: namespaces})
	if err := os.WriteFile(filepath.Join(dir, domain.MetaFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func sharedDB() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up -d db",
		Scope:     domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "createdb"},
	}}}
}

// clean drops a worktree's namespace by the job's name: removing the job leaves
// that data with nothing to drop it.
func TestRemoveRefusesASharedJobWorktreesHoldDataIn(t *testing.T) {
	ctx := flow.Context{StateDir: t.TempDir()}
	writeMeta(t, ctx.StateDir, "feat/a", "db")
	writeMeta(t, ctx.StateDir, "feat/b")

	_, _, err := removeUnattended(t, ctx, jobflow.RemoveRequest{Name: "db", Config: sharedDB()})
	if err == nil || !strings.Contains(err.Error(), "feat/a") || strings.Contains(err.Error(), "feat/b") {
		t.Fatalf("err = %v, want a refusal naming feat/a only", err)
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagForce) {
		t.Errorf("err = %v, want it to name --%s", err, domain.FlagForce)
	}
}

func TestRemoveWithForceWarnsTheDataWillNotBeDropped(t *testing.T) {
	ctx := flow.Context{StateDir: t.TempDir()}
	writeMeta(t, ctx.StateDir, "feat/a", "db")

	presenter, outcome, err := removeUnattended(t, ctx, jobflow.RemoveRequest{Name: "db", Force: true, Config: sharedDB()})
	if err != nil {
		t.Fatalf("Remove --force: %v", err)
	}
	if outcome.Status != domain.JobActionRemoved {
		t.Errorf("outcome = %+v, want removed", outcome)
	}
	if !noticeContaining(presenter.Notices, "feat/a") {
		t.Errorf("notices = %+v, want a warning naming feat/a", presenter.Notices)
	}
}

// A namespace owed by a clean that could not reach the service is held too.
func TestRemoveRefusesASharedJobWithAPendingRemoval(t *testing.T) {
	ctx := flow.Context{StateDir: t.TempDir()}
	pending := "[[pending]]\njob = \"db\"\nworktree = \"feat-gone\"\nordinal = 3\n"
	if err := os.WriteFile(filepath.Join(ctx.StateDir, domain.PendingRemovalsFileName), []byte(pending), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := removeUnattended(t, ctx, jobflow.RemoveRequest{Name: "db", Config: sharedDB()})
	if err == nil || !strings.Contains(err.Error(), "feat-gone") {
		t.Fatalf("err = %v, want a refusal naming the pending removal", err)
	}
}

// Renaming the job is the same loss without even a flag to take it knowingly.
func TestRenameRefusesASharedJobWorktreesHoldDataIn(t *testing.T) {
	ctx := flow.Context{StateDir: t.TempDir()}
	writeMeta(t, ctx.StateDir, "feat/a", "db")
	name := "postgres"

	_, err := jobflow.Edit(jobflow.EditParams{
		Context:   ctx,
		Request:   jobflow.EditRequest{Name: "db", Patch: rules.JobPatch{Name: &name}, Config: sharedDB()},
		Prompter:  flow.Unattended{},
		Presenter: &recorder{},
	})
	if err == nil || !strings.Contains(err.Error(), "feat/a") {
		t.Fatalf("err = %v, want a refusal naming feat/a", err)
	}
}

// A job published without url.host is published under its name, so renaming it
// moves its address.
func TestRenameWarnsWhenThePublishedHostChanges(t *testing.T) {
	ctx := flow.Context{StateDir: t.TempDir()}
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev",
		Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"},
	}}}
	name := "backend"
	presenter := &recorder{}

	_, err := jobflow.Edit(jobflow.EditParams{
		Context:   ctx,
		Request:   jobflow.EditRequest{Name: "api", Patch: rules.JobPatch{Name: &name}, Config: cfg},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if !noticeContaining(presenter.Notices, "backend") {
		t.Errorf("notices = %+v, want the new host said", presenter.Notices)
	}
}

func noticeContaining(notices []flow.Notice, text string) bool {
	for _, notice := range notices {
		if strings.Contains(notice.Text, text) {
			return true
		}
	}
	return false
}

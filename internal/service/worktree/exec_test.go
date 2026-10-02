package worktree

import (
	"os"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestExecCandidatesSkipsVanishedDirectoriesAndKeepsMain(t *testing.T) {
	repo := newOrdinalRepo(t)
	gone := repo.addWorktree(t, "gone")
	repo.addWorktree(t, "kept")
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	candidates, err := ExecCandidates(ExecCandidatesParams{ProjectDir: repo.dir})
	if err != nil {
		t.Fatal(err)
	}
	branches := map[string]bool{}
	mainSeen := false
	for _, candidate := range candidates {
		branches[candidate.Branch] = true
		mainSeen = mainSeen || candidate.IsMain
	}
	if branches["gone"] || !branches["kept"] || !mainSeen {
		t.Fatalf("candidates = %v, main seen = %v", branches, mainSeen)
	}
}

func TestExecEnvAlwaysStripsTheCallersWorktreeVariables(t *testing.T) {
	t.Setenv(domain.EnvComposeProjectName, "caller-project")
	t.Setenv(domain.EnvBranch, "caller-branch")
	repo := newOrdinalRepo(t)
	path := repo.addWorktree(t, "plain")

	env := ExecEnv(ExecEnvParams{Ref: WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "plain"}, WorktreePath: path})

	if _, ok := rules.LookupEnv(env, domain.EnvComposeProjectName); ok {
		t.Error("the caller's COMPOSE_PROJECT_NAME leaked into another worktree")
	}
	if _, ok := rules.LookupEnv(env, domain.EnvBranch); ok {
		t.Error("the caller's branch variable leaked")
	}
	if _, ok := rules.LookupEnv(env, "PATH"); !ok {
		t.Error("the rest of the environment must be inherited")
	}
}

func TestExecEnvCarriesTheTargetsRunVariables(t *testing.T) {
	unsetComposeProject(t)
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, composeJobConfig)
	path := repo.addWorktree(t, "feat/x")
	recordIsolation(t, repo, "feat/x", domain.IsolationIsolated)

	env := ExecEnv(ExecEnvParams{Ref: WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "feat/x"}, WorktreePath: path})

	saw := hookSaw(t, repo, "feat/x", path)
	for _, key := range append([]string{"DB_PORT"}, domain.WorktreeScopedEnv...) {
		want, hookHas := saw[key]
		got, execHas := rules.LookupEnv(env, key)
		if hookHas != execHas || got != want {
			t.Errorf("%s: exec has %q (%v), hook has %q (%v)", key, got, execHas, want, hookHas)
		}
	}
	if got, _ := rules.LookupEnv(env, "DB_PORT"); got != "5442" {
		t.Errorf("DB_PORT = %q, want the shifted 5442", got)
	}
}

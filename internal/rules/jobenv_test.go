package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestWorktreeSlug(t *testing.T) {
	cases := []struct {
		name   string
		branch string
		want   string
	}{
		{"simple", "main", "main"},
		{"slashes", "feat/isolation", "feat-isolation"},
		{"majuscules refusées par compose", "feat/LUC-99", "feat-luc-99"},
		{"underscore conservé", "feat/my_branch", "feat-my_branch"},
		{"caractères exotiques", "feat/été+2026", "feat--t--2026"},
		{"préfixe non alphanumérique", "-/-feat", "feat"},
		{"vide", "", domain.ComposeProjectFallback},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := WorktreeSlug(c.branch); got != c.want {
				t.Errorf("WorktreeSlug(%q) = %q, want %q", c.branch, got, c.want)
			}
		})
	}
}

func TestWorktreeJobEnv(t *testing.T) {
	cases := []struct {
		name   string
		params WorktreeJobEnvParams
		want   map[string]string
	}{
		{
			name:   "worktree principal garde les ports par défaut",
			params: WorktreeJobEnvParams{Branch: "main", Ordinal: 0, PortOffsetBlock: 10},
			want: map[string]string{
				domain.EnvWorktree:           "main",
				domain.EnvBranch:             "main",
				domain.EnvOrdinal:            "0",
				domain.EnvPortOffset:         "0",
				domain.EnvComposeProjectName: "main",
				domain.EnvProject:            domain.HostLabelFallback,
				domain.EnvIsolation:          string(domain.IsolationIsolated),
			},
		},
		{
			name:   "offset = ordinal x bloc",
			params: WorktreeJobEnvParams{Branch: "feat/x", Ordinal: 3, PortOffsetBlock: 10},
			want: map[string]string{
				domain.EnvWorktree:           "feat-x",
				domain.EnvBranch:             "feat/x",
				domain.EnvOrdinal:            "3",
				domain.EnvPortOffset:         "30",
				domain.EnvComposeProjectName: "feat-x",
				domain.EnvProject:            domain.HostLabelFallback,
				domain.EnvIsolation:          string(domain.IsolationIsolated),
			},
		},
		{
			name:   "bloc absent retombe sur le défaut",
			params: WorktreeJobEnvParams{Branch: "feat/x", Ordinal: 2},
			want: map[string]string{
				domain.EnvWorktree:           "feat-x",
				domain.EnvBranch:             "feat/x",
				domain.EnvOrdinal:            "2",
				domain.EnvPortOffset:         "20",
				domain.EnvComposeProjectName: "feat-x",
				domain.EnvProject:            domain.HostLabelFallback,
				domain.EnvIsolation:          string(domain.IsolationIsolated),
			},
		},
		{
			name:   "COMPOSE_PROJECT_NAME défini par l'utilisateur non écrasé",
			params: WorktreeJobEnvParams{Branch: "feat/x", Ordinal: 1, PortOffsetBlock: 10, ComposeProject: "perso"},
			want: map[string]string{
				domain.EnvWorktree:           "feat-x",
				domain.EnvBranch:             "feat/x",
				domain.EnvOrdinal:            "1",
				domain.EnvPortOffset:         "10",
				domain.EnvComposeProjectName: "perso",
				domain.EnvProject:            domain.HostLabelFallback,
				domain.EnvIsolation:          string(domain.IsolationIsolated),
			},
		},
		{
			name:   "le dépôt est assaini pour servir de label d'hôte",
			params: WorktreeJobEnvParams{Branch: "feat/x", Ordinal: 1, PortOffsetBlock: 10, Project: "My.App"},
			want: map[string]string{
				domain.EnvWorktree:           "feat-x",
				domain.EnvBranch:             "feat/x",
				domain.EnvOrdinal:            "1",
				domain.EnvPortOffset:         "10",
				domain.EnvComposeProjectName: "my-app-feat-x",
				domain.EnvProject:            "my-app",
				domain.EnvIsolation:          string(domain.IsolationIsolated),
			},
		},
		{
			name:   "verbatim tourne sur les ports de base, sans projet compose inventé",
			params: WorktreeJobEnvParams{Branch: "feat/x", Ordinal: 3, PortOffsetBlock: 10, Isolation: domain.IsolationVerbatim},
			want: map[string]string{
				domain.EnvWorktree:   "feat-x",
				domain.EnvBranch:     "feat/x",
				domain.EnvOrdinal:    "3",
				domain.EnvPortOffset: "0",
				domain.EnvProject:    domain.HostLabelFallback,
				domain.EnvIsolation:  string(domain.IsolationVerbatim),
			},
		},
		{
			name:   "verbatim garde le COMPOSE_PROJECT_NAME posé par l'utilisateur",
			params: WorktreeJobEnvParams{Branch: "feat/x", Ordinal: 3, PortOffsetBlock: 10, ComposeProject: "perso", Isolation: domain.IsolationVerbatim},
			want: map[string]string{
				domain.EnvWorktree:           "feat-x",
				domain.EnvBranch:             "feat/x",
				domain.EnvOrdinal:            "3",
				domain.EnvPortOffset:         "0",
				domain.EnvComposeProjectName: "perso",
				domain.EnvProject:            domain.HostLabelFallback,
				domain.EnvIsolation:          string(domain.IsolationVerbatim),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := WorktreeJobEnv(c.params)
			if len(got) != len(c.want) {
				t.Fatalf("WorktreeJobEnv() = %v, want %v", got, c.want)
			}
			for key, want := range c.want {
				if got[key] != want {
					t.Errorf("WorktreeJobEnv()[%s] = %q, want %q", key, got[key], want)
				}
			}
		})
	}
}

func TestComposeProjectName(t *testing.T) {
	cases := []struct {
		name     string
		project  string
		worktree string
		want     string
	}{
		// The Docker daemon is machine-wide: two clones both sitting on `main`
		// must not land on the same stack.
		{"qualifié par le dépôt", "myproject", "main", "myproject-main"},
		{"dépôt aux majuscules", "MyProject", "feat-x", "myproject-feat-x"},
		{"dépôt inconnu", "", "feat-x", "feat-x"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ComposeProjectName(ComposeProjectNameParams{Project: c.project, Worktree: c.worktree})
			if got != c.want {
				t.Errorf("ComposeProjectName(%q, %q) = %q, want %q", c.project, c.worktree, got, c.want)
			}
		})
	}
}

func TestWorktreeJobEnvQualifiesTheComposeProject(t *testing.T) {
	env := WorktreeJobEnv(WorktreeJobEnvParams{Branch: "main", Project: "myproject", Ordinal: 0})
	if got := env[domain.EnvComposeProjectName]; got != "myproject-main" {
		t.Errorf("%s = %q, want %q", domain.EnvComposeProjectName, got, "myproject-main")
	}

	// A name the user set for this run is an answer, not a value to qualify.
	env = WorktreeJobEnv(WorktreeJobEnvParams{Branch: "main", Project: "myproject", ComposeProject: "perso"})
	if got := env[domain.EnvComposeProjectName]; got != "perso" {
		t.Errorf("%s = %q, want %q", domain.EnvComposeProjectName, got, "perso")
	}
}

func TestMainComposeProjectName(t *testing.T) {
	pairs := func(entries ...string) []domain.EnvLine {
		lines := make([]domain.EnvLine, 0, len(entries))
		for _, entry := range entries {
			key, value, _ := strings.Cut(entry, "=")
			lines = append(lines, domain.EnvLine{Kind: domain.EnvLinePair, Key: key, Value: value})
		}
		return lines
	}
	cases := []struct {
		name     string
		envFiles [][]domain.EnvLine
		want     string
	}{
		{"sans .env, le dépôt seul", nil, "my-app"},
		{".env sans le nom", [][]domain.EnvLine{pairs("DB_PORT=5432")}, "my-app"},
		{"le .env nomme le projet", [][]domain.EnvLine{pairs("COMPOSE_PROJECT_NAME=stack")}, "stack"},
		{"valeur vide ignorée", [][]domain.EnvLine{pairs("COMPOSE_PROJECT_NAME="), pairs("COMPOSE_PROJECT_NAME=infra")}, "infra"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MainComposeProjectName(MainComposeProjectNameParams{Project: "My-App", EnvFiles: c.envFiles})
			if got != c.want {
				t.Errorf("MainComposeProjectName = %q, want %q", got, c.want)
			}
		})
	}
}

func TestPurgeableMetaDir(t *testing.T) {
	cases := []struct {
		name     string
		stateDir string
		branch   string
		want     string
	}{
		{"branche normale", "/state", "feat/x", "/state/worktrees/feat%2Fx"},
		{"state dir absent", "", "feat/x", ""},
		{"branche absente", "/state", "", ""},
		{"branche remontant d'un cran", "/state", "..", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := PurgeableMetaDir(PurgeableMetaDirParams{StateDir: c.stateDir, Branch: c.branch})
			if got != c.want {
				t.Errorf("PurgeableMetaDir(%q, %q) = %q, want %q", c.stateDir, c.branch, got, c.want)
			}
		})
	}
}

func TestRunEnvReachesHooks(t *testing.T) {
	compose := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "db", Cmd: "docker compose up -d"}}}
	script := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}}}}

	cases := []struct {
		name     string
		params   RunEnvReachesHooksParams
		expected bool
	}{
		{"isolated with a compose job", RunEnvReachesHooksParams{Config: compose, Recorded: domain.IsolationIsolated}, true},
		{"verbatim with a compose job", RunEnvReachesHooksParams{Config: compose, Recorded: domain.IsolationVerbatim}, true},
		{"never chose", RunEnvReachesHooksParams{Config: compose}, false},
		{"no compose job", RunEnvReachesHooksParams{Config: script, Recorded: domain.IsolationIsolated}, false},
		{"no run config", RunEnvReachesHooksParams{Recorded: domain.IsolationIsolated}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RunEnvReachesHooks(tc.params); got != tc.expected {
				t.Errorf("RunEnvReachesHooks = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestDeclaredComposeProjectIsEmptyWhenNoFileNamesOne(t *testing.T) {
	files := [][]domain.EnvLine{ParseEnv("PORT=1\n"), ParseEnv("COMPOSE_PROJECT_NAME=\n")}
	if got := DeclaredComposeProject(files); got != "" {
		t.Errorf("DeclaredComposeProject = %q, want empty", got)
	}
	files = append(files, ParseEnv("COMPOSE_PROJECT_NAME=stack\n"))
	if got := DeclaredComposeProject(files); got != "stack" {
		t.Errorf("DeclaredComposeProject = %q, want %q", got, "stack")
	}
}

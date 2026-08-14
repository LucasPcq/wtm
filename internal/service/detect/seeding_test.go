package detect

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// TestProjectEnvironmentSeedsInstallHooks walks the real seam — detection feeding
// the answer builder — for each repo layout, so the end-to-end result of
// `wtm init` is pinned, not just the pieces in isolation.
func TestProjectEnvironmentSeedsInstallHooks(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		wantKind domain.MonorepoKind
		want     []domain.HookCommand
	}{
		{
			name: "pnpm workspace monorepo gets a single root install",
			files: map[string]string{
				"pnpm-workspace.yaml":     "packages:\n  - \"apps/*\"\n",
				"pnpm-lock.yaml":          "",
				"package.json":            `{"name":"root"}`,
				"apps/web/package.json":   `{"name":"web"}`,
				"apps/api/package.json":   `{"name":"api"}`,
				"apps/api/pnpm-lock.yaml": "",
			},
			wantKind: domain.MonorepoKindWorkspace,
			want:     []domain.HookCommand{{Cmd: domain.InstallCommandPnpm}},
		},
		{
			name: "npm workspaces monorepo gets a single root install",
			files: map[string]string{
				"package-lock.json":     "{}",
				"package.json":          `{"name":"root","workspaces":["apps/*"]}`,
				"apps/web/package.json": `{"name":"web"}`,
			},
			wantKind: domain.MonorepoKindWorkspace,
			want:     []domain.HookCommand{{Cmd: domain.InstallCommandNpm}},
		},
		{
			name: "independent sub-projects each get their own install",
			files: map[string]string{
				"front/package-lock.json": "{}",
				"front/package.json":      `{"name":"front"}`,
				"api/requirements.txt":    "",
			},
			wantKind: domain.MonorepoKindIndependent,
			want: []domain.HookCommand{
				{Cmd: domain.InstallCommandPip, Cwd: "api"},
				{Cmd: domain.InstallCommandNpm, Cwd: "front"},
			},
		},
		{
			name: "plain single project gets a single root install",
			files: map[string]string{
				"package-lock.json": "{}",
				"package.json":      `{"name":"app"}`,
			},
			wantKind: domain.MonorepoKindNone,
			want:     []domain.HookCommand{{Cmd: domain.InstallCommandNpm}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				writeFile(t, filepath.Join(dir, filepath.FromSlash(name)), content)
			}

			detection := ProjectEnvironment(dir)
			if detection.Monorepo.Kind != tc.wantKind {
				t.Fatalf("Kind = %q, want %q", detection.Monorepo.Kind, tc.wantKind)
			}

			// BaseBranch is undetectable outside a git repo; supply it so the builder
			// reaches the hooks section instead of erroring.
			answers, err := rules.BuildProjectAnswers(rules.InitProjectFlags{BaseBranch: "main"}, detection)
			if err != nil {
				t.Fatalf("BuildProjectAnswers: %v", err)
			}

			if len(answers.OnCreate) != len(tc.want) {
				t.Fatalf("OnCreate = %+v, want %+v", answers.OnCreate, tc.want)
			}
			for i := range tc.want {
				if answers.OnCreate[i] != tc.want[i] {
					t.Errorf("OnCreate[%d] = %+v, want %+v", i, answers.OnCreate[i], tc.want[i])
				}
			}
		})
	}
}

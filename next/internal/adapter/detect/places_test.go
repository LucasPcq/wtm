package detect

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
)

func TestAWorkspaceListsTheRootThenEachPackageItSelectsWithItsScripts(t *testing.T) {
	dir := repository(t, map[string]string{
		"package.json":                  `{"name": "acme", "packageManager": "pnpm@9.12.0", "scripts": {"dev": "turbo run dev", "build": "turbo run build"}}`,
		"pnpm-workspace.yaml":           "packages:\n  - \"apps/*\"\n  - packages/**\n  - \"!apps/legacy\"\n",
		"apps/api/package.json":         `{"name": "@acme/api", "scripts": {"dev": "tsx watch src/server.ts"}}`,
		"apps/legacy/package.json":      `{"name": "legacy", "scripts": {"dev": "x"}}`,
		"packages/db/package.json":      `{"name": "@acme/db", "scripts": {"seed": "tsx seed.ts", "migrate": "prisma migrate deploy"}}`,
		"packages/ui/lib/package.json":  `{"scripts": {}}`,
		"tools/package.json":            `{"name": "tools", "scripts": {"dev": "x"}}`,
		"node_modules/x/package.json":   `{"scripts": {"dev": "x"}}`,
		"apps/api/node_modules/y/a.txt": "",
	})

	workspace := Repository{Dir: dir}.Workspace()

	assert.Equal(t, project.Workspace{Manager: "pnpm", ManagerFrom: "package.json", Places: []project.Place{
		{Dir: ".", Name: "acme", Scripts: []project.Script{{Name: "build", Cmd: "turbo run build"}, {Name: "dev", Cmd: "turbo run dev"}}},
		{Dir: "apps/api", Name: "api", Scripts: []project.Script{{Name: "dev", Cmd: "tsx watch src/server.ts"}}},
		{Dir: "packages/db", Name: "db", Scripts: []project.Script{{Name: "migrate", Cmd: "prisma migrate deploy"}, {Name: "seed", Cmd: "tsx seed.ts"}}},
		{Dir: "packages/ui/lib", Name: "lib", Scripts: []project.Script{}},
	}}, workspace)
}

func TestThePackageManagerIsThePackageManagerFieldElseALockfile(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		want  project.Workspace
	}{
		"field":    {files: map[string]string{"package.json": `{"packageManager": "yarn@4.0.0"}`, "pnpm-lock.yaml": ""}, want: project.Workspace{Manager: "yarn", ManagerFrom: "package.json"}},
		"lockfile": {files: map[string]string{"package.json": `{}`, "pnpm-lock.yaml": ""}, want: project.Workspace{Manager: "pnpm", ManagerFrom: "pnpm-lock.yaml"}},
		"none":     {files: map[string]string{"package.json": `{}`}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			workspace := Repository{Dir: repository(t, c.files)}.Workspace()

			workspace.Places = nil
			assert.Equal(t, c.want, workspace)
		})
	}
}

func TestNpmWorkspacesAreReadAsAnArrayOrAsYarnsObject(t *testing.T) {
	for name, workspaces := range map[string]string{"array": `["apps/*"]`, "object": `{"packages": ["apps/*"]}`} {
		t.Run(name, func(t *testing.T) {
			dir := repository(t, map[string]string{"package.json": `{"workspaces": ` + workspaces + `}`, "apps/web/package.json": `{"name": "web"}`})

			workspace := Repository{Dir: dir}.Workspace()

			assert.Equal(t, []string{".", "apps/web"}, dirs(workspace.Places))
		})
	}
}

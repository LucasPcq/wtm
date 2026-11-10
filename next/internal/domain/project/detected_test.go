package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvFilesPairEachFileWithItsPreferredTemplate(t *testing.T) {
	files := EnvFilesOf([]string{
		"apps/api/.env", "apps/api/.env.sample", "apps/api/.env.example",
		"apps/web/.env.dist", "apps/web/.env.local",
		".env.production", "README.md",
	})

	assert.Equal(t, []EnvFile{
		{Target: "apps/api/.env", Template: "apps/api/.env.example"},
		{Target: "apps/web/.env", Template: "apps/web/.env.dist"},
		{Target: "apps/web/.env.local", Local: true},
	}, files)
}

func TestTheInstallHookFollowsPackageJSONThenTheLockfile(t *testing.T) {
	cases := map[string]struct {
		manager string
		files   []string
		want    FoundHook
		found   bool
	}{
		"package_manager": {manager: "pnpm@9.0.0", files: []string{"package-lock.json"}, want: FoundHook{Hook: Hook{Cmd: "pnpm install"}, File: "package.json"}, found: true},
		"lockfile":        {files: []string{"README.md", "yarn.lock"}, want: FoundHook{Hook: Hook{Cmd: "yarn install"}, File: "yarn.lock"}, found: true},
		"unknown_manager": {manager: "deno@2", files: []string{"go.mod"}, want: FoundHook{Hook: Hook{Cmd: "go mod download"}, File: "go.mod"}, found: true},
		"none":            {files: []string{"README.md"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, found := InstallOf(c.manager, c.files)

			assert.Equal(t, c.found, found)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestAComposeFileAtTheRootIsTornDownBeforeARemoval(t *testing.T) {
	got, found := TeardownOf([]string{"package.json", "docker-compose.yml"})

	assert.True(t, found)
	assert.Equal(t, FoundHook{Hook: Hook{Cmd: "docker compose down"}, File: "docker-compose.yml"}, got)
	_, found = TeardownOf([]string{"package.json"})
	assert.False(t, found)
}

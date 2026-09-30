package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestDockerFilesConfigured(t *testing.T) {
	run := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Cmd: "docker compose -f docker-compose.yml up -d"},
	}}
	got := rules.DockerFilesConfigured(run, []string{"docker-compose.yml", "docker-compose.dev.yml"})
	if !got["docker-compose.yml"] {
		t.Error("docker-compose.yml should be configured")
	}
	if got["docker-compose.dev.yml"] {
		t.Error("docker-compose.dev.yml should not be configured")
	}
}

func TestScriptsConfigured(t *testing.T) {
	run := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "dev", Cmd: "pnpm run dev", Cwd: "."},
		{Name: "api-dev", Cmd: "pnpm run dev", Cwd: "packages/api"},
	}}
	scripts := []domain.PackageScript{
		{Name: "dev"},                            // index 0 → root, configured
		{Name: "build"},                          // index 1 → not configured
		{Name: "dev", Workspace: "packages/api"}, // index 2 → configured
	}
	got := rules.ScriptsConfigured(run, scripts, domain.PkgManagerPnpm)
	if !got[0] || got[1] || !got[2] {
		t.Errorf("got %v, want {0:true, 1:false, 2:true}", got)
	}
}

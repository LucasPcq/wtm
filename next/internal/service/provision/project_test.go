package provision_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/stretchr/testify/assert"
)

func TestTheProjectProvisionedIsTheOneObserved(t *testing.T) {
	runErr := errors.New("bad run.toml")
	repo := worktree.Repository{
		MainPath: "/repo/main",
		Config:   project.Config{Env: project.Env{Files: []project.EnvFile{{Target: ".env"}}}, OnCreate: []project.Hook{{Cmd: "make"}}},
		Global:   project.Global{Proxy: project.Proxy{Port: 8080}},
		Run:      project.Run{Isolation: project.Verbatim},
		RunErr:   runErr,
	}

	assert.Equal(t, provision.Project{
		MainPath: "/repo/main",
		StateDir: "/state",
		EnvFiles: []project.EnvFile{{Target: ".env"}},
		OnCreate: []project.Hook{{Cmd: "make"}},
		Run:      project.Run{Isolation: project.Verbatim},
		RunErr:   runErr,
		Global:   project.Global{Proxy: project.Proxy{Port: 8080}},
	}, provision.ProjectOf(repo, "/state"))
}

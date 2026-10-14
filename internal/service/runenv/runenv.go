// Package runenv resolves a worktree's run environment from the run.toml and the identity it is handed.
package runenv

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	envcase "github.com/LucasPcq/wtm/internal/rules/runenv"
	"github.com/LucasPcq/wtm/internal/store"
)

// Identity is the worktree as its caller read it: the number is settled
// before anything here resolves, since nothing here allocates.
type Identity struct {
	Branch    string
	Ordinal   int
	IsMain    bool
	Isolation domain.Isolation
}

type Params struct {
	// ProjectDir is the main checkout: the project's name, and where its own
	// .env files are.
	ProjectDir string
	Run        domain.RunConfig
	Identity   Identity
	// Files reads the main checkout's own .env files.
	Files store.EnvFiles
}

// JobEnv is what a worktree's jobs and hooks learn about it: its identity, its
// offset and its declared ports.
func JobEnv(params Params) (map[string]string, error) {
	composeProject, err := jobComposeProject(params)
	if err != nil {
		return nil, err
	}
	identity := params.Identity
	env := rules.WorktreeJobEnv(rules.WorktreeJobEnvParams{
		Branch:          identity.Branch,
		Project:         filepath.Base(params.ProjectDir),
		Ordinal:         identity.Ordinal,
		PortOffsetBlock: rules.EffectivePortOffsetBlock(params.Run),
		ComposeProject:  composeProject,
		Isolation:       identity.Isolation,
	})
	offset, err := offsetOf(env)
	if err != nil {
		return nil, err
	}
	return rules.WithPortEnv(env, rules.LifecyclePorts(rules.LifecyclePortsParams{Config: params.Run, PortOffset: offset})), nil
}

// offsetOf reads the offset back from the environment rather than recomputing
// it, so a hook, a job and a .env of the same worktree never disagree on it.
func offsetOf(env map[string]string) (int, error) {
	offset, err := strconv.Atoi(env[domain.EnvPortOffset])
	if err != nil {
		return 0, fmt.Errorf("resolve port offset: %w", err)
	}
	return offset, nil
}

// jobComposeProject is empty for wtm to derive it from the branch; only the
// main checkout's is set on purpose, read from its own .env.
func jobComposeProject(params Params) (string, error) {
	if _, main := caseOf(params.Identity).(envcase.Main); !main {
		return "", nil
	}
	return mainComposeProject(params)
}

// ComposeProject is the COMPOSE_PROJECT_NAME written into the worktree's .env:
// derived from the branch, never from the variable this process carries, and
// for the main checkout the name its jobs run under, which never follows its
// branch. A verbatim worktree gets none.
func ComposeProject(params Params) (string, error) {
	switch caseOf(params.Identity).(type) {
	case envcase.Verbatim:
		return "", nil
	case envcase.Main:
		return mainComposeProject(params)
	case envcase.Linked:
	}
	return rules.ComposeProjectName(rules.ComposeProjectNameParams{
		Project:  filepath.Base(params.ProjectDir),
		Worktree: rules.WorktreeSlug(params.Identity.Branch),
	}), nil
}

func caseOf(identity Identity) envcase.Case {
	return envcase.CaseOf(envcase.CaseParams{Isolation: identity.Isolation, IsMain: identity.IsMain})
}

func mainComposeProject(params Params) (string, error) {
	files, err := composeEnvFiles(params)
	if err != nil {
		return "", err
	}
	return rules.MainComposeProjectName(rules.MainComposeProjectNameParams{
		Project:  filepath.Base(params.ProjectDir),
		EnvFiles: files,
	}), nil
}

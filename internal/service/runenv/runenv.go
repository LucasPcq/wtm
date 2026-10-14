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

type Identity struct {
	Branch    string
	Ordinal   int
	IsMain    bool
	Isolation domain.Isolation
}

type Params struct {
	// ProjectDir is the main checkout: the project's name, and where its own .env files are.
	ProjectDir string
	Run        domain.RunConfig
	Identity   Identity
	Files      store.EnvFiles
}

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

func offsetOf(env map[string]string) (int, error) {
	// Read back rather than recomputed, so a hook, a job and a .env of one worktree never disagree on it.
	offset, err := strconv.Atoi(env[domain.EnvPortOffset])
	if err != nil {
		return 0, fmt.Errorf("resolve port offset: %w", err)
	}
	return offset, nil
}

func jobComposeProject(params Params) (string, error) {
	if _, main := caseOf(params.Identity).(envcase.Main); !main {
		// Empty for wtm to derive it from the branch; only the main checkout's is set, read from its own .env.
		return "", nil
	}
	return mainComposeProject(params)
}

// ComposeProject never reads the variable this process carries; the main checkout's is the name its jobs run under, which never follows its branch.
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

// Package runenv resolves a worktree's run environment from the run.toml and the identity it is handed.
package runenv

import (
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	envcase "github.com/LucasPcq/wtm/internal/rules/runenv"
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
}

// JobEnv is what a worktree's jobs and hooks learn about it: its identity, its
// offset and its declared ports.
func JobEnv(params Params) map[string]string {
	identity := params.Identity
	env := rules.WorktreeJobEnv(rules.WorktreeJobEnvParams{
		Branch:          identity.Branch,
		Project:         filepath.Base(params.ProjectDir),
		Ordinal:         identity.Ordinal,
		PortOffsetBlock: rules.EffectivePortOffsetBlock(params.Run),
		ComposeProject:  jobComposeProject(params),
		Isolation:       identity.Isolation,
	})
	// The offset is read back from the environment just resolved rather than
	// recomputed, so a hook and a job of the same worktree never disagree on it.
	offset, err := strconv.Atoi(env[domain.EnvPortOffset])
	if err != nil {
		offset = 0
	}
	return rules.WithPortEnv(env, rules.LifecyclePorts(rules.LifecyclePortsParams{Config: params.Run, PortOffset: offset}))
}

// jobComposeProject is empty for wtm to derive it from the branch; only the
// main checkout's is set on purpose, read from its own .env.
func jobComposeProject(params Params) string {
	if _, main := caseOf(params.Identity).(envcase.Main); !main {
		return ""
	}
	return mainComposeProject(params)
}

// ComposeProject is the COMPOSE_PROJECT_NAME written into the worktree's .env:
// derived from the branch, never from the variable this process carries, and
// for the main checkout the name its jobs run under, which never follows its
// branch. A verbatim worktree gets none.
func ComposeProject(params Params) string {
	switch caseOf(params.Identity).(type) {
	case envcase.Verbatim:
		return ""
	case envcase.Main:
		return mainComposeProject(params)
	case envcase.Linked:
	}
	return rules.ComposeProjectName(rules.ComposeProjectNameParams{
		Project:  filepath.Base(params.ProjectDir),
		Worktree: rules.WorktreeSlug(params.Identity.Branch),
	})
}

func caseOf(identity Identity) envcase.Case {
	return envcase.CaseOf(envcase.CaseParams{Isolation: identity.Isolation, IsMain: identity.IsMain})
}

func mainComposeProject(params Params) string {
	return rules.MainComposeProjectName(rules.MainComposeProjectNameParams{
		Project:  filepath.Base(params.ProjectDir),
		EnvFiles: composeEnvFiles(composeEnvFilesParams{Dir: params.ProjectDir, Config: params.Run}),
	})
}

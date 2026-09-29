package rules

import (
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
)

type WorktreeJobEnvParams struct {
	Branch string
	// Project names the repository, so a compose project is unique on the
	// machine and not merely within this repo. Two clones both sitting on
	// `main` would otherwise share one stack.
	Project string
	Ordinal int
	// PortOffsetBlock spaces two worktrees' ports apart. Zero falls back to the
	// default block rather than collapsing every worktree onto offset 0.
	PortOffsetBlock int
	// ComposeProject is the COMPOSE_PROJECT_NAME the caller's own environment
	// already defines. Non-empty wins: a project name set on purpose is an
	// answer, not a value to overwrite.
	ComposeProject string
	// Isolation verbatim runs the worktree as its copied .env describes it: on
	// the base ports, and under whatever compose project that file or the
	// directory names — never one wtm made up.
	Isolation domain.Isolation
}

// WorktreeJobEnv is what every job and lifecycle hook learns about the worktree
// it runs in. It is resolved client-side, by the only process that can ask git
// which worktree this is.
func WorktreeJobEnv(params WorktreeJobEnvParams) map[string]string {
	block := params.PortOffsetBlock
	if block <= 0 {
		block = domain.PortOffsetBlock
	}

	slug := WorktreeSlug(params.Branch)
	isolation := EffectiveIsolation(params.Isolation)
	offset := params.Ordinal * block
	composeProject := params.ComposeProject
	if composeProject == "" && isolation == domain.IsolationIsolated {
		composeProject = ComposeProjectName(ComposeProjectNameParams{Project: params.Project, Worktree: slug})
	}
	if isolation == domain.IsolationVerbatim {
		offset = 0
	}

	env := map[string]string{
		domain.EnvWorktree:   slug,
		domain.EnvBranch:     params.Branch,
		domain.EnvOrdinal:    strconv.Itoa(params.Ordinal),
		domain.EnvPortOffset: strconv.Itoa(offset),
		domain.EnvProject:    HostLabel(params.Project),
		domain.EnvIsolation:  string(isolation),
	}
	if composeProject != "" {
		env[domain.EnvComposeProjectName] = composeProject
	}
	return env
}

type ComposeProjectNameParams struct {
	Project  string
	Worktree string
}

// ComposeProjectName qualifies a worktree's slug with its repository's. The
// Docker daemon is machine-wide, so a name that is only unique within one
// repository is not unique enough.
func ComposeProjectName(params ComposeProjectNameParams) string {
	if params.Project == "" {
		return params.Worktree
	}
	return WorktreeSlug(params.Project) + "-" + params.Worktree
}

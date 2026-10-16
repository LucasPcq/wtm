package create

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
	rulebranch "github.com/LucasPcq/wtm/internal/rules/branch"
	"github.com/LucasPcq/wtm/internal/rules/worktree"
	"github.com/LucasPcq/wtm/internal/service/identity"
)

type Facts struct {
	MainPath   string                     `json:"main_path"`
	Identity   identity.Snapshot          `json:"identity"`
	Branches   git.Branches               `json:"branches"`
	Divergence map[string]git.AheadBehind `json:"divergence,omitempty"`
	Taken      map[string]git.Location    `json:"taken,omitempty"`
	Config     domain.ProjectConfig       `json:"config"`
	Global     domain.GlobalConfig        `json:"global"`
	Run        domain.RunConfig           `json:"run"`
	RunProblem string                     `json:"run_problem,omitempty"`
}

func observer(ports Ports) func(context.Context) (Facts, error) {
	return func(ctx context.Context) (Facts, error) {
		cfg, err := config.Load(config.LoadParams{StateDir: ports.StateDir})
		if errors.Is(err, domain.ErrConfigNotFound) {
			return Facts{}, kernel.Precondition(kernel.Problem{Code: CodeConfigNotFound, Cause: err})
		}
		if err != nil {
			return Facts{}, err
		}
		snapshot, err := identity.Read(ctx, identity.ReadParams{Git: ports.Git, Meta: ports.Meta})
		if err != nil {
			return Facts{}, err
		}
		branches, err := ports.Git.Branches(ctx)
		if err != nil {
			return Facts{}, err
		}
		divergence, err := divergenceOf(ctx, divergenceParams{Git: ports.Git, Branches: branches})
		if err != nil {
			return Facts{}, err
		}
		taken, err := takenUnder(ctx, takenParams{Git: ports.Git, Base: filepath.Join(ports.Dir, cfg.Project.Worktrees.BasePath)})
		if err != nil {
			return Facts{}, err
		}
		run, runErr := config.LoadRun(ports.StateDir)
		return Facts{
			MainPath:   ports.Dir,
			Identity:   snapshot,
			Branches:   branches,
			Divergence: divergence,
			Taken:      taken,
			Config:     cfg.Project,
			Global:     cfg.Global,
			Run:        run,
			RunProblem: problemOf(runErr),
		}, nil
	}
}

func problemOf(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type divergenceParams struct {
	Git      git.Reader
	Branches git.Branches
}

func divergenceOf(ctx context.Context, params divergenceParams) (map[string]git.AheadBehind, error) {
	remote := tips(params.Branches.Remote)
	found := map[string]git.AheadBehind{}
	for _, local := range params.Branches.Local {
		tip, tracked := remote[local.Name]
		if !tracked || tip == local.Tip {
			continue
		}
		counts, err := params.Git.AheadBehind(ctx, git.AheadBehindParams{Of: git.LocalBranch{Name: local.Name}, Against: git.RemoteBranch{Name: local.Name}})
		if err != nil {
			return nil, err
		}
		found[local.Name] = counts
	}
	return found, nil
}

func tips(branches []git.Branch) map[string]string {
	byName := make(map[string]string, len(branches))
	for _, branch := range branches {
		byName[branch.Name] = branch.Tip
	}
	return byName
}

type takenParams struct {
	Git  git.Reader
	Base string
}

func takenUnder(ctx context.Context, params takenParams) (map[string]git.Location, error) {
	entries, err := os.ReadDir(params.Base)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	taken := make(map[string]git.Location, len(entries))
	for _, entry := range entries {
		location, err := params.Git.Locate(ctx, filepath.Join(params.Base, entry.Name()))
		if err != nil {
			return nil, err
		}
		taken[entry.Name()] = location
	}
	return taken, nil
}

func forget(f Facts) Facts {
	f.Config.Wizard.Remembered = nil
	return f
}

func (f Facts) remembered(id string) (string, bool) {
	value, found := f.Config.Wizard.Remembered[id]
	return value, found && value != ""
}

func (f Facts) local(branch string) bool {
	return slices.ContainsFunc(f.Branches.Local, func(b git.Branch) bool { return b.Name == branch })
}

func (f Facts) remote(branch string) bool {
	return slices.ContainsFunc(f.Branches.Remote, func(b git.Branch) bool { return b.Name == branch })
}

func (f Facts) candidate(name string) bool {
	if f.local(name) {
		return true
	}
	remote, isRemote := cutRemote(name)
	return isRemote && f.remote(remote)
}

func cutRemote(name string) (string, bool) {
	return strings.CutPrefix(name, domain.RemoteBranchPrefix)
}

func (f Facts) target(branch string) domain.BranchTarget {
	if !f.local(branch) {
		return domain.BranchTarget{State: domain.BranchTargetNew, Branch: branch, Upstream: f.upstream(branch)}
	}
	state, counts := f.divergence(branch)
	at := f.Identity.CheckedOutAt(branch)
	return domain.BranchTarget{State: localState(at), Branch: branch, WorktreePath: at, Origin: state, AheadBehind: counts}
}

func localState(checkedOutAt string) domain.BranchTargetState {
	if checkedOutAt != "" {
		return domain.BranchTargetCheckedOut
	}
	return domain.BranchTargetExisting
}

func (f Facts) upstream(branch string) string {
	if !f.remote(branch) {
		return ""
	}
	return domain.RemoteBranchPrefix + branch
}

func (f Facts) divergence(branch string) (domain.DivergenceState, domain.AheadBehind) {
	if !f.remote(branch) {
		return domain.DivergenceUnknown, domain.AheadBehind{}
	}
	counts := f.Divergence[branch]
	return rulebranch.Divergence(counts.Ahead, counts.Behind), domain.AheadBehind{Ahead: counts.Ahead, Behind: counts.Behind}
}

func (f Facts) pathOf(branch string) string {
	return filepath.Join(f.MainPath, f.Config.Worktrees.BasePath, rules.SanitizeBranchName(branch))
}

type creationParams struct {
	Branch      string
	IfNotExists bool
}

func (f Facts) creation(params creationParams) worktree.Creation {
	path := f.pathOf(params.Branch)
	location, taken := f.Taken[filepath.Base(path)]
	occupant := ""
	if taken {
		occupant = f.Identity.Occupant(location)
	}
	return worktree.CreationOf(worktree.CreationParams{
		Branch:       params.Branch,
		Path:         path,
		CheckedOutAt: f.Identity.CheckedOutAt(params.Branch),
		PathTaken:    taken,
		Occupant:     occupant,
		IfNotExists:  params.IfNotExists,
	})
}

func (f Facts) derivedNames() bool {
	return f.RunProblem != "" || rules.IsRunInitialized(f.Run)
}

func (f Facts) isolation() domain.Isolation {
	return rules.EffectiveIsolation(f.Run.Isolation)
}

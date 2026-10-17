package create

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"

	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

const remotePrefix = "origin/"

type Facts struct {
	MainPath   string
	Config     project.Config
	Global     project.Global
	Run        project.Run
	RunErr     error
	Worktrees  []worktree.Worktree
	Branches   worktree.Branches
	Divergence map[string]worktree.AheadBehind
	Folders    map[string]worktree.Location
}

func (d Deps) Observe(ctx context.Context, branches []string) (Facts, error) {
	config, global, err := d.LoadConfig()
	if errors.Is(err, project.ErrConfigNotFound) {
		return Facts{}, &wtm.Error{Kind: wtm.KindPrecondition, Code: CodeConfigNotFound, Message: "no wtm config found — run `wtm init` first", Cause: err}
	}
	if err != nil {
		return Facts{}, err
	}
	worktrees, err := d.Git.Worktrees(ctx)
	if err != nil {
		return Facts{}, err
	}
	refs, err := d.Git.Branches(ctx)
	if err != nil {
		return Facts{}, err
	}
	divergence, err := d.divergence(ctx, refs)
	if err != nil {
		return Facts{}, err
	}
	facts := Facts{MainPath: d.MainPath, Config: config, Global: global, Worktrees: worktrees, Branches: refs, Divergence: divergence}
	folders, err := d.locate(ctx, facts.pathsOf(branches))
	if err != nil {
		return Facts{}, err
	}
	facts.Folders = folders
	facts.Run, facts.RunErr = d.LoadRun()
	return facts, nil
}

func (d Deps) divergence(ctx context.Context, refs worktree.Branches) (map[string]worktree.AheadBehind, error) {
	found := map[string]worktree.AheadBehind{}
	for _, local := range refs.Local {
		at := slices.IndexFunc(refs.Remote, func(remote worktree.Branch) bool { return remote.Name == local.Name })
		if at < 0 || refs.Remote[at].Tip == local.Tip {
			continue
		}
		counts, err := d.Git.AheadBehind(ctx, local.Name)
		if err != nil {
			return nil, err
		}
		found[local.Name] = counts
	}
	return found, nil
}

func (d Deps) locate(ctx context.Context, paths []string) (map[string]worktree.Location, error) {
	folders := make(map[string]worktree.Location, len(paths))
	for _, path := range paths {
		location, err := d.Git.Locate(ctx, path)
		if err != nil {
			return nil, err
		}
		folders[path] = location
	}
	return folders, nil
}

func (f Facts) pathsOf(branches []string) []string {
	var paths []string
	for _, name := range branches {
		if !worktree.BlankBranch(name) {
			paths = append(paths, f.pathOf(name))
		}
	}
	return paths
}

func (f Facts) pathOf(name string) string {
	return filepath.Join(f.MainPath, f.Config.Worktrees.BasePath, worktree.PathSegment(name))
}

func (f Facts) local(name string) bool {
	return slices.ContainsFunc(f.Branches.Local, func(b worktree.Branch) bool { return b.Name == name })
}

func (f Facts) remote(name string) bool {
	return slices.ContainsFunc(f.Branches.Remote, func(b worktree.Branch) bool { return b.Name == name })
}

func (f Facts) candidate(name string) bool {
	remote, onOrigin := cutRemote(name)
	return f.local(name) || onOrigin && f.remote(remote)
}

func (f Facts) checkedOutAt(name string) string {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Branch() == name })
	if at < 0 {
		return ""
	}
	return f.Worktrees[at].Path
}

func (f Facts) occupant(location worktree.Location) string {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Path == location.Path })
	if at < 0 {
		return ""
	}
	return f.Worktrees[at].Branch()
}

func (f Facts) standing(name string) worktree.Divergence {
	if !f.remote(name) {
		return worktree.NoOrigin
	}
	return f.Divergence[name].Divergence()
}

func (f Facts) namesWorktrees() bool {
	return f.RunErr != nil || len(f.Run.Jobs) > 0 || len(f.Run.Profiles) > 0
}

func (f Facts) live() []string {
	names := make([]string, 0, len(f.Worktrees))
	for _, w := range f.Worktrees {
		names = append(names, w.Branch())
	}
	return names
}

func cutRemote(name string) (string, bool) {
	return strings.CutPrefix(name, remotePrefix)
}

func aheadAndBehind(counts worktree.AheadBehind) string {
	return strconv.Itoa(counts.Ahead) + " ahead, " + strconv.Itoa(counts.Behind) + " behind"
}

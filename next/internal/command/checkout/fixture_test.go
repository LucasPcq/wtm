package checkout_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// The pull requests of the world, by what their branch is in the repository.
const (
	prRemote   = 1  // feat/remote: only origin has it
	prLocal    = 2  // feat/local: local, as origin has it
	prBehind   = 3  // feat/behind: local, one commit behind origin's
	prDiverged = 4  // feat/diverged: local, a commit of its own and one of origin's
	prFork     = 5  // a fork's branch named main
	prHeld     = 6  // feat/held: checked out in a linked worktree
	prNowhere  = 7  // feat/gone: neither local nor on origin
	prOnBase   = 8  // feat/remote, merging into release, a branch only origin has
	prFresh    = 9  // feat/fresh: local, origin moved on since the last fetch
	prMerged   = 10 // feat/remote, merged
	prDeleted  = 11 // feat/deleted: fetched, then deleted on origin, its remote-tracking branch left
	prStale    = 12 // feat/stale: local, fetched behind origin, then deleted on origin
	prNone     = 99 // none
)

const (
	mainEnv     = "PORT=3000\n"
	jobsToml    = "[[job]]\nname = \"web\"\nkind = \"service\"\ncmd = \"npm start\"\nports = { PORT = 3000 }\n\n[[env_port]]\nfile = \".env\"\nkey = \"PORT\"\njob = \"web\"\nport = \"PORT\"\n"
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n\n[env]\nstrategy = \"main\"\n\n[[env.file]]\ntarget = \".env\"\n\n[hooks]\non_create = []\n"
)

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func pullRequests() []worktree.PullRequest {
	open := func(number int, branch, base string) worktree.PullRequest {
		return worktree.PullRequest{Number: number, URL: fmt.Sprintf("https://github.com/acme/acme/pull/%d", number), Branch: branch, State: worktree.PullRequestOpen, Title: "PR " + branch, Base: base, Author: "alice"}
	}
	fork := open(prFork, "main", "main")
	fork.Fork = true
	merged := open(prMerged, "feat/remote", "main")
	merged.State = worktree.PullRequestMerged
	return []worktree.PullRequest{
		open(prRemote, "feat/remote", "main"),
		open(prLocal, "feat/local", "main"),
		open(prBehind, "feat/behind", "main"),
		open(prDiverged, "feat/diverged", "main"),
		fork,
		open(prHeld, "feat/held", "main"),
		open(prNowhere, "feat/gone", "main"),
		open(prOnBase, "feat/remote", "release"),
		open(prFresh, "feat/fresh", "main"),
		merged,
		open(prDeleted, "feat/deleted", "main"),
		open(prStale, "feat/stale", "main"),
	}
}

type world struct {
	root     string
	main     string
	stateDir string
	trees    string
	meta     store.Meta
	asked    *[]string // the calls made to GitHub
	usecase  checkout.UseCase
}

func newWorld(t *testing.T) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.Build(t, gittest.FakeParams{
		Main:      main,
		Branch:    "main",
		Local:     []string{"feat/local", "feat/behind", "feat/diverged", "feat/fresh", "feat/stale"},
		Remote:    []string{"main", "feat/remote", "feat/local", "feat/behind", "feat/diverged", "feat/held", "release", "feat/deleted", "feat/stale"},
		Deleted:   []string{"feat/deleted", "feat/stale"},
		Worktrees: []worktree.Worktree{{Path: filepath.Join(root, "wt-held"), State: worktree.OnBranch{Branch: "feat/held"}}},
		Commits: map[worktree.Ref][]string{
			worktree.RemoteBranch{Name: "feat/remote"}:   {"remote work"},
			worktree.LocalBranch{Name: "feat/behind"}:    {"behind work"},
			worktree.RemoteBranch{Name: "feat/behind"}:   {"behind work", "pushed since"},
			worktree.LocalBranch{Name: "feat/diverged"}:  {"diverged work", "local only"},
			worktree.RemoteBranch{Name: "feat/diverged"}: {"diverged work", "origin only"},
			worktree.LocalBranch{Name: "feat/fresh"}:     {"fresh work"},
			worktree.LocalBranch{Name: "feat/stale"}:     {"stale work"},
			worktree.RemoteBranch{Name: "feat/stale"}:    {"stale work", "merged since"},
		},
		Origin: map[string][]string{"feat/fresh": {"fresh work", "pushed after the fetch"}},
	})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), projectToml)
	writeFile(t, filepath.Join(main, ".env"), mainEnv)
	meta := store.Meta{StateDir: stateDir}
	projectConfig := config.Project{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")}
	asked := &[]string{}
	ids := 0
	return world{
		root:     root,
		main:     main,
		stateDir: stateDir,
		trees:    filepath.Join(root, ".trees"),
		meta:     meta,
		asked:    asked,
		usecase: checkout.UseCase{
			Git:          repo,
			PullRequest:  github{prs: pullRequests(), asked: asked}.pullRequest,
			PullRequests: github{prs: pullRequests(), asked: asked}.open,
			Repository:   repository.Service{Config: projectConfig, Git: repo, MainPath: main},
			Operations: operation.Service{
				Locker:    locks{oplock.Locks{StateDir: stateDir}},
				Publisher: &operationtest.Recorder{},
				NewOpID: func() string {
					ids++
					return fmt.Sprintf("op-%d", ids)
				},
				Now: func() time.Time { return createdAt },
			},
			Provision: provision.Service{
				Git:         repo,
				Meta:        meta,
				EnvFiles:    store.EnvFiles{},
				StateDir:    stateDir,
				Answers:     projectConfig,
				RunHooks:    shell.RunHooks,
				PublicPort:  func(context.Context, int) int { return 0 },
				Shield:      shell.Shield,
				Now:         func() time.Time { return createdAt },
				LeftBehind:  func(branch string) domain.FollowUp { return domain.FollowUp{Command: "clean", Code: "clean"} },
				Settle:      func(branch string) domain.FollowUp { return domain.FollowUp{Command: "env", Code: "env"} },
				ProxyStatus: func() domain.FollowUp { return domain.FollowUp{Command: "run proxy status", Code: "proxy"} },
			},
		},
	}
}

func (w world) path(name string) string {
	return filepath.Join(w.trees, worktree.PathSegment(name))
}

// github answers the pull requests of prs as the adapter does, recording each call in asked; a non-nil err fails every call.
type github struct {
	prs   []worktree.PullRequest
	asked *[]string
	err   error
}

func (g github) pullRequest(_ context.Context, number int) (worktree.PullRequest, error) {
	*g.asked = append(*g.asked, fmt.Sprintf("view %d", number))
	if g.err != nil {
		return worktree.PullRequest{}, g.err
	}
	for _, pr := range g.prs {
		if pr.Number == number {
			return pr, nil
		}
	}
	return worktree.PullRequest{}, fmt.Errorf("pull request #%d: %w", number, worktree.ErrPullRequestNotFound)
}

func (g github) open(_ context.Context, filter worktree.PullRequestFilter) ([]worktree.PullRequest, error) {
	*g.asked = append(*g.asked, "list "+string(filter))
	if g.err != nil {
		return nil, g.err
	}
	var open []worktree.PullRequest
	for _, pr := range g.prs {
		if pr.State == worktree.PullRequestOpen {
			open = append(open, pr)
		}
	}
	return open, nil
}

type locks struct {
	oplock.Locks
}

func (l locks) Take(op domain.Operation) (operation.Hold, error) {
	return l.Locks.Take(op)
}

// unfetched is the repository with a fetch that fails, as when origin cannot be reached.
type unfetched struct {
	git.Repo
}

func (unfetched) FetchRef(context.Context, string) error {
	return fmt.Errorf("fatal: unable to access origin")
}

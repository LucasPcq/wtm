package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/github"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/portprobe"
	"github.com/LucasPcq/wtm/next/internal/adapter/proxyport"
	reposjson "github.com/LucasPcq/wtm/next/internal/adapter/registry"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/command/clean"
	cleancli "github.com/LucasPcq/wtm/next/internal/command/clean/cli"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/command/exec"
	execcli "github.com/LucasPcq/wtm/next/internal/command/exec/cli"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	fastforwardcli "github.com/LucasPcq/wtm/next/internal/command/fastforward/cli"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/command/prune"
	prunecli "github.com/LucasPcq/wtm/next/internal/command/prune/cli"
	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	reparentcli "github.com/LucasPcq/wtm/next/internal/command/reparent/cli"
	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/command/runup"
	runupcli "github.com/LucasPcq/wtm/next/internal/command/runup/cli"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	synccommand "github.com/LucasPcq/wtm/next/internal/command/sync"
	synccli "github.com/LucasPcq/wtm/next/internal/command/sync/cli"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/registry"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
)

const opIDBytes = 8

// wireParams is where the repository a command runs on keeps its files; it also publishes the events of the operations on it.
type wireParams struct {
	MainPath  string // the main worktree
	StateDir  string // wtm's state: meta records, locks, remembered answers
	CommonDir string // git's common dir, shared by every worktree
}

// An app is every use case wired on one repository.
type app struct {
	Create      create.UseCase
	Extract     extract.UseCase
	Checkout    checkout.UseCase
	Clean       clean.UseCase
	Prune       prune.UseCase
	FastForward fastforward.UseCase
	Sync        synccommand.UseCase
	Exec        exec.UseCase
	Reparent    reparent.UseCase
	List        list.UseCase
	Resolve     resolve.UseCase
	Status      status.UseCase
	Tree        tree.UseCase
	RunUp       runup.UseCase
}

// wire builds each adapter of the repository params locates once and injects it into every use case.
func wire(params wireParams) app {
	repo := git.Repo{Dir: params.MainPath}
	projectConfig := config.Project{StateDir: params.StateDir, GlobalPath: config.GlobalPath()}

	facts := params.facts()

	operations := operation.Service{
		Locker:    locker{oplock.Locks{StateDir: params.StateDir}},
		Publisher: params,
		NewOpID:   newOpID,
		Now:       time.Now,
	}

	meta := store.Meta{StateDir: params.StateDir}
	runs := runDaemon()
	jobs := runs.Jobs
	gh := github.GitHub{Dir: params.MainPath}

	provisions := provision.Service{
		Git:         repo,
		Meta:        meta,
		EnvFiles:    store.EnvFiles{},
		Answers:     projectConfig,
		StateDir:    params.StateDir,
		RunHooks:    shell.RunHooks,
		PublicPort:  proxyport.Port{Plist: proxyport.PlistPath()}.Public,
		Shield:      shell.Shield,
		Now:         time.Now,
		LeftBehind:  leftBehind,
		Settle:      settle,
		ProxyStatus: proxyStatus,
	}

	teardowns := teardown.Service{
		Git:      repo,
		Meta:     meta,
		EnvFiles: store.EnvFiles{},
		StateDir: params.StateDir,
		Gone:     repository.GoneIn(repo),
		RunHooks: shell.RunHooks,
		Shield:   shell.Shield,
	}

	return app{
		Create: create.UseCase{
			Git:        repo,
			Meta:       meta,
			Repository: facts,
			Operations: operations,
			Provision:  provisions,
		},
		Extract: extract.UseCase{
			Git:        repo,
			Repository: facts,
			Meta:       meta,
			Shield:     shell.Shield,
			Operations: operations,
			Provision:  provisions,
		},
		Checkout: checkout.UseCase{
			Git:          refFetchedOnce{Repo: repo, fetch: askedOnce(func(ctx context.Context, name string) (struct{}, error) { return struct{}{}, repo.FetchRef(ctx, name) })},
			PullRequest:  askedOnce(gh.PullRequest),
			PullRequests: askedOnce(gh.OpenPullRequests),
			Repository:   facts,
			Operations:   operations,
			Provision:    provisions,
		},
		Clean: clean.UseCase{
			Git:          repo,
			Meta:         meta,
			Jobs:         jobs,
			PullRequests: gh.PullRequestsOf,
			Teardown:     teardowns,
			Repository:   facts,
			Operations:   operations,
		},
		Prune: prune.UseCase{
			Git:          fetchedOnce{Repo: repo, once: &sync.Once{}, err: new(error)},
			Meta:         meta,
			Jobs:         jobs,
			PullRequests: gh.PullRequestsOf,
			Teardown:     teardowns,
			Repository:   facts,
			Operations:   operations,
		},
		FastForward: fastforward.UseCase{
			Git:        repo,
			Shield:     shell.Shield,
			Repository: facts,
			Operations: operations,
			Provision:  provisions,
		},
		Sync: synccommand.UseCase{
			Git:        repo,
			Meta:       meta,
			Shield:     shell.Shield,
			Repository: facts,
			Operations: operations,
			Provision:  provisions,
		},
		Exec: exec.UseCase{
			Git:        repo,
			Meta:       meta,
			EnvFiles:   store.EnvFiles{},
			RunHooks:   shell.RunHooks,
			Syntax:     shell.Syntax,
			OpenLog:    shell.OpenRunLog,
			StateDir:   params.StateDir,
			CPUs:       runtime.NumCPU(),
			Now:        time.Now,
			Repository: facts,
			Operations: operations,
		},
		Reparent: reparent.UseCase{
			Git:        repo,
			Meta:       meta,
			Repository: facts,
			Operations: operations,
		},
		List: list.UseCase{
			Git:          repo,
			Meta:         meta,
			Jobs:         jobs,
			PullRequests: gh.PullRequestsOf,
			Repository:   facts,
		},
		Resolve: resolve.UseCase{
			Git:        repo,
			Repository: facts,
		},
		Status: status.UseCase{
			Git:          repo,
			Meta:         meta,
			EnvFiles:     store.EnvFiles{},
			Jobs:         jobs,
			PullRequests: gh.PullRequestsOf,
			Repository:   facts,
		},
		Tree: tree.UseCase{
			Git:          repo,
			Meta:         meta,
			Jobs:         jobs,
			PullRequests: gh.PullRequestsOf,
			Repository:   facts,
		},
		RunUp: runup.UseCase{
			Git:        repo,
			Meta:       meta,
			EnvFiles:   store.EnvFiles{},
			Daemon:     runs,
			Listening:  portprobe.Listening,
			StateDir:   params.StateDir,
			CommonDir:  bus.Resolved(params.CommonDir),
			Settle:     runSettle,
			Repository: facts,
			Operations: operations,
			Provision:  provisions,
		},
	}
}

// runSettle is how long run up waits, once its services are launched and no port check waits, for one that fails at once to end: a process that cannot bind its port exits within instants of its spawn, after the start that spawned it returned.
const runSettle = time.Second

// fetchedOnce is the repository as prune reads it, origin fetched once however often one opened command plans: the wizard plans at each of its steps, and a request naming no worktree plans again under the locks it finds.
type fetchedOnce struct {
	git.Repo
	once *sync.Once
	err  *error
}

// RefreshOrigin refreshes origin's branches the first time, and says again how that went every time after.
func (f fetchedOnce) RefreshOrigin(ctx context.Context, branches []string) error {
	f.once.Do(func() { *f.err = f.Repo.RefreshOrigin(ctx, branches) })
	return *f.err
}

// askedOnce answers each key as ask does, asking it once per opened command, as fetchedOnce does for prune: checkout's wizard asks GitHub and fetches the pull request's branch at each of its steps, and a run plans again once it holds the branch's lock. An interrupted ask is not kept.
func askedOnce[K comparable, V any](ask func(ctx context.Context, key K) (V, error)) func(ctx context.Context, key K) (V, error) {
	type answer struct {
		value V
		err   error
	}
	var mu sync.Mutex
	asked := map[K]answer{}
	return func(ctx context.Context, key K) (V, error) {
		mu.Lock()
		defer mu.Unlock()
		if kept, ok := asked[key]; ok {
			return kept.value, kept.err
		}
		value, err := ask(ctx, key)
		if ctx.Err() == nil {
			asked[key] = answer{value, err}
		}
		return value, err
	}
}

// refFetchedOnce is the repository as checkout reads it, each branch fetched once per opened command.
type refFetchedOnce struct {
	git.Repo
	fetch func(ctx context.Context, name string) (struct{}, error)
}

// FetchRef fetches the branch name from origin the first time, and says again how that went every time after.
func (r refFetchedOnce) FetchRef(ctx context.Context, name string) error {
	_, err := r.fetch(ctx, name)
	return err
}

// facts reads the repository params locates, as every use case decides from it.
func (p wireParams) facts() repository.Service {
	return repository.Service{Config: config.Project{StateDir: p.StateDir, GlobalPath: config.GlobalPath()}, Git: git.Repo{Dir: p.MainPath}, MainPath: p.MainPath, Init: initialize(), Restart: daemonRestart()}
}

// repo is how the event bus and the registry name the repository params locates.
func (p wireParams) repo() domain.Repo {
	return domain.Repo{Root: p.MainPath, CommonDir: bus.Resolved(p.CommonDir)}
}

// The answers the legacy commands a follow-up names take as flags of their own name: a switch alone when said true (run start's --detach), the others followed by their value (env's --from and --isolation, run start's --job, reparent's --to).
var (
	switches = []string{"detach"}
	valued   = []string{"from", domain.PathIsolation, "job", "to"}
)

// leftBehind is the repair of a branch an undo left behind, and of its worktree: clean, forced.
func leftBehind(branch string) domain.FollowUp {
	return domain.FollowUp{Command: clean.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{branch}}, {Path: clean.PathForce, Values: []string{"true"}}}}
}

// initialize sets wtm up in a repository that has no config, in the legacy wtm until init migrates.
func initialize() *domain.FollowUp {
	return &domain.FollowUp{Command: domain.CommandInit}
}

// daemonRestart is the follow-up that replaces a run daemon of another build by one of this build: run daemon restart.
func daemonRestart() *domain.FollowUp {
	return &domain.FollowUp{Command: commandRestartDaemon}
}

// settle settles a worktree's env ports, in the legacy wtm until env migrates.
func settle(branch string) domain.FollowUp {
	return domain.FollowUp{Command: domain.CommandEnv, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{branch}}}}
}

// proxyStatus reports what serves named addresses on this machine, in the legacy wtm until run proxy migrates.
func proxyStatus() domain.FollowUp {
	return domain.FollowUp{Command: "run proxy status"}
}

// commandLine spells a follow-up as the command line runs it: one naming a migrated command another command gives (clean, prune, fast-forward, sync, exec, reparent) from that command's schema; go as the shell function over resolve, which takes resolve's answers, but a worktree on no branch entered with cd; a command not yet migrated as the legacy wtm runs it; a pull request opened by gh.
func commandLine(followUp domain.FollowUp) []string {
	switch followUp.Command {
	case clean.Name:
		return cleancli.Line(followUp)
	case prune.Name:
		return prunecli.Line(followUp)
	case fastforward.Name:
		return fastforwardcli.Line(followUp)
	case synccommand.Name:
		return synccli.Line(followUp)
	case exec.Name:
		return execcli.Line(followUp)
	case reparent.Name:
		return reparentcli.Line(followUp)
	case runup.Name:
		return runupcli.Line(followUp)
	case commandRestartDaemon:
		return []string{cli.Program, "run", "daemon", "restart"}
	}

	argv, _ := spelled(followUp)
	return argv
}

// spelled is followUp's command line and what of it could not be spelled, which a test requires nothing of: an answer's path, or url for a follow-up with neither a command nor a pull request to open. The worktree is positional, the other answers and a dry run are flags.
func spelled(followUp domain.FollowUp) (argv, unspelled []string) {
	if followUp.Command == "" {
		if argv = pullRequestLine(followUp); argv == nil {
			return nil, []string{"url"}
		}
		return argv, nil
	}

	var worktree, flags []string
	for _, answer := range followUp.Answers {
		switch {
		case answer.Path == domain.PathWorktree:
			worktree = answer.Values
		case slices.Contains(switches, answer.Path) && slices.Equal(answer.Values, []string{"true"}):
			flags = append(flags, "--"+answer.Path)
		case slices.Contains(valued, answer.Path) && len(answer.Values) == 1:
			flags = append(flags, "--"+answer.Path, answer.Values[0])
		default:
			unspelled = append(unspelled, answer.Path)
		}
	}
	if followUp.DryRun {
		flags = append(flags, "--dry-run")
	}
	// a branch never starts with a slash: an absolute worktree is one on no branch, entered without the shell function
	if followUp.Command == domain.CommandGo && len(worktree) == 1 && filepath.IsAbs(worktree[0]) {
		return []string{"cd", worktree[0]}, unspelled
	}

	argv = append([]string{"wtm"}, strings.Fields(followUp.Command)...)
	return append(append(argv, worktree...), flags...), unspelled
}

// pullRequestLine opens the pull request a follow-up links to: by its number, else by its URL; nothing without either.
func pullRequestLine(followUp domain.FollowUp) []string {
	if number, err := strconv.Atoi(followUp.Params[repository.ParamNumber]); err == nil {
		return github.ViewCommand(number)
	}
	if followUp.URL == "" {
		return nil
	}
	return []string{"gh", "pr", "view", followUp.URL, "--web"}
}

// A locker adapts [oplock.Locks] to the port of [operation.Service], whose Take returns an [operation.Hold] where oplock's returns its concrete [*oplock.Held].
type locker struct {
	oplock.Locks
}

// Take takes the locks op declares; it fails when another process holds one of them.
func (l locker) Take(op domain.Operation) (operation.Hold, error) {
	return l.Locks.Take(op)
}

// Open returns the report that publishes op's events on the event bus.
func (p wireParams) Open(op domain.Operation) domain.Report {
	return bus.NewPublisher(p.publisherParams(op)).Publish
}

func (p wireParams) publisherParams(op domain.Operation) bus.PublisherParams {
	return bus.PublisherParams{
		Root:          p.MainPath,
		CommonDir:     p.CommonDir,
		SocketPath:    bus.SocketPath(),
		OpID:          op.ID,
		CorrelationID: op.CorrelationID,
		Now:           time.Now,
	}
}

// newOpID returns a random hexadecimal id for a new operation.
func newOpID() string {
	raw := make([]byte, opIDBytes)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}

// ----------------------------------------------------------------------------
// The event stream

// streamParams is where the event stream finds its bus, the socket of its daemon, which Start starts when none answers, and the registry of the repositories wtm knows.
type streamParams struct {
	Socket   string
	Start    func() error
	Registry string
}

// eventStream is the events use case over the bus and the registry params name, opening each repository with openEvents and asking the run daemon at the bus's socket for the jobs it polls.
func eventStream(params streamParams) events.UseCase {
	return events.UseCase{
		Bus:      bus.Bus{SocketPath: params.Socket, Start: params.Start},
		Registry: knownRepos(params.Registry, params.Socket),
		Open:     openEvents,
		Jobs:     daemon.Daemon{SocketPath: params.Socket, Version: version}.Jobs,
		Now:      time.Now,
	}
}

// The process's own event stream: its bus at the socket every wtm-next talks to, started from this binary.
func systemStream() events.UseCase {
	return eventStream(streamParams{Socket: bus.SocketPath(), Start: startDaemon, Registry: reposjson.Path()})
}

// knownRepos is the registry of the repositories wtm knows, in the file at path, announcing on the bus at socket each one that joins or leaves it.
func knownRepos(path, socket string) registry.Service {
	return registry.Service{
		Store: reposjson.Registry{Path: path},
		Announce: func(event domain.Event, repo domain.Repo) {
			bus.NewPublisher(bus.PublisherParams{Root: repo.Root, CommonDir: repo.CommonDir, SocketPath: socket}).Publish(event)
		},
		Moved:       func() bool { return movedState() != "" },
		Initialized: func(repo domain.Repo) bool { return config.Project{StateDir: stateDir(repo.CommonDir)}.Initialized() },
	}
}

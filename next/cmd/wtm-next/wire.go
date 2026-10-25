package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/github"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/proxyport"
	reposjson "github.com/LucasPcq/wtm/next/internal/adapter/registry"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/registry"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
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
	Create  create.UseCase
	Extract extract.UseCase
	List    list.UseCase
	Status  status.UseCase
	Tree    tree.UseCase
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
	jobs := daemon.Daemon{SocketPath: bus.SocketPath()}.Jobs
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
		List: list.UseCase{
			Git:          repo,
			Meta:         meta,
			Jobs:         jobs,
			PullRequests: gh.PullRequestsOf,
			Repository:   facts,
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
	}
}

// facts reads the repository params locates, as every use case decides from it.
func (p wireParams) facts() repository.Service {
	return repository.Service{Config: config.Project{StateDir: p.StateDir, GlobalPath: config.GlobalPath()}, Git: git.Repo{Dir: p.MainPath}, MainPath: p.MainPath, Init: initialize()}
}

// repo is how the event bus and the registry name the repository params locates.
func (p wireParams) repo() domain.Repo {
	return domain.Repo{Root: p.MainPath, CommonDir: bus.Resolved(p.CommonDir)}
}

// pathForce is the answer that lifts a legacy command's safety refusals, spelled --force.
const pathForce = "force"

// The answers the legacy commands a follow-up names take as flags of their own name: a switch alone when said true (run start's --detach), the others followed by their value (env's --from and --isolation, run start's --job).
var (
	switches = []string{pathForce, "detach"}
	valued   = []string{"from", domain.PathIsolation, "job", "to"}
)

// leftBehind is the repair of a branch an undo left behind, and of its worktree: clean, forced, in the legacy wtm until it migrates.
func leftBehind(branch string) domain.FollowUp {
	return domain.FollowUp{Command: domain.CommandClean, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{branch}}, {Path: pathForce, Values: []string{"true"}}}}
}

// initialize sets wtm up in a repository that has no config, in the legacy wtm until init migrates.
func initialize() *domain.FollowUp {
	return &domain.FollowUp{Command: domain.CommandInit}
}

// settle settles a worktree's env ports, in the legacy wtm until env migrates.
func settle(branch string) domain.FollowUp {
	return domain.FollowUp{Command: domain.CommandEnv, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{branch}}}}
}

// proxyStatus reports what serves named addresses on this machine, in the legacy wtm until run proxy migrates.
func proxyStatus() domain.FollowUp {
	return domain.FollowUp{Command: "run proxy status"}
}

// commandLine spells a follow-up as the legacy wtm runs it until its command migrates, a worktree on no branch entered with cd, and a pull request opened by gh.
func commandLine(followUp domain.FollowUp) []string {
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
	// a branch never starts with a slash: an absolute worktree is one on no branch, which wtm go cannot name
	if followUp.Command == domain.CommandGo && len(worktree) == 1 && filepath.IsAbs(worktree[0]) {
		return []string{"cd", worktree[0]}, unspelled
	}

	argv = append([]string{"wtm"}, strings.Fields(followUp.Command)...)
	return append(append(argv, worktree...), flags...), unspelled
}

// pullRequestLine opens the pull request a follow-up links to: by its number, else by its URL; nothing without either.
func pullRequestLine(followUp domain.FollowUp) []string {
	if number, err := strconv.Atoi(followUp.Params[list.ParamNumber]); err == nil {
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
		Jobs:     daemon.Daemon{SocketPath: params.Socket}.Jobs,
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

// daemonCommand is the command line that runs the bus's daemon in a process of its own.
const daemonCommand = "daemon"

// daemonIdle is how long the daemon waits with no subscriber and no request before it stops.
const daemonIdle = 30 * time.Second

// errDaemonInTest refuses to start a daemon from a test: the executable is the test binary, which would run the whole suite again as "daemon".
var errDaemonInTest = errors.New("no event bus is started from a test")

// startDaemon starts this binary's daemon in a session of its own, with this process's environment but its correlation ID, which belongs to the one command that started it.
func startDaemon() error {
	if testing.Testing() {
		return errDaemonInTest
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	return shell.Detached(exe, []string{daemonCommand}, shell.Environ([]string{cli.CorrelationIDEnv}, nil))
}

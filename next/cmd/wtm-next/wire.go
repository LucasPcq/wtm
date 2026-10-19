package main

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	"github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/github"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/proxyport"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
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
}

// wire builds each adapter of the repository params locates once and injects it into every use case.
func wire(params wireParams) app {
	repo := git.Repo{Dir: params.MainPath}
	projectConfig := config.Project{StateDir: params.StateDir, GlobalPath: config.GlobalPath()}

	facts := repository.Service{Config: projectConfig, Git: repo, MainPath: params.MainPath}

	operations := operation.Service{
		Locker:    locker{oplock.Locks{StateDir: params.StateDir}},
		Publisher: params,
		NewOpID:   newOpID,
		Now:       time.Now,
	}

	meta := store.Meta{StateDir: params.StateDir}

	provisions := provision.Service{
		Git:        repo,
		Meta:       meta,
		EnvFiles:   store.EnvFiles{},
		Answers:    projectConfig,
		StateDir:   params.StateDir,
		RunHooks:   shell.RunHooks,
		PublicPort: proxyport.Port{Plist: proxyport.PlistPath()}.Public,
		Shield:     shell.Shield,
		Now:        time.Now,
		LeftBehind: leftBehind,
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
			Jobs:         daemon.Daemon{SocketPath: events.SocketPath()}.Jobs,
			PullRequests: github.GitHub{Dir: params.MainPath}.PullRequests,
			Repository:   facts,
		},
	}
}

// leftBehind is the repair of a branch an undo left behind, and of its worktree: clean, forced, in the legacy wtm until it migrates.
func leftBehind(branch string) domain.FollowUp {
	return domain.FollowUp{Command: "clean", Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{branch}}, {Path: domain.PathForce, Values: []string{"true"}}}}
}

// commandLine spells a follow-up as the legacy wtm runs it until its command migrates, a worktree on no branch entered with cd, and a pull request opened by gh; nothing for a follow-up with neither a command nor a link.
func commandLine(followUp domain.FollowUp) []string {
	argv, _ := spelled(followUp)
	return argv
}

// spelled is followUp's command line and the paths of the answers it could not spell, which a test requires none of: the worktree is positional, force and a dry run are flags.
func spelled(followUp domain.FollowUp) (argv, unspelled []string) {
	if followUp.Command == "" {
		return pullRequestLine(followUp), nil
	}

	var worktree, flags []string
	for _, answer := range followUp.Answers {
		switch {
		case answer.Path == domain.PathWorktree:
			worktree = answer.Values
		case answer.Path == domain.PathForce && slices.Equal(answer.Values, []string{"true"}):
			flags = append(flags, "--"+domain.PathForce)
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

// Open returns the report that publishes op's events on the events socket.
func (p wireParams) Open(op domain.Operation) domain.Report {
	return events.NewPublisher(p.publisherParams(op)).Publish
}

func (p wireParams) publisherParams(op domain.Operation) events.PublisherParams {
	return events.PublisherParams{
		Root:          p.MainPath,
		CommonDir:     p.CommonDir,
		SocketPath:    events.SocketPath(),
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

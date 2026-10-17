package main

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/proxyport"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

const (
	remote    = "origin"
	opIDBytes = 8
)

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
}

// wire builds each adapter of the repository params locates once and injects it into every use case.
func wire(params wireParams) app {
	repo := git.Repo{Dir: params.MainPath, Remote: remote}
	projectConfig := config.Project{StateDir: params.StateDir, GlobalPath: config.GlobalPath()}

	operations := operation.Service{
		Locker:    locker{oplock.Locks{StateDir: params.StateDir}},
		Publisher: params,
		NewOpID:   newOpID,
		Now:       time.Now,
	}

	provisions := provision.Service{
		Git:        repo,
		Meta:       store.Meta{StateDir: params.StateDir},
		EnvFiles:   store.EnvFiles{},
		RunHooks:   shell.RunHooks,
		PublicPort: proxyport.Port{Plist: proxyport.PlistPath()}.Public,
		Shield:     shell.Shield,
		Now:        time.Now,
		// clean exists only in the legacy wtm until it migrates
		LeftBehind: func(branch string) []string { return []string{"wtm", "clean", branch, "--force"} },
	}

	return app{
		Create: create.UseCase{
			Git:        repo,
			Config:     projectConfig,
			Answers:    projectConfig,
			Operations: operations,
			Provision:  provisions,
			MainPath:   params.MainPath,
			StateDir:   params.StateDir,
		},
		Extract: extract.UseCase{
			Git:        repo,
			Config:     projectConfig,
			Answers:    projectConfig,
			Meta:       provisions.Meta,
			Shield:     shell.Shield,
			Operations: operations,
			Provision:  provisions,
			MainPath:   params.MainPath,
			StateDir:   params.StateDir,
		},
	}
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

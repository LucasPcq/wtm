package main

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/proxyport"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

const (
	remote           = "origin"
	correlationIDEnv = "WTM_CORRELATION_ID"
	opIDBytes        = 8
)

type wireParams struct {
	MainPath  string
	StateDir  string
	CommonDir string
}

func wire(params wireParams) create.UseCase {
	repo := git.Repo{Dir: params.MainPath, Remote: remote}
	projectConfig := config.Project{StateDir: params.StateDir, GlobalPath: config.GlobalPath()}
	operations := operation.Service{
		Locker:        locker{oplock.Locks{StateDir: params.StateDir}},
		Publisher:     params,
		NewOpID:       newOpID,
		Now:           time.Now,
		CorrelationID: os.Getenv(correlationIDEnv),
	}
	provisions := provision.Service{
		Git:        repo,
		Meta:       store.Meta{StateDir: params.StateDir},
		EnvFiles:   store.EnvFiles{},
		RunHooks:   shell.RunHooks,
		PublicPort: proxyport.Port{Plist: proxyport.PlistPath()}.Public,
		Shield:     shell.Shield,
		Now:        time.Now,
	}
	return create.UseCase{
		Git:        repo,
		Config:     projectConfig,
		Answers:    projectConfig,
		Operations: operations,
		Provision:  provisions,
		MainPath:   params.MainPath,
		StateDir:   params.StateDir,
	}
}

type locker struct {
	oplock.Locks
}

func (l locker) Take(op domain.Operation) (operation.Hold, error) {
	return l.Locks.Take(op)
}

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

func newOpID() string {
	raw := make([]byte, opIDBytes)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}

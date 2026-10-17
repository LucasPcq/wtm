package main

import (
	"context"
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
	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/operation"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

const (
	remote           = "origin"
	surface          = "cli"
	correlationIDEnv = "WTM_CORRELATION_ID"
	opIDBytes        = 8
)

type appParams struct {
	MainPath  string
	StateDir  string
	CommonDir string
}

func newApp(params appParams) *app.App {
	repo := git.Repo{Dir: params.MainPath, Remote: remote}
	return app.New(app.Deps{
		Create: create.Deps{
			Git: repo,
			LoadConfig: func() (project.Config, project.Global, error) {
				return config.Load(config.LoadParams{StateDir: params.StateDir, GlobalPath: config.GlobalPath()})
			},
			LoadRun: func() (run.Config, error) { return config.LoadRun(params.StateDir) },
			Remember: func(ctx context.Context, answers map[string]string) (wtm.Written, error) {
				return config.Remember(ctx, config.RememberParams{StateDir: params.StateDir, Answers: answers})
			},
			Provision: provision.Deps{
				Git:        repo,
				Meta:       store.Meta{StateDir: params.StateDir},
				EnvFiles:   store.EnvFiles{},
				RunHooks:   shell.RunHooks,
				PublicPort: proxyport.Port{Plist: proxyport.PlistPath()}.Public,
				Shield:     shell.Shield,
				Now:        time.Now,
			},
			MainPath: params.MainPath,
			StateDir: params.StateDir,
		},
		TakeLocks: func(op operation.Operation) (app.Hold, error) {
			return oplock.Locks{StateDir: params.StateDir}.Take(op)
		},
		Publisher: func(op operation.Operation) wtm.Report {
			return events.NewPublisher(params.publisherParams(op)).Publish
		},
		NewOpID:       newOpID,
		Now:           time.Now,
		Surface:       surface,
		CorrelationID: os.Getenv(correlationIDEnv),
	})
}

func (p appParams) publisherParams(op operation.Operation) events.PublisherParams {
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

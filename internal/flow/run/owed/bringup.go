package owed

import (
	"context"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type BringUpParams struct {
	Context flow.Context
	Config  domain.RunConfig
	Job     string
}

// BringUp starts a shared service from the main checkout — where it runs — so
// what is owed to it can be given back now, and hands back how to let it go.
// Letting go is main's hold released: the service stops unless a worktree took
// a claim on it meanwhile.
func BringUp(ctx context.Context, params BringUpParams) (release func(), err error) {
	job, found := rules.FindJob(params.Config, params.Job)
	if !found {
		return nil, fmt.Errorf("%w: %s", domain.ErrJobNotFound, params.Job)
	}
	main, err := worktree.MainCheckout(ctx, worktree.MainCheckoutParams{ProjectDir: params.Context.ProjectDir})
	if err != nil {
		return nil, err
	}
	socket := process.SocketPath()
	if err := process.EnsureDaemon(process.DaemonParams{SocketPath: socket, ProxyPort: rules.ProxyPort(params.Context.Config.Global)}); err != nil {
		return nil, fmt.Errorf("ensure daemon: %w", err)
	}

	mainSeam := seam.Open(ctx, seam.Params{
		ProjectDir: params.Context.ProjectDir,
		StateDir:   params.Context.StateDir,
		WorkDir:    main,
		Jobs:       []domain.JobConfig{job},
		Declared:   params.Config.Jobs,
		NoProbe:    true,
		Publisher:  params.Context.Publisher,
	})
	outcomes, err := mainSeam.Starter(seam.StartParams{Jobs: rules.JobsWithEffectivePorts(params.Config, []domain.JobConfig{job})})(ctx, nil)
	if err != nil {
		return nil, err
	}
	if outcomes.Aborted() {
		return nil, fmt.Errorf(domain.OwedBringUpFailedFmt, job.Name, rules.SanitizeLogLine(string(outcomes.One().FailedOutput)))
	}
	return func() {
		_, _ = process.NewClient(socket).Send(process.Request{Action: process.ActionStop, Name: job.Name, WorkDir: main})
	}, nil
}

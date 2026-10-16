package provision

import (
	"context"
	"encoding/json"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules/envcopy"
	"github.com/LucasPcq/wtm/internal/service/gitwt"
	"github.com/LucasPcq/wtm/internal/store"
)

func (u unit) saga() kernel.Saga[Detail] {
	worktree := gitwt.Params{Git: u.Ports.Git, Path: u.Worktree.Path, Checkout: u.Worktree.Checkout}
	metadata := u.metadata()
	return kernel.Saga[Detail]{
		Steps: []kernel.SagaStep{
			{
				Name: StepWorktree,
				Do:   func(ctx context.Context, _ kernel.Reporter) error { return gitwt.Add(ctx, worktree) },
				Undo: func(ctx context.Context) error { return gitwt.Remove(ctx, worktree) },
			},
			{Name: StepEnv, Do: u.copyEnv},
			{
				Name: StepMeta,
				Do: func(ctx context.Context, report kernel.Reporter) error {
					return u.record(ctx, recordParams{Report: report, Metadata: metadata})
				},
				Undo: func(context.Context) error { return u.Ports.Meta.Purge(u.Worktree.Branch) },
			},
		},
		Commit: func(context.Context) (Detail, error) {
			return Detail{Path: u.Worktree.Path, Metadata: metadata, Origin: u.Worktree.Origin}, nil
		},
		LeftBehind: u.leftBehind,
	}
}

func (u unit) metadata() domain.WorktreeMetadata {
	return domain.WorktreeMetadata{
		SourceBranch: u.Worktree.Source,
		CreatedAt:    u.now().UTC().Format(time.RFC3339),
		EnvStrategy:  u.Worktree.Strategy,
		Isolation:    u.isolation(),
	}
}

func (u unit) now() time.Time {
	if u.Ports.Now == nil {
		return time.Now()
	}
	return u.Ports.Now()
}

func (u unit) copyEnv(ctx context.Context, report kernel.Reporter) error {
	if len(u.Project.EnvFiles) == 0 {
		return nil
	}
	copied, err := u.Ports.Env.Copy(ctx, envcopy.Request{
		Strategy:           u.Worktree.Strategy,
		Files:              u.Project.EnvFiles,
		TargetDir:          u.Worktree.Path,
		MainWorktreePath:   u.Project.MainPath,
		ParentWorktreePath: u.Worktree.ParentPath,
	})
	if err != nil {
		return err
	}
	for _, warning := range copied.Warnings {
		report.Warn(copyWarning(warning))
	}
	warnUnlocked(warnUnlockedParams{Report: report, Unlocked: copied.Unlocked})
	return nil
}

func copyWarning(warning envcopy.Warning) kernel.Warning {
	switch w := warning.(type) {
	case envcopy.NoTemplate:
		return kernel.Warning{Code: CodeEnvNoTemplate, Params: kernel.Params{ParamTarget: w.Target}}
	case envcopy.SourceMissing:
		return kernel.Warning{Code: CodeEnvSourceMissing, Params: kernel.Params{ParamTarget: w.Target, ParamDir: w.Dir}}
	case envcopy.ParentFallback:
		return kernel.Warning{Code: CodeEnvParentFallback, Params: kernel.Params{ParamTarget: w.Target}}
	}
	return kernel.Warning{}
}

type recordParams struct {
	Report   kernel.Reporter
	Metadata domain.WorktreeMetadata
}

func (u unit) record(ctx context.Context, params recordParams) error {
	written, err := u.Ports.Meta.Update(ctx, store.MetaUpdate{
		Branch:  u.Worktree.Branch,
		Missing: store.MissingCreate,
		Change: func(stored *domain.WorktreeMetadata) error {
			*stored = params.Metadata
			return nil
		},
	})
	warnUnlocked(warnUnlockedParams{Report: params.Report, Unlocked: written.Unlocked})
	return err
}

type cleanRequest struct {
	Branches []string `json:"branches"`
	Force    bool     `json:"force"`
}

func (u unit) leftBehind(string) *kernel.FollowUp {
	request, _ := json.Marshal(cleanRequest{Branches: []string{u.Worktree.Branch}, Force: true})
	return &kernel.FollowUp{
		Command: domain.CmdClean,
		Request: request,
		Code:    CodeLeftBehind,
		Params:  kernel.Params{ParamBranch: u.Worktree.Branch, kernel.ParamPath: u.Worktree.Path},
	}
}

type warnUnlockedParams struct {
	Report   kernel.Reporter
	Unlocked bool
}

func warnUnlocked(params warnUnlockedParams) {
	if params.Unlocked {
		params.Report.Warn(kernel.Warning{Code: kernel.CodeLocksUnavailable})
	}
}

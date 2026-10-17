// Package foreigndata stops a run before a job changes data the worktree does
// not own, shared by `run up` and `run start`.
package foreigndata

import (
	"context"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/rules"
)

type Params struct {
	Context  flow.Context
	Config   domain.RunConfig
	Jobs     []domain.JobConfig
	WorkDirs []string
	// Force is the safety axis: it lifts the refusal and the question alike.
	Force    bool
	Prompter flow.Prompter
}

// Allow says whether the run may go on. A terminal is asked; nobody to ask is
// a refusal naming --force, never a silent reset of someone else's database.
func Allow(ctx context.Context, params Params) (bool, error) {
	if params.Force {
		return true, nil
	}
	risks, err := Risks(ctx, params)
	if err != nil {
		return false, err
	}
	if len(risks) == 0 {
		return true, nil
	}

	lines := rules.ForeignDataLines(rules.ForeignDataLinesParams{Risks: risks, Several: len(params.WorkDirs) > 1})
	if !params.Prompter.Interactive() {
		return false, fmt.Errorf(domain.RunForeignDataRefusedFmt, domain.RunForeignDataTitle,
			strings.Join(lines, "\n"), domain.FlagForce, strings.Join(rules.ForeignDataHints(risks), domain.RunForeignDataHintSep))
	}
	return params.Prompter.Confirm(flow.ConfirmParams{
		Title:       domain.RunForeignDataTitle,
		Description: strings.Join(lines, "\n"),
		Warning:     domain.RunForeignDataDesc,
		DefaultYes:  false,
		YesLabel:    domain.RunForeignDataYes,
		NoLabel:     domain.RunForeignDataNo,
	})
}

// Risks reads each worktree's isolation from the environment its jobs would
// get, which is the same answer the daemon acts on.
func Risks(ctx context.Context, params Params) ([]domain.DataRisk, error) {
	if !rules.DeclaresTouches(params.Config, params.Jobs) {
		return nil, nil
	}
	var risks []domain.DataRisk
	for _, dir := range params.WorkDirs {
		env, err := seam.JobEnv(ctx, seam.JobEnvParams{ProjectDir: params.Context.ProjectDir, StateDir: params.Context.StateDir, WorkDir: dir, Publisher: params.Context.Publisher})
		if err != nil {
			return nil, err
		}
		risks = append(risks, rules.ForeignDataRisks(rules.ForeignDataParams{
			Config:    params.Config,
			Jobs:      params.Jobs,
			WorkDir:   dir,
			Isolation: domain.Isolation(env[domain.EnvIsolation]),
			Main:      env[domain.EnvOrdinal] == fmt.Sprint(domain.MainWorktreeOrdinal),
		})...)
	}
	return risks, nil
}

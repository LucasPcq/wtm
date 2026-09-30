// Package probes offers to silence the port warnings a job repeats at every
// run, shared by `run up` and `run start`.
package probes

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

type Params struct {
	Context   flow.Context
	Prompter  flow.Prompter
	Presenter flow.Presenter
	Config    domain.RunConfig
	Results   runlogs.Outcomes
}

// OfferToSilence asks once about a job binding its base port because its
// command never reads the variable. A port another worktree holds is not
// offered: the run already said whose it is. It returns the config the run goes
// on with.
func OfferToSilence(params Params) (domain.RunConfig, error) {
	// Never after an abort: the question to answer then is why the run stopped,
	// not whether to hear less about it.
	if !params.Prompter.Interactive() || params.Results.Aborted() {
		return params.Config, nil
	}

	var probes []domain.PortProbe
	for _, outcome := range params.Results {
		probes = append(probes, outcome.Probes...)
	}
	names := rules.JobsToSilence(rules.JobsToSilenceParams{Probes: probes, Jobs: params.Config.Jobs})
	if len(names) == 0 {
		return params.Config, nil
	}

	proceed, err := params.Prompter.Confirm(flow.ConfirmParams{
		Title:       domain.ProbeSilenceTitle,
		Description: fmt.Sprintf(domain.ProbeSilenceDescFmt, strings.Join(names, domain.CmdListVarSep)),
		DefaultYes:  false,
	})
	if err != nil || !proceed {
		return params.Config, nil
	}

	cfg := rules.SilenceProbes(rules.SilenceProbesParams{Config: params.Config, Jobs: names})
	if err := runconfig.Save(runconfig.SaveParams{StateDir: params.Context.StateDir, Config: cfg}); err != nil {
		return params.Config, fmt.Errorf("silence port probes: %w", err)
	}
	params.Presenter.Status(flow.Notice{
		Kind: flow.NoticeMessage,
		Text: fmt.Sprintf(domain.ProbeSilencedFmt, strings.Join(names, domain.CmdListVarSep)),
	})
	return cfg, nil
}

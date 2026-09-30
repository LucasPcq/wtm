// Package list runs the `wtm run list` flow: the listing that acts. It answers
// which entry of run.toml the reader picked and what to do to it; the surface
// runs that action through the flow it already has for it.
package list

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

const (
	KeyEntry  = "run.list.entry"
	KeyAction = "run.list.action"
)

// Selection is what the two questions came back with. Kind decides which
// command Name is handed to, which is why it travels with it.
type Selection struct {
	Kind    string
	Name    string
	Action  string
	Aborted bool
}

type Request struct {
	Config domain.RunConfig
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter flow.Presenter
}

func Run(params Params) (Selection, error) {
	answers, err := params.Prompter.Ask(flow.Session{
		ErrLabel: domain.CmdList,
		Steps: []flow.Step{
			entryStep(params.Request.Config),
			actionStep(),
		},
	})
	if errors.Is(err, domain.ErrUserAborted) {
		params.Presenter.Notice(flow.AbortedNotice)
		return Selection{Aborted: true}, nil
	}
	if err != nil {
		return Selection{}, err
	}

	kind, name, ok := splitEntry(answers.Value(KeyEntry))
	if !ok {
		return Selection{Aborted: true}, nil
	}
	return Selection{Kind: kind, Name: name, Action: answers.Value(KeyAction)}, nil
}

// entryStep offers the profiles and the jobs in one list, each row saying which
// it is. It has no Resolve: picking is the whole gesture, so a run that cannot
// ask has nothing to fall back to — the surface prints the table instead.
func entryStep(cfg domain.RunConfig) flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyEntry,
		Label: domain.RunListEntryStepName,
		Build: func(flow.Answers) (flow.StepContent, error) {
			options := entryOptions(cfg)
			if len(options) == 0 {
				return flow.StepContent{}, domain.ErrNoJobsDeclared
			}
			return flow.StepContent{Title: domain.RunListPickerTitle, Options: options}, nil
		},
	}
}

func entryOptions(cfg domain.RunConfig) []flow.Option {
	var options []flow.Option
	for _, profile := range cfg.Profiles {
		badges := []flow.Badge{{Text: fmt.Sprintf(domain.RunListProfileJobsFmt, len(profile.Jobs))}}
		if profile.Default {
			badges = append([]flow.Badge{{Text: domain.RunProfileDefaultBadge, Tone: domain.ToneSuccess}}, badges...)
		}
		options = append(options, flow.Option{
			Label:  profile.Name,
			Value:  entryValue(domain.RunListKindProfile, profile.Name),
			Badges: badges,
		})
	}
	if len(options) > 0 && len(cfg.Jobs) > 0 {
		options = append(options, flow.Option{Separator: true})
	}
	for _, job := range cfg.Jobs {
		options = append(options, flow.Option{
			Label:  job.Name,
			Value:  entryValue(domain.RunListKindJob, job.Name),
			Badges: []flow.Badge{{Text: string(job.Kind)}},
		})
	}
	return options
}

// actionStep offers what the picked kind can be told to do — a profile comes up
// and goes down as a whole, a job starts, stops and shows its output.
func actionStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyAction,
		Label: domain.RunCRUDActionStepName,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			kind, name, ok := splitEntry(answers.Value(KeyEntry))
			if !ok {
				return flow.StepContent{}, domain.ErrUserAborted
			}
			return flow.StepContent{
				Title:   fmt.Sprintf(domain.RunCRUDActionTitleFmt, name),
				Options: actionOptions(kind),
			}, nil
		},
	}
}

func actionOptions(kind string) []flow.Option {
	if kind == domain.RunListKindProfile {
		return []flow.Option{
			{Label: domain.RunListActionUpLabel, Value: domain.RunListActionUp},
			{Label: domain.RunListActionDownLabel, Value: domain.RunListActionDown},
		}
	}
	return []flow.Option{
		{Label: domain.RunListActionStartLabel, Value: domain.RunListActionStart},
		{Label: domain.RunListActionStopLabel, Value: domain.RunListActionStop},
		{Label: domain.RunListActionLogsLabel, Value: domain.RunListActionLogs},
	}
}

func entryValue(kind, name string) string { return kind + domain.RunListKindSep + name }

func splitEntry(value string) (kind, name string, ok bool) {
	kind, name, ok = strings.Cut(value, domain.RunListKindSep)
	return kind, name, ok
}

// Package orphans asks what becomes of the children a removal leaves behind,
// for clean and prune alike.
package orphans

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

const (
	Reparent = "reparent"
	Orphan   = "orphan"
)

const label = "Reparent children"

type StepParams struct {
	Key   string
	Moves func(flow.Answers) []domain.ReparentResult
}

func Step(params StepParams) flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   params.Key,
		Label: label,
		Skip: func(answers flow.Answers) (bool, string) {
			if len(params.Moves(answers)) == 0 {
				return true, domain.NoOrphanedChildren
			}
			return false, ""
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			moves := params.Moves(answers)
			return flow.StepContent{
				Description: proposal(moves),
				Options: []flow.Option{
					{Label: optionLabel(moves), Value: Reparent},
					{Separator: true},
					{Label: domain.OrphanOption, Value: Orphan},
				},
			}, nil
		},
		// Reparenting is opt-in: `wtm reparent` can still move the children afterwards.
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: Orphan}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == Reparent {
				return domain.ReparentSummary
			}
			return domain.OrphanSummary
		},
		Flag: domain.FlagReparentChildren,
	}
}

// Preset answers the step from --reparent-children, so the decision is not asked
// and every reader sees the same answer.
func Preset(reparentChildren bool) string {
	if reparentChildren {
		return Reparent
	}
	return ""
}

type RecapLineParams struct {
	Moves    []domain.ReparentResult
	Reparent bool
}

func RecapLine(params RecapLineParams) string {
	if len(params.Moves) == 0 {
		return ""
	}
	if !params.Reparent {
		return fmt.Sprintf(domain.RecapOrphanFmt, len(params.Moves))
	}
	if parent, single := soleParent(params.Moves); single {
		return fmt.Sprintf(domain.RecapReparentFmt, len(params.Moves), parent)
	}
	return fmt.Sprintf(domain.RecapReparentManyFmt, len(params.Moves))
}

func proposal(moves []domain.ReparentResult) string {
	lines := make([]string, 0, len(moves)+1)
	lines = append(lines, domain.ReparentIntro)
	for _, move := range moves {
		lines = append(lines, fmt.Sprintf(domain.ReparentChildFmt, move.Branch, move.NewParent, move.OldParent))
	}
	return strings.Join(lines, "\n")
}

// One destination reads as it always did; several are named one by one in the
// proposal above the options.
func optionLabel(moves []domain.ReparentResult) string {
	if parent, single := soleParent(moves); single {
		return fmt.Sprintf(domain.ReparentOptionFmt, parent, len(moves))
	}
	return fmt.Sprintf(domain.ReparentManyOptionFmt, len(moves))
}

func soleParent(moves []domain.ReparentResult) (string, bool) {
	if len(moves) == 0 {
		return "", false
	}
	for _, move := range moves[1:] {
		if move.NewParent != moves[0].NewParent {
			return "", false
		}
	}
	return moves[0].NewParent, true
}

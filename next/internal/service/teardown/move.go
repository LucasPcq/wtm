package teardown

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
)

// A Move is a child worktree a removal leaves behind: its branch, the parent removed under it, and the nearest worktree that stays, else the base branch, which reparenting records as its parent; Base says it is the base branch.
type Move struct {
	Branch string `json:"branch"`
	From   string `json:"from"`
	To     string `json:"to"`
	Base   bool   `json:"base,omitempty"`
}

// Moves are the children of the removed worktrees that stay, each with where it would move: up the chain of parents, past every one removed, to the base branch at its end.
func (f Facts) Moves(removals []Removal) []Move {
	removed := map[string]bool{}
	for _, r := range removals {
		if r.Branch != "" {
			removed[r.Branch] = true
		}
	}

	var moves []Move
	for _, r := range removals {
		for _, w := range f.Worktrees {
			child := w.Branch()
			if w.Main || child == "" || removed[child] || r.Branch == "" || f.Records[child].SourceBranch != r.Branch {
				continue
			}
			to := f.survivor(r.Branch, removed)
			moves = append(moves, Move{Branch: child, From: r.Branch, To: to, Base: to == f.Config.Worktrees.BaseBranch})
		}
	}

	return moves
}

// survivor is the nearest parent of branch that is not removed and still exists, a branch or a worktree's, else the base branch; a cycle of removed parents ends on the base branch too.
func (f Facts) survivor(branch string, removed map[string]bool) string {
	seen := map[string]bool{}
	for current := branch; current != "" && !seen[current]; current = f.Records[current].SourceBranch {
		if !removed[current] && (f.Local(current) || f.CheckedOutAt(current) != "") {
			return current
		}
		seen[current] = true
	}

	return f.Config.Worktrees.BaseBranch
}

// ChildPath is the path of the worktree of move's child, empty when no worktree holds it.
func (f Facts) ChildPath(move Move) string {
	return f.CheckedOutAt(move.Branch)
}

// The codes of a child a removal leaves behind, after the command's name: the warning that it still points at its removed parent, and the follow-up that moves it.
const (
	Orphaned = ".orphaned"
	Reparent = ".reparent"
)

// OrphanedBy warns of each child left on a parent removed, coded <command>.orphaned, followed by the command that moves it where reparenting would have.
func OrphanedBy(command string, moves []Move) []domain.Warning {
	warnings := make([]domain.Warning, 0, len(moves))
	for _, move := range moves {
		warnings = append(warnings, domain.Warning{
			Code:     command + Orphaned,
			Message:  move.Branch + " still points at the removed parent " + move.From,
			Params:   domain.Params{"branch": move.Branch, "from": move.From, "to": move.To},
			FollowUp: &domain.FollowUp{Command: commandReparent, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{move.Branch}}, {Path: "to", Values: []string{move.To}}}, Code: command + Reparent, Message: "move it onto " + move.To},
		})
	}

	return warnings
}

// NotReparented warns, coded <command>.orphaned, of the child of move whose record could not be written: it stays on its removed parent.
func NotReparented(command string, move Move, err error) domain.Warning {
	warning := OrphanedBy(command, []Move{move})[0]
	warning.Message += ": couldn't reparent it (" + err.Error() + ")"
	return warning
}

// commandReparent changes the parent a worktree is rebased onto: not migrated yet.
const commandReparent = "reparent"

// RecappedMoves are moves as a recap row's values: each child with where it goes, or, unless reparented, left on its removed parent.
func RecappedMoves(moves []Move, reparented bool) []domain.RecapValue {
	values := make([]domain.RecapValue, 0, len(moves))
	for _, move := range moves {
		value := domain.RecapValue{Value: move.Branch, Params: domain.Params{"from": move.From, "to": move.To}, Text: move.Branch + " → " + move.To}
		if !reparented {
			value.Note, value.Text = NoteOrphaned, move.Branch+" (left on "+move.From+")"
		}
		values = append(values, value)
	}

	return values
}

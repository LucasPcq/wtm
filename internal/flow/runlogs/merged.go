package runlogs

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// MergedEntry is one worktree's board and the worktree it is about. The pair is
// what routing needs: a worktree that declares no job and has nothing running
// lists nothing, and a board found by scanning its rows would not be found at all.
type MergedEntry struct {
	WorkDir string
	Board   Board
}

// NewMergedBoard shows several worktrees' jobs as one board, in the order they
// were selected. A board of one is returned as itself: a surface reading a
// single worktree must not pay for a routing layer it never uses.
func NewMergedBoard(entries []MergedEntry) Board {
	if len(entries) == 1 {
		return entries[0].Board
	}
	return &mergedBoard{entries: entries}
}

type mergedBoard struct {
	entries []MergedEntry
}

// Refresh refreshes every board and reports the first failure, having asked
// them all: a daemon that answered for two worktrees out of three still has
// something to show for the two.
func (m *mergedBoard) Refresh() error {
	var first error
	for _, entry := range m.entries {
		if err := entry.Board.Refresh(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (m *mergedBoard) Jobs() []JobView {
	var views []JobView
	for _, entry := range m.entries {
		views = append(views, entry.Board.Jobs()...)
	}
	return views
}

func (m *mergedBoard) Attach(params AttachParams) (Stream, error) {
	board, err := m.route(params.WorkDir)
	if err != nil {
		return nil, err
	}
	return board.Attach(params)
}

func (m *mergedBoard) History(params HistoryParams) ([]string, error) {
	board, err := m.route(params.WorkDir)
	if err != nil {
		return nil, err
	}
	return board.History(params)
}

// route finds the board a worktree belongs to. An unnamed worktree is refused
// rather than defaulted to the first: guessing would read one job's output under
// another's name.
func (m *mergedBoard) route(workDir string) (Board, error) {
	for _, entry := range m.entries {
		if entry.WorkDir == workDir {
			return entry.Board, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", domain.ErrWorktreeNotFound, workDir)
}

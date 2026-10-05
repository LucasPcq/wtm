package owed

import (
	"context"

	"github.com/LucasPcq/wtm/internal/flow"
)

// Holdings reads what a selection of worktrees holds once per selection: the
// data step and the recap both ask, and each read dials the daemon. The zero
// value is ready to use.
type Holdings struct {
	memo flow.SetMemo[Snapshot]
}

func (h *Holdings) Of(ctx context.Context, project flow.Context, branches []string) Snapshot {
	if len(branches) == 0 {
		return Snapshot{}
	}
	return h.memo.Get(branches, func() Snapshot {
		return Read(ctx, ReadParams{Context: project, Branches: branches})
	})
}

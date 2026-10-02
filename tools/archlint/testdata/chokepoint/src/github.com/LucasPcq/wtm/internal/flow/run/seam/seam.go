package seam

import wt "github.com/LucasPcq/wtm/internal/service/worktree"

func Ordinal() (int, error) { return wt.EnsureOrdinal() }

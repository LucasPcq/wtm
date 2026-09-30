package runjobs

import (
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

func LoadPendingRemovals(stateDir string) []domain.NamespaceRef {
	return config.LoadPendingRemovals(stateDir)
}

type QueueRemovalsParams struct {
	StateDir string
	Refs     []domain.NamespaceRef
}

// QueueRemovals adds what a clean could not give back. An entry already held is
// not added twice: the same worktree cleaned twice owes one namespace, not two.
func QueueRemovals(params QueueRemovalsParams) error {
	if len(params.Refs) == 0 {
		return nil
	}
	return config.WritePendingRemovals(config.WritePendingRemovalsParams{
		StateDir: params.StateDir,
		Refs:     mergeRefs(LoadPendingRemovals(params.StateDir), params.Refs),
	})
}

type SettleRemovalsParams struct {
	StateDir string
	Refs     []domain.NamespaceRef
}

// SettleRemovals removes what has since been given back. The queue only ever
// shrinks this way, which is what keeps it from drifting.
func SettleRemovals(params SettleRemovalsParams) error {
	if len(params.Refs) == 0 {
		return nil
	}
	settled := make(map[domain.NamespaceRef]bool, len(params.Refs))
	for _, ref := range params.Refs {
		settled[ref] = true
	}

	var kept []domain.NamespaceRef
	for _, ref := range LoadPendingRemovals(params.StateDir) {
		if !settled[ref] {
			kept = append(kept, ref)
		}
	}
	return config.WritePendingRemovals(config.WritePendingRemovalsParams{StateDir: params.StateDir, Refs: kept})
}

func mergeRefs(held, added []domain.NamespaceRef) []domain.NamespaceRef {
	seen := make(map[domain.NamespaceRef]bool, len(held))
	for _, ref := range held {
		seen[ref] = true
	}
	for _, ref := range added {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		held = append(held, ref)
	}
	return held
}

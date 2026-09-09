package runjobs

import (
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

// LoadPendingDetach is what a clean owes and a prune settles.
func LoadPendingDetach(stateDir string) []domain.TenantRef {
	return config.LoadPendingDetach(stateDir)
}

type QueueDetachParams struct {
	StateDir string
	Refs     []domain.TenantRef
}

// QueueDetach adds what a clean could not give back. An entry already held is
// not added twice: the same worktree cleaned twice owes one tenant, not two.
func QueueDetach(params QueueDetachParams) error {
	if len(params.Refs) == 0 {
		return nil
	}
	return config.WritePendingDetach(config.WritePendingDetachParams{
		StateDir: params.StateDir,
		Refs:     mergeRefs(LoadPendingDetach(params.StateDir), params.Refs),
	})
}

type SettleDetachParams struct {
	StateDir string
	Refs     []domain.TenantRef
}

// SettleDetach removes what has since been given back. The queue only ever
// shrinks this way, which is what keeps it from drifting.
func SettleDetach(params SettleDetachParams) error {
	if len(params.Refs) == 0 {
		return nil
	}
	settled := make(map[domain.TenantRef]bool, len(params.Refs))
	for _, ref := range params.Refs {
		settled[ref] = true
	}

	var kept []domain.TenantRef
	for _, ref := range LoadPendingDetach(params.StateDir) {
		if !settled[ref] {
			kept = append(kept, ref)
		}
	}
	return config.WritePendingDetach(config.WritePendingDetachParams{StateDir: params.StateDir, Refs: kept})
}

func mergeRefs(held, added []domain.TenantRef) []domain.TenantRef {
	seen := make(map[domain.TenantRef]bool, len(held))
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

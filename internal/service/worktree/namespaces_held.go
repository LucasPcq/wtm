package worktree

import (
	"slices"

	"github.com/LucasPcq/wtm/internal/store"
)

type NamespaceHoldersParams struct {
	StateDir string
	Job      string
}

// NamespaceHolders names the worktrees whose meta.json records a namespace in
// this shared job, sorted.
func NamespaceHolders(params NamespaceHoldersParams) []string {
	records, err := (store.MetaDisk{StateDir: params.StateDir}).Records()
	if err != nil {
		return nil
	}
	var holders []string
	for _, record := range records {
		if slices.Contains(record.Meta.Namespaces, params.Job) {
			holders = append(holders, record.Branch)
		}
	}
	return holders
}

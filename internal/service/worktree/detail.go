package worktree

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// ParentBranchParams holds inputs for resolving a worktree's parent branch.
type ParentBranchParams struct {
	StateDir string
	Branch   string
}

// ParentBranch returns the branch the given worktree was created from, read from
// its metadata. Returns an empty string when no metadata is recorded.
func ParentBranch(params ParentBranchParams) string {
	return loadSourceBranch(params.StateDir, params.Branch)
}

// Metadata returns the recorded metadata for a worktree and true, or a zero
// value and false when none is recorded. Callers that need the memorized env
// strategy or parent branch (e.g. `wtm env`) read it through here.
func Metadata(params ParentBranchParams) (domain.WorktreeMetadata, bool) {
	meta, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil {
		return domain.WorktreeMetadata{}, false
	}
	return meta, true
}

func loadSourceBranch(stateDir, branch string) string {
	meta, err := loadMetadata(stateDir, branch)
	if err != nil {
		return ""
	}
	return meta.SourceBranch
}

// loadMetadata reads the full meta.json for a worktree, so callers can update one
// field while preserving the rest (CreatedAt, EnvStrategy).
func loadMetadata(stateDir, branch string) (domain.WorktreeMetadata, error) {
	metaPath := filepath.Join(rules.WorktreeMetaDir(stateDir, branch), domain.MetaFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return domain.WorktreeMetadata{}, err
	}
	var meta domain.WorktreeMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return domain.WorktreeMetadata{}, err
	}
	return meta, nil
}

type RecordTenantsParams struct {
	StateDir string
	Branch   string
	Jobs     []string
}

// RecordTenants remembers that this worktree holds a tenant in each of these
// shared services, so `clean` gives back exactly what exists. Additive and
// idempotent: a job already recorded is not recorded twice, and a worktree with
// no metadata — the main checkout — records nothing rather than creating some.
func RecordTenants(params RecordTenantsParams) error {
	if len(params.Jobs) == 0 {
		return nil
	}
	meta, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil {
		return nil
	}

	held := make(map[string]bool, len(meta.Tenants))
	for _, job := range meta.Tenants {
		held[job] = true
	}
	changed := false
	for _, job := range params.Jobs {
		if held[job] {
			continue
		}
		held[job] = true
		meta.Tenants = append(meta.Tenants, job)
		changed = true
	}
	if !changed {
		return nil
	}
	return writeMetadata(rules.WorktreeMetaDir(params.StateDir, params.Branch), meta)
}

// TenantsOf is what this worktree has to give back when it goes.
func TenantsOf(params ParentBranchParams) []string {
	meta, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil {
		return nil
	}
	return meta.Tenants
}

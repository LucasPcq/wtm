package env

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// EnvPortsParams locates the .env files a set of links names and the offset the
// worktree binds on. Bases comes from the run config, so this package never
// reads run.toml itself.
type EnvPortsParams struct {
	WorktreePath string
	Links        []domain.EnvPortLink
	Bases        map[domain.PortRef]int
	Offset       int
	// Block is the spacing between two worktrees' ports, which the reconciliation
	// diff needs to recognize another worktree's spelling of the same port.
	Block int
	// Origins is what turns a port substitution into an address one. A zero
	// value plans ports, which is every caller that knows nothing of the proxy.
	Origins rules.OriginContext
	// Owned are the wtm-derived keys this worktree's .env carries, resolved by
	// the caller — the only one that can ask git which worktree this is.
	Owned []domain.EnvOwnedEntry
	// Shared names the jobs that run once for the repository, so their ports are
	// written into the .env unshifted — the service binds what it declares.
	Shared map[string]bool
}

// Empty reports whether the project declares no link at all, which is the common
// case and means every caller can skip the whole reconciliation.
func (p EnvPortsParams) Empty() bool { return len(p.Links) == 0 && len(p.Owned) == 0 }

// ComputeEnvPorts resolves every link against the worktree's current .env files,
// writing nothing. A file that does not exist yields missing keys rather than
// silence — a link the user declared must stay visible in the report.
func ComputeEnvPorts(params EnvPortsParams) (domain.EnvPortPlan, error) {
	lines, err := readLinkedFiles(params)
	if err != nil {
		return domain.EnvPortPlan{}, err
	}

	plan := rules.PlanEnvPorts(rules.PlanEnvPortsParams{
		Links:   params.Links,
		Bases:   params.Bases,
		Offset:  params.Offset,
		Block:   params.Block,
		Lines:   lines,
		Origins: params.Origins,
		Shared:  params.Shared,
	})
	owned, err := planOwned(params)
	if err != nil {
		return domain.EnvPortPlan{}, err
	}
	plan.Owned = owned
	return plan, nil
}

// planOwned reads each file an owned key lands in and says whether it already
// holds that value, so a report never announces a write that changed nothing.
func planOwned(params EnvPortsParams) ([]domain.EnvOwnedEntry, error) {
	if len(params.Owned) == 0 {
		return nil, nil
	}

	byFile := map[string][]domain.EnvLine{}
	out := make([]domain.EnvOwnedEntry, 0, len(params.Owned))
	for _, entry := range params.Owned {
		lines, read := byFile[entry.File]
		if !read {
			parsed, err := readEnvFile(filepath.Join(params.WorktreePath, entry.File))
			if err != nil {
				return nil, err
			}
			byFile[entry.File] = parsed
			lines = parsed
		}
		updated, changed := rules.UpsertEnvPair(rules.UpsertEnvPairParams{Lines: lines, Key: entry.Key, Value: entry.Value})
		byFile[entry.File] = updated
		entry.Changed = changed
		out = append(out, entry)
	}
	return out, nil
}

// ApplyEnvPorts recomputes the plan and writes the rewrites it holds, one file at
// a time. It returns the plan it acted on, so a caller reports what happened
// rather than what it intended.
func ApplyEnvPorts(params EnvPortsParams) (domain.EnvPortPlan, error) {
	plan, err := ComputeEnvPorts(params)
	if err != nil {
		return domain.EnvPortPlan{}, err
	}

	for _, file := range rewrittenFiles(plan) {
		path := filepath.Join(params.WorktreePath, file)
		lines, readErr := readEnvFile(path)
		if readErr != nil {
			return domain.EnvPortPlan{}, readErr
		}

		rendered := rules.RenderEnv(rules.ApplyEnvPorts(lines, entriesForFile(plan, file)))
		if rendered == rules.RenderEnv(lines) {
			continue
		}
		if writeErr := writeEnvFile(path, rendered); writeErr != nil {
			return domain.EnvPortPlan{}, writeErr
		}
	}

	if err := applyOwned(params, plan); err != nil {
		return domain.EnvPortPlan{}, err
	}

	return plan, nil
}

// ApplyOwnedEnv writes the worktree identity alone. The port pass is a question
// the user may answer no to; which worktree this is, is not one of the things
// being asked.
func ApplyOwnedEnv(params EnvPortsParams) error {
	if len(params.Owned) == 0 {
		return nil
	}
	plan, err := ComputeEnvPorts(params)
	if err != nil {
		return err
	}
	return applyOwned(params, plan)
}

// applyOwned writes the owned keys one file at a time, after the port rewrites:
// both touch the same documents, and reading the file back is what keeps the
// second write from dropping the first.
func applyOwned(params EnvPortsParams, plan domain.EnvPortPlan) error {
	for _, file := range ownedFiles(plan) {
		path := filepath.Join(params.WorktreePath, file)
		lines, err := readEnvFile(path)
		if err != nil {
			return err
		}

		changed := false
		for _, entry := range plan.Owned {
			if entry.File != file {
				continue
			}
			updated, wrote := rules.UpsertEnvPair(rules.UpsertEnvPairParams{Lines: lines, Key: entry.Key, Value: entry.Value})
			lines = updated
			changed = changed || wrote
		}
		if !changed {
			continue
		}
		if err := writeEnvFile(path, rules.RenderEnv(lines)); err != nil {
			return err
		}
	}
	return nil
}

func ownedFiles(plan domain.EnvPortPlan) []string {
	var files []string
	seen := map[string]bool{}
	for _, entry := range plan.Owned {
		if !entry.Changed || seen[entry.File] {
			continue
		}
		seen[entry.File] = true
		files = append(files, entry.File)
	}
	return files
}

// EnvValueRefsFor is how each linked key of one env target may be spelled, which
// the reconciliation diff needs to compare two worktrees' values.
func EnvValueRefsFor(params EnvPortsParams, target string) map[string]rules.EnvValueRef {
	return rules.EnvValueRefsByKey(linksForFile(params.Links, target), params.Bases, params.Origins)
}

func readLinkedFiles(params EnvPortsParams) (map[string][]domain.EnvLine, error) {
	lines := map[string][]domain.EnvLine{}
	for _, link := range params.Links {
		if _, read := lines[link.File]; read {
			continue
		}
		parsed, err := readEnvFile(filepath.Join(params.WorktreePath, link.File))
		if err != nil {
			return nil, err
		}
		lines[link.File] = parsed
	}
	return lines, nil
}

func rewrittenFiles(plan domain.EnvPortPlan) []string {
	var files []string
	seen := map[string]bool{}
	for _, entry := range rules.EnvPortRewrites(plan) {
		if seen[entry.File] {
			continue
		}
		seen[entry.File] = true
		files = append(files, entry.File)
	}
	return files
}

func entriesForFile(plan domain.EnvPortPlan, file string) []domain.EnvPortEntry {
	var out []domain.EnvPortEntry
	for _, entry := range plan.Entries {
		if entry.File == file {
			out = append(out, entry)
		}
	}
	return out
}

func linksForFile(links []domain.EnvPortLink, file string) []domain.EnvPortLink {
	var out []domain.EnvPortLink
	for _, link := range links {
		if link.File == file {
			out = append(out, link)
		}
	}
	return out
}

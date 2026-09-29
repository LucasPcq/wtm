package worktree

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

type NamespaceHoldersParams struct {
	StateDir string
	Job      string
}

// NamespaceHolders names the worktrees whose meta.json records a namespace in
// this shared job. It reads every record on disk rather than the live
// worktrees: a record is what clean drops by, whether or not git still lists
// the worktree.
func NamespaceHolders(params NamespaceHoldersParams) []string {
	root := filepath.Join(params.StateDir, domain.WorktreesSubdir)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var holders []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name(), domain.MetaFileName))
		if err != nil {
			continue
		}
		var meta domain.WorktreeMetadata
		if json.Unmarshal(data, &meta) != nil || !slices.Contains(meta.Namespaces, params.Job) {
			continue
		}
		branch, err := url.PathUnescape(entry.Name())
		if err != nil {
			branch = entry.Name()
		}
		holders = append(holders, branch)
	}
	sort.Strings(holders)
	return holders
}

package detect

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// SubProjectsParams holds the inputs for SubProjects.
type SubProjectsParams struct {
	// Root is the project directory scanned for independently-installable dirs.
	Root string
	// MaxDepth caps how deep the scan descends below Root (1 = direct children).
	MaxDepth int
}

// SubProjects finds directories below Root that carry their own lockfile — the
// signature of a repo whose projects install independently. Root itself is never
// returned. A matched directory is not descended into: the outermost lockfile
// wins, so a nested package inside an already-installable directory does not
// produce a second hook. Excludes node_modules, .trees, .git, vendor and dist.
// Results are sorted by directory.
func SubProjects(params SubProjectsParams) []domain.SubProject {
	var subs []domain.SubProject

	_ = filepath.WalkDir(params.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if path == params.Root {
			return nil
		}
		if scanSkipDirs[d.Name()] {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(params.Root, path)
		if err != nil {
			return filepath.SkipDir
		}
		if len(strings.Split(rel, string(filepath.Separator))) > params.MaxDepth {
			return filepath.SkipDir
		}

		pm := PackageManager(path)
		if pm == domain.PkgManagerNone {
			return nil
		}

		subs = append(subs, domain.SubProject{Dir: filepath.ToSlash(rel), PackageManager: pm})
		return filepath.SkipDir
	})

	sort.Slice(subs, func(i, j int) bool { return subs[i].Dir < subs[j].Dir })
	return subs
}

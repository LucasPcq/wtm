package detect

import "github.com/LucasPcq/wtm/internal/domain"

// scanSkipDirs are the directory names every project scan walks past — build
// output, vendored deps, VCS internals, and wtm's own worktree root.
var scanSkipDirs = map[string]bool{
	domain.ScanSkipNodeModules: true,
	domain.ScanSkipTrees:       true,
	domain.ScanSkipGit:         true,
	domain.ScanSkipVendor:      true,
	domain.ScanSkipDist:        true,
}

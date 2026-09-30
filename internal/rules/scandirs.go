package rules

// scanIgnoredDirs are the directories a project scan never descends into:
// dependency trees, build output, and wtm's own worktree root. Walking them
// costs more than the whole rest of the repository and answers nothing.
var scanIgnoredDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".trees":       true,
	".next":        true,
	".turbo":       true,
	".venv":        true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	"target":       true,
	"coverage":     true,
}

// IsScanIgnoredDir reports whether a directory name is one a project scan skips.
func IsScanIgnoredDir(name string) bool {
	return scanIgnoredDirs[name]
}

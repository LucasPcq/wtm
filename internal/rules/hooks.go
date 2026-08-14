package rules

import (
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TemplateVars holds the variables available for interpolation in hook commands.
type TemplateVars struct {
	Worktree   string
	Branch     string
	Root       string
	FromBranch string
}

// ResolveTemplateVars substitutes template variables in a hook command's Cmd and Cwd fields.
func ResolveTemplateVars(hook domain.HookCommand, vars TemplateVars) domain.HookCommand {
	hook.Cmd = Interpolate(hook.Cmd, vars)
	hook.Cwd = Interpolate(hook.Cwd, vars)
	return hook
}

// HookDirParams holds the inputs for HookDir.
type HookDirParams struct {
	// Cwd is the hook's configured working directory: empty, relative to the
	// worktree root, or absolute.
	Cwd string
	// WorkDir is the worktree root the hook runs in by default.
	WorkDir string
}

// HookDir resolves the directory a hook runs in. An empty Cwd runs at WorkDir; an
// absolute Cwd — including one produced by interpolating {{worktree}} or {{root}}
// — is used as-is; a relative Cwd is joined onto WorkDir, honouring the documented
// "relative to the worktree root" contract instead of resolving against the
// process working directory.
func HookDir(params HookDirParams) string {
	if params.Cwd == "" {
		return params.WorkDir
	}
	if filepath.IsAbs(params.Cwd) {
		return params.Cwd
	}
	return filepath.Join(params.WorkDir, params.Cwd)
}

// Interpolate replaces {{worktree}}, {{branch}}, {{root}}, {{from_branch}} in s.
func Interpolate(s string, vars TemplateVars) string {
	if s == "" {
		return s
	}
	r := strings.NewReplacer(
		"{{worktree}}", vars.Worktree,
		"{{branch}}", vars.Branch,
		"{{root}}", vars.Root,
		"{{from_branch}}", vars.FromBranch,
	)
	return r.Replace(s)
}

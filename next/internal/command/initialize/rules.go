package initialize

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what init decides from: the settings' facts (config.toml as written, what the repository suggests, its branches), where the main worktree and config.toml are, the linked worktrees, and whether run.toml declares anything.
type Facts struct {
	Settings    settings.Facts
	MainPath    string
	ConfigPath  string
	Worktrees   []worktree.Worktree
	RunDeclared bool
}

// shown is path relative to the main worktree when it is in it, else as it is.
func (p Plan) shown(path string) string {
	return Facts{MainPath: p.mainPath}.shown(path)
}

// outside are the linked worktrees whose directory is not under basePath, relative to the main worktree.
func (f Facts) outside(basePath string) []string {
	base := filepath.Join(f.MainPath, basePath) + string(filepath.Separator)
	var found []string
	for _, w := range f.Worktrees {
		if w.Path != f.MainPath && !strings.HasPrefix(w.Path, base) {
			found = append(found, w.Path)
		}
	}

	return found
}

// shown is path relative to the main worktree when it is in it, else as it is.
func (f Facts) shown(path string) string {
	if rel, err := filepath.Rel(f.MainPath, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}

	return path
}

// linked is the count of worktrees but the main one.
func (f Facts) linked() int {
	return len(slices.DeleteFunc(slices.Clone(f.Worktrees), func(w worktree.Worktree) bool { return w.Path == f.MainPath }))
}

// ----------------------------------------------------------------------------
// Decide

// CodeBasePathMoved warns that a re-init moved the folder of new worktrees while worktrees stay in the one before: relocate moves them.
const CodeBasePathMoved = Name + ".base_path_moved"

// Decide turns the facts and the resolved request into the plan: the settings to write, the file created or rewritten, and on a re-init the settings changed; a new folder for worktrees that leaves some in the old one is warned.
func Decide(f Facts, req Request) (Plan, []domain.Warning) {
	now := NewQuestions(f, req)
	plan := Plan{Path: f.ConfigPath, Created: f.Settings.Saved == nil, Settings: now.Document(), Changed: []string{}, shownPath: f.shown(f.ConfigPath), mainPath: f.MainPath, rewrite: now.Rewrite(), runDeclared: f.RunDeclared}
	plan.elsewhere = f.outside(plan.Settings.BasePath)
	if plan.Created {
		return plan, nil
	}

	plan.was = NewQuestions(f, Request{}).Document()
	if changed := settings.Changed(plan.was, plan.Settings); changed != nil {
		plan.Changed = changed
	}
	stay := f.linked() - len(f.outside(plan.was.BasePath))
	if plan.Settings.BasePath == plan.was.BasePath || stay == 0 {
		return plan, nil
	}

	plan.moved = true
	count := strconv.Itoa(stay)
	warning := domain.Warning{
		Code:     CodeBasePathMoved,
		Message:  count + " worktrees stay in " + plan.was.BasePath + ": new worktrees go in " + plan.Settings.BasePath,
		Params:   domain.Params{"from": plan.was.BasePath, "to": plan.Settings.BasePath, "count": count},
		FollowUp: relocate(),
	}
	return plan, []domain.Warning{warning}
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: config.toml's path, whether it is created, the settings it will hold and, on a re-init, those that change; then the recap, which the fingerprint leaves out.
type Plan struct {
	Path     string            `json:"path"`
	Created  bool              `json:"created"`
	Settings settings.Document `json:"settings"`
	Changed  []string          `json:"changed"`
	domain.Recapped
	shownPath   string // Path, relative to the main worktree when it is in it
	mainPath    string
	rewrite     project.Rewrite
	was         settings.Document
	elsewhere   []string
	runDeclared bool
	moved       bool
}

// Locks are the locks the plan needs: the repository exclusive, since its settings change for every operation.
func (Plan) Locks() []domain.Lock {
	return Request{}.Locks()
}

// Fingerprinted is what the plan's fingerprint covers: the plan, its recap left out, and the file it replaces, so a config.toml changed since the preview refuses the apply.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	return struct {
		Plan
		Replaces string `json:"replaces"`
	}{p, p.rewrite.Replaces}
}

// Locks are the locks taken before planning: the repository exclusive.
func (Request) Locks() []domain.Lock {
	return []domain.Lock{{Scope: domain.ScopeRepo, Mode: domain.LockExclusive}}
}

// ----------------------------------------------------------------------------
// The recap

// NoteChanged notes a recap's value a re-init changes, with the value it had (was).
const NoteChanged = "changed"

// empty reports whether applying the plan changes nothing: a re-init that gives every setting the value it has.
func (p Plan) empty() bool {
	return !p.Created && len(p.Changed) == 0
}

// recap is the plan setting by setting, each value a re-init changes noted with the one it had.
func (p Plan) recap() []domain.RecapRow {
	s := p.Settings
	files := make([]string, 0, len(s.EnvFiles))
	for _, file := range s.EnvFiles {
		files = append(files, file.Target)
	}

	return []domain.RecapRow{
		{Key: "base_path", Label: "Worktrees folder", Path: settings.PathBasePath, Values: []domain.RecapValue{p.one(settings.PathBasePath, s.BasePath, p.was.BasePath)}},
		{Key: "base_branch", Label: "Base branch", Path: settings.PathBaseBranch, Values: []domain.RecapValue{p.one(settings.PathBaseBranch, s.BaseBranch, p.was.BaseBranch)}},
		{Key: "env_strategy", Label: ".env from", Path: settings.PathEnvStrategy, Values: []domain.RecapValue{p.one(settings.PathEnvStrategy, s.EnvStrategy, p.was.EnvStrategy)}},
		{Key: "env_files", Label: ".env files", Path: settings.PathEnvFiles, Stacked: true, Values: listed(files)},
		{Key: "on_create", Label: "On create", Path: settings.PathOnCreate, Stacked: true, Values: listed(hooksText(s.OnCreate))},
		{Key: "on_clean", Label: "On clean", Path: settings.PathOnClean, Stacked: true, Values: listed(hooksText(s.OnClean))},
		{Key: "file", Label: "Settings file", Values: []domain.RecapValue{{Value: p.Path, Text: p.shownPath}}},
	}
}

// one is a recap's single value, noted with the one it had when a re-init changes it.
func (p Plan) one(path, value, was string) domain.RecapValue {
	if !slices.Contains(p.Changed, path) {
		return domain.RecapValue{Value: value, Text: value}
	}

	return domain.RecapValue{Value: value, Note: NoteChanged, Params: domain.Params{"was": was}, Text: value + " (was " + was + ")"}
}

// NoteNone notes a recap's list that holds nothing.
const NoteNone = "none"

func listed(values []string) []domain.RecapValue {
	if len(values) == 0 {
		return []domain.RecapValue{{Note: NoteNone, Text: "none"}}
	}

	return domain.RecapValues(values)
}

// hooksText spells each hook as the recap shows it: its command, the folder it runs in and whether it goes on after a failure.
func hooksText(hooks []project.Hook) []string {
	texts := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		text := hook.Cmd
		if hook.Cwd != "" {
			text += " (in " + hook.Cwd + ")"
		}
		if hook.ContinueOnError {
			text += " (goes on if it fails)"
		}
		texts = append(texts, text)
	}

	return texts
}

// ----------------------------------------------------------------------------
// The outcome

// The steps after a set-up: create a worktree; relocate those made elsewhere; set up what runs in a worktree, while run.toml declares nothing.
const (
	CodeCreate   = Name + ".create"
	CodeRelocate = Name + ".relocate"
	CodeRunInit  = Name + ".run_init"
)

// The commands not yet migrated a set-up follows up with, spelled as the legacy wtm runs them.
const (
	commandRelocate = "relocate"
	commandRunInit  = "run init"
)

func relocate() *domain.FollowUp {
	return &domain.FollowUp{Command: commandRelocate, Code: CodeRelocate, Message: "move the worktrees made elsewhere into the folder"}
}

// next are the steps after the settings were written: create a worktree, relocate those outside the folder of new worktrees unless a warning offers it already, set up run while run.toml declares nothing.
func (p Plan) next() []domain.FollowUp {
	steps := []domain.FollowUp{{Command: domain.CommandCreate, Code: CodeCreate, Message: "create a worktree to get started"}}
	if len(p.elsewhere) > 0 && !p.moved {
		follow := relocate()
		follow.Params = domain.Params{"count": strconv.Itoa(len(p.elsewhere))}
		steps = append(steps, *follow)
	}
	if !p.runDeclared {
		steps = append(steps, domain.FollowUp{Command: commandRunInit, Code: CodeRunInit, Message: "set up what runs in each worktree"})
	}

	return steps
}

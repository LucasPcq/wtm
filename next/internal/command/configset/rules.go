package configset

import (
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what config set decides from: config.toml as written and what the repository knows of the settings (its branches, its folders, the files and hooks it suggests).
type Facts struct {
	Settings settings.Facts
}

// ----------------------------------------------------------------------------
// Decide

// The codes of a config.toml config set refuses to write: one that changed on disk since the request was made on it, one a newer wtm wrote (this one does not know what its fields mean; its questions still read it).
const (
	CodeChangedOnDisk = Name + ".changed_on_disk"
	CodeConfigNewer   = Name + ".config_newer"
)

// Decide turns the request into the plan: config.toml rewritten with each setting given, refused when the file changed since the request was made on it, and with every problem of the settings it changes; a problem another setting holds on disk already is left to its own change.
func Decide(facts Facts, req Request) (Plan, error) {
	if saved := facts.Settings.Saved; saved.Newer() {
		newer := &domain.SchemaNewer{Path: subject, Found: saved.Schema, Supported: project.ConfigSchema}
		return Plan{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigNewer, Message: newer.Error(), Cause: newer}
	}
	if req.Replaces != "" && req.Replaces != facts.Settings.Saved.Digest {
		return Plan{}, &domain.Error{Kind: domain.KindConflict, Code: CodeChangedOnDisk, Message: "config.toml changed on disk since this change was made: nothing written"}
	}

	asked := NewQuestions(facts, req)
	if problems := slices.DeleteFunc(asked.Problems(), func(problem domain.FieldError) bool { return req.Values(fieldOf(problem.Path)) == nil }); len(problems) > 0 {
		return Plan{}, domain.Invalid(problems)
	}

	was := NewQuestions(facts, Request{})
	plan := Plan{Changes: []Change{}, rewrite: asked.Questions.Rewrite()}
	plan.rewrite.Config.Remembered = asked.remembered()
	plan.Replaces = plan.rewrite.Replaces
	changed := settings.Changed(was.Document(), asked.Document())
	if !slices.Equal(was.Remembered.Value, asked.Remembered.Value) {
		changed = append(changed, PathRemembered)
	}
	before, after := was.request(), asked.request()
	for _, path := range changed {
		plan.Changes = append(plan.Changes, Change{Path: path, Before: answered(before.Values(path)), After: answered(after.Values(path))})
	}
	plan.Recap, plan.Empty = plan.recap(), len(plan.Changes) == 0
	return plan, nil
}

// fieldOf is the field a problem's path names: env_files of env_files[1], remembered of remembered.isolation.
func fieldOf(path string) string {
	if at := strings.IndexAny(path, "[."); at >= 0 {
		return path[:at]
	}

	return path
}

// request is the request the questions answer, each field as settled.
func (q Questions) request() Request {
	return Request{Given: q.Given(), Remembered: q.Remembered.Value}
}

// answered is values as compared and published: empty rather than nil.
func answered(values []string) []string {
	if values == nil {
		return []string{}
	}

	return values
}

// ----------------------------------------------------------------------------
// The plan

// A Change is one field config set changes: its path and its values before and after, spelled as the request spells them.
type Change struct {
	Path   string   `json:"path"`
	Before []string `json:"before"`
	After  []string `json:"after"`
}

// A Plan is what Apply writes: the fields that change, and the digest of the config.toml it replaces, which the fingerprint covers so a file changed on disk since the preview is a stale plan; then the recap.
type Plan struct {
	Changes  []Change `json:"changes"`
	Replaces string   `json:"replaces"`
	rewrite  project.Rewrite
	domain.Recapped
}

// Locks are none: config.toml is written under its own lock, the worktrees are left alone.
func (Plan) Locks() []domain.Lock {
	return nil
}

// Fingerprinted is what the plan's fingerprint covers: the changes and the file they replace, the recap left out.
func (p Plan) Fingerprinted() any {
	return struct {
		Changes  []Change
		Replaces string
	}{p.Changes, p.Replaces}
}

// A Detail is what config set wrote: nothing more than its item's status and the plan say.
type Detail struct{}

// subject is the one item of config set: the file it writes.
const subject = "config.toml"

// ----------------------------------------------------------------------------
// The recap

// labels name each field in the recap.
var labels = map[string]string{
	settings.PathBasePath:    "Worktrees folder",
	settings.PathBaseBranch:  "Base branch",
	settings.PathEnvStrategy: ".env from",
	settings.PathEnvFiles:    ".env files",
	settings.PathOnCreate:    "On create",
	settings.PathOnClean:     "On clean",
	PathRemembered:           "Remembered answers",
}

// recap is a row per field that changes, its new values.
func (p Plan) recap() []domain.RecapRow {
	rows := make([]domain.RecapRow, 0, len(p.Changes))
	for _, change := range p.Changes {
		values := domain.RecapValues(change.After)
		if len(values) == 0 {
			values = []domain.RecapValue{{Text: "none"}}
		}
		rows = append(rows, domain.RecapRow{Key: change.Path, Label: labels[change.Path], Path: change.Path, Values: values})
	}

	return rows
}

// ----------------------------------------------------------------------------
// The outcome

// CodeUndo is the follow-up that puts back what config set changed.
const CodeUndo = Name + ".undo"

// undo is config set again with each field it changed as it was, nil when it changed nothing.
func (p Plan) undo() *domain.FollowUp {
	if len(p.Changes) == 0 {
		return nil
	}

	answers := make([]domain.Answered, 0, len(p.Changes))
	for _, change := range p.Changes {
		answers = append(answers, domain.Answered{Path: change.Path, Values: change.Before})
	}
	return &domain.FollowUp{Command: Name, Answers: answers, Code: CodeUndo, Message: "put back what it held"}
}

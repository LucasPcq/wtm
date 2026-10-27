package domain

import (
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Origin says where an answer to a question came from.
type Origin string

// The origins of an answer.
const (
	OriginRequest    Origin = "request"    // a flag or the request
	OriginRemembered Origin = "remembered" // the answer remembered in config.toml
	OriginConfig     Origin = "config"     // a value of the project's configuration
	OriginDefault    Origin = "default"    // the question's default
)

// Origins returns every Origin.
func Origins() []Origin {
	return []Origin{OriginRequest, OriginRemembered, OriginConfig, OriginDefault}
}

// Skip says why a question is not asked, as a code each surface words its own way; empty when it is asked.
type Skip string

// The reasons a question is not asked.
const (
	SkipFlagOnly        Skip = "flag_only"        // only its flag sets it: a form does not ask it
	SkipExists          Skip = "exists"           // the target worktree exists: nothing is created
	SkipNotBehind       Skip = "not_behind"       // the source is not behind origin: nothing to fast-forward
	SkipHeld            Skip = "held"             // the source holds the changes being extracted: it is not fast-forwarded
	SkipRunUnread       Skip = "run_unread"       // run.toml could not be read
	SkipNothingIsolated Skip = "nothing_isolated" // run.toml declares nothing a worktree isolates
	SkipNoConflict      Skip = "no_conflict"      // nothing conflicts with the target
	SkipAll             Skip = "all"              // every worktree is read
	SkipNoOrphan        Skip = "no_orphan"        // no worktree removed leaves a child behind
	SkipSafe            Skip = "safe"             // nothing asked is unsafe: nothing to force
	SkipNoCandidate     Skip = "no_candidate"     // nothing to pick from: no worktree is done with
)

// Skips returns every Skip.
func Skips() []Skip {
	return []Skip{SkipFlagOnly, SkipExists, SkipNotBehind, SkipHeld, SkipRunUnread, SkipNothingIsolated, SkipNoConflict, SkipAll, SkipNoOrphan, SkipSafe, SkipNoCandidate}
}

// Answer is the value a question settled on, and where that value came from.
type Answer[T any] struct {
	Value  T
	Origin Origin
}

// Option is one value a choice offers, with what the repository knows of it: facts each surface words its own way, never copy.
type Option struct {
	Value    string           `json:"value"`
	Tag      OptionTag        `json:"tag,omitempty"`      // what sets it apart, empty when nothing does
	Base     bool             `json:"base,omitempty"`     // the configured base branch
	Detached bool             `json:"detached,omitempty"` // a worktree on no branch, valued by its path
	Origin   *worktree.Origin `json:"origin,omitempty"`   // where the branch stands against origin's, nil when origin has none
	Changes  int              `json:"changes,omitempty"`  // the uncommitted changes the worktree holds
	Change   *worktree.Change `json:"change,omitempty"`   // the uncommitted change it is
}

// OptionTag is what sets an option apart, as a code each surface words its own way.
type OptionTag string

// The tags of an option.
const (
	TagLocal      OptionTag = "local"       // a local branch no worktree holds
	TagCheckedOut OptionTag = "checked_out" // a local branch a worktree holds
	TagRemote     OptionTag = "remote"      // a branch only origin has
	TagCurrent    OptionTag = "current"     // the worktree the caller stands in
	TagMain       OptionTag = "main"        // the main worktree
)

// OptionTags returns every OptionTag.
func OptionTags() []OptionTag {
	return []OptionTag{TagLocal, TagCheckedOut, TagRemote, TagCurrent, TagMain}
}

// Choice is a question with a closed set of options: its answer, its options, the reason it is not asked (Skip, empty when it is), and the answer remembered for it.
type Choice[T ~string] struct {
	Answer[T]
	Options    []T
	Skip       Skip
	Remembered T
}

// Confirm is a yes-or-no question: its answer, the reason it is not asked (Skip, empty when it is), and the answer remembered for it (nil when none).
type Confirm struct {
	Answer[bool]
	Skip       Skip
	Remembered *bool
}

// CodeOneOf is the field-error code of a value that is not one of those its field accepts.
const CodeOneOf = "one_of"

// OneOfParams is a value given to a field with a closed set of accepted values.
type OneOfParams struct {
	Path     string
	Value    string
	Accepted []string
}

// OneOf is the problem of a value that is not one of those accepted, as a field error; an empty value is none.
func OneOf(params OneOfParams) []FieldError {
	if params.Value == "" || slices.Contains(params.Accepted, params.Value) {
		return nil
	}

	return []FieldError{{
		Path:     params.Path,
		Code:     CodeOneOf,
		Template: "{path} must be one of {accepted}, not {value}",
		Params:   Params{"value": params.Value},
		Accepted: params.Accepted,
	}}
}

// Names spells each of values as a string, in order: the accepted values of a field, for its validation and its completion.
func Names[T ~string](values []T) []string {
	found := make([]string, 0, len(values))
	for _, value := range values {
		found = append(found, string(value))
	}

	return found
}

// OptionsOf are values as the options of a choice, in order, with no fact of their own.
func OptionsOf[T ~string](values []T) []Option {
	options := make([]Option, 0, len(values))
	for _, value := range values {
		options = append(options, Option{Value: string(value)})
	}

	return options
}

// A WorktreeQuery is how a request names its worktree: Name, by its branch, a part of its branch or a path ("" names none); Located, the path Name stands for, resolved ("" when it stands for none); Cwd, where the caller stands.
type WorktreeQuery struct {
	Name    string
	Located string
	Cwd     string
}

// WorktreeOptions are worktrees as a worktree picker offers them for q, in order: each one, or only those an ambiguous name matches; each by its name (worktree.Worktree.Name), tagged current when it is the deepest holding q.Cwd, else main for the main worktree.
func WorktreeOptions(worktrees []worktree.Worktree, q WorktreeQuery) []Option {
	if matches := worktree.Named(worktrees, q.Name, q.Located); len(matches) > 1 {
		worktrees = matches
	}
	current := worktree.Holding(worktrees, q.Cwd, func(w worktree.Worktree) string { return w.Path })
	options := make([]Option, 0, len(worktrees))
	for i, w := range worktrees {
		option := Option{Value: w.Name(), Detached: w.Branch() == ""}
		switch {
		case i == current:
			option.Tag = TagCurrent
		case w.Main:
			option.Tag = TagMain
		}
		options = append(options, option)
	}

	return options
}

// TargetWorktree is the worktree of worktrees, none empty, that q names (worktree.Named); else the deepest holding q.Cwd, where the caller stands; else the first. It refuses a name that names no worktree (CodeNotFound) or several (CodeAmbiguous, their branches in its params), each a problem of the field at PathWorktree.
func TargetWorktree(worktrees []worktree.Worktree, q WorktreeQuery) (worktree.Worktree, error) {
	if q.Name != "" {
		return namedWorktree(worktrees, q)
	}
	if at := worktree.Holding(worktrees, q.Cwd, func(w worktree.Worktree) string { return w.Path }); at >= 0 {
		return worktrees[at], nil
	}
	return worktrees[0], nil
}

func namedWorktree(worktrees []worktree.Worktree, q WorktreeQuery) (worktree.Worktree, error) {
	matches := worktree.Named(worktrees, q.Name, q.Located)
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return worktree.Worktree{}, Invalid([]FieldError{{Path: PathWorktree, Code: CodeNotFound, Template: "{value} names no worktree", Params: Params{"value": q.Name}}})
	}

	names := make([]string, 0, len(matches))
	for _, w := range matches {
		names = append(names, w.Branch())
	}
	return worktree.Worktree{}, Invalid([]FieldError{{Path: PathWorktree, Code: CodeAmbiguous, Template: "{value} matches several worktrees: {matches}", Params: Params{"value": q.Name, "matches": strings.Join(names, ", ")}}})
}

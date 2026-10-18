package cli

import (
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/form"

	tea "github.com/charmbracelet/bubbletea"
)

// createTarget is the value the target list gives its option with no value, a worktree to create.
const createTarget = "\x00new-worktree"

// special returns the steps extract asks its own way: the file picker, and the target, picked as a worktree then, for one to create, typed as its branch; both answer to.
func special(s *session, at int, field domain.Field) form.Step {
	switch field := field.(type) {
	case domain.MultiSelectField:
		files := &filesStep{MultiSelectStep: s.MultiSelectStep(at), s: s}
		files.Tone = files.tone
		return files
	case domain.SelectField:
		if field.Path == extract.PathTo {
			return &targetStep{Asked: s.Asked(at), s: s, at: at}
		}
	case domain.TextField:
		for before := range at {
			if target, isTarget := s.Step(before).(*targetStep); isTarget {
				branch := &branchStep{TextStep: s.TextStep(at), s: s, at: at, target: target}
				branch.Check = branch.check
				return branch
			}
		}
	case domain.ConfirmField, domain.TextListField:
	}

	return nil
}

// ----------------------------------------------------------------------------
// The files

// filesStep asks the changes to extract, each tagged with its status in its tone, the trail counting the changes picked.
type filesStep struct {
	*form.MultiSelectStep
	s *session
}

// Standing counts the changes the files answered stand for, a directory standing for every change under it.
func (f *filesStep) Standing() form.Standing {
	st := f.MultiSelectStep.Standing()
	st.Value = ""

	files := f.s.Request().Files
	if len(files) == 0 {
		return st
	}

	selected, missed := worktree.SelectChanges(f.s.Questions().Files.Options, files)
	if missed != "" {
		st.Value = fileCount(len(files))
		return st
	}

	st.Value = fileCount(len(selected))
	return st
}

// tone is the colour of a change's tag: an addition, a loss, a risk, or a rename that changes no content.
func (f *filesStep) tone(option domain.Option) form.Tone {
	at := slices.IndexFunc(f.s.Questions().Files.Options, func(c worktree.Change) bool { return c.Path == option.Value })
	if at < 0 {
		return form.ToneMuted
	}

	switch f.s.Questions().Files.Options[at].Status {
	case worktree.ChangeUntracked:
		return form.ToneSuccess
	case worktree.ChangeDeleted:
		return form.ToneDanger
	case worktree.ChangeModified:
		return form.ToneWarning
	case worktree.ChangeRenamed:
	}

	return form.ToneMuted
}

// Answer records the changes picked as the request's files, the trail counting them.
func (f *filesStep) Answer() string {
	f.MultiSelectStep.Answer()
	return fileCount(len(f.s.Request().Files))
}

// ----------------------------------------------------------------------------
// The target

// targetStep asks the target among the worktrees, or a worktree to create, whose branch the branch step asks next.
type targetStep struct {
	form.Asked
	s         *session
	at        int
	list      *form.SelectList
	newTarget bool // the user picked a worktree to create
}

// Standing is the target answered, "new worktree" for one to create; its problems are the branch step's once one is created.
func (t *targetStep) Standing() form.Standing {
	st := t.Asked.Standing()
	st.Erred = st.Erred && !t.creating()
	if t.creating() {
		st.Value = t.created()
	}

	return st
}

// created is how the trail names a worktree to create: the Short of the target's option with no value.
func (t *targetStep) created() string {
	field, _ := t.s.Field(t.at).(domain.SelectField)
	at := slices.IndexFunc(field.Options, func(o domain.Option) bool { return o.Value == "" })
	if at < 0 {
		return ""
	}

	return field.Options[at].Short
}

// Open lists the worktrees after the option to create one, picked by the sentinel createTarget, set apart by a rule.
func (t *targetStep) Open() tea.Cmd {
	field, _ := t.s.Field(t.at).(domain.SelectField)
	options := form.Options(field.Options)
	if at := slices.IndexFunc(field.Options, func(o domain.Option) bool { return o.Value == "" }); at >= 0 {
		options[at].Value = createTarget
		options = slices.Insert(options, at+1, form.Option{Separator: true})
	}

	start := t.s.Request().To
	if t.creating() {
		start = createTarget
	}

	t.list = form.NewSelectList(form.SelectParams{Options: options, Start: start})
	return nil
}

// Field is the select list.
func (t *targetStep) Field() form.Field {
	return t.list
}

// Answer records an existing worktree as the target, or a worktree to create, keeping the branch already given to one.
func (t *targetStep) Answer() string {
	value := t.list.Value()
	if value != createTarget {
		t.newTarget = false
		t.s.Answer(t.at, domain.Answered{Path: extract.PathTo, Values: []string{value}})
		return value
	}

	if t.creating() {
		t.s.Answer(t.at)
	} else {
		t.s.Answer(t.at, domain.Answered{Path: extract.PathTo, Values: []string{""}})
	}
	t.newTarget = true
	return t.created()
}

// Reset forgets the target picked.
func (t *targetStep) Reset() {
	t.newTarget = false
	t.Asked.Reset()
}

// creating reports whether the target is a worktree to create: picked as one, or a branch that no worktree other than the source holds.
func (t *targetStep) creating() bool {
	req := t.s.Request()
	held := slices.ContainsFunc(t.s.Questions().To.Options, func(o domain.Option) bool { return o.Value == req.To })
	return t.newTarget || req.To != "" && req.To != req.Source && !held
}

// ----------------------------------------------------------------------------
// The branch of a created target

// branchStep asks the branch of a created target, refusing one a worktree already holds before the repository checks it.
type branchStep struct {
	*form.TextStep
	s      *session
	at     int
	target *targetStep
}

// Standing is the branch answered; its flag and its problems are the target's own while an existing worktree is picked.
func (b *branchStep) Standing() form.Standing {
	st := b.TextStep.Standing()
	creating := b.target.creating()
	st.Erred = st.Erred && creating
	if !creating {
		st.Given, st.Flag = false, ""
	}

	return st
}

// check refuses a branch a worktree already holds, then checks it against the repository.
func (b *branchStep) check(branch string) error {
	label := b.Label()
	if held := b.s.Questions().NewTarget(branch); len(held) > 0 {
		return form.Problem(held[0], label)
	}

	questions, err := b.s.Check(domain.Answered{Path: extract.PathTo, Values: []string{branch}})
	if err != nil {
		return form.Sentence(err)
	}

	for _, found := range questions.Errors {
		if found.Path == extract.PathTo {
			return form.Problem(found, label)
		}
	}

	return nil
}

func fileCount(count int) string {
	if count == 1 {
		return "1 file"
	}

	return strconv.Itoa(count) + " files"
}

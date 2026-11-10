// Package configedit opens config.toml in the user's editor, then checks what the editor left: a file that does not parse or holds a setting wtm cannot use is kept as edited, its problems said, and the edit offered again.
package configedit

import (
	"context"
	"errors"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `config edit`.
const Name = domain.CommandConfigEdit

// The codes of a repository whose config.toml is missing, or that does not parse once edited, and of an editor that failed.
const (
	CodeConfigNotFound = Name + domain.ConfigNotFound
	CodeConfigInvalid  = Name + domain.ConfigInvalid
	CodeEditorFailed   = Name + ".editor_failed"
)

// A UseCase opens config.toml, at Path, in Editor, then reads it through Settings and the branches through Git; Init is the command that sets wtm up, offered when there is no config.toml.
type UseCase struct {
	Path     string
	Editor   Editor
	Settings settings.Service
	Git      Git
	Init     *domain.FollowUp
}

// An Outcome is what Run found once the editor was closed: one Item, config.toml, failed with a warning per problem it holds.
type Outcome = domain.Read[Detail]

// An Item is config.toml as the editor left it.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run opens config.toml in the editor, then checks it; it takes no lock, the editor being the user's. An error returned is a config.toml missing before or after, an editor that failed, or a file that no longer parses.
func (u UseCase) Run(ctx context.Context, _ Request, _ domain.RunOptions) (Outcome, error) {
	// 1. Refuse a repository with no config.toml; one that does not parse is opened, to be fixed.
	saved, err := u.Settings.Read()
	if _, invalid := errors.AsType[*project.InvalidConfigError](err); err != nil && !invalid {
		return Outcome{}, err
	}
	if saved == nil && err == nil {
		return Outcome{}, u.notFound()
	}

	// 2. Open it in the editor, until it is closed.
	if err := u.Editor(ctx, u.Path); err != nil {
		return Outcome{}, domain.AsError(domain.AsErrorParams{Err: err, Code: CodeEditorFailed, Message: "couldn't open the editor"})
	}

	// 3. Read what the editor left, and report it.
	facts, err := u.Observe(ctx)
	if err != nil {
		return Outcome{}, err
	}
	return Report(facts), nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is config edit's form: it asks nothing.
func (u UseCase) Questions(context.Context, Request) (Questions, error) {
	return Questions{}, nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads config.toml as the editor left it, refusing none or one that does not parse, followed by the edit that fixes it, and the branches.
func (u UseCase) Observe(ctx context.Context) (Facts, error) {
	saved, err := u.Settings.Read()
	if invalid, ok := errors.AsType[*project.InvalidConfigError](err); ok {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigInvalid, Message: "invalid config.toml, kept as edited: " + invalid.Reason.Error(), FollowUp: editAgain(), Cause: err}
	}
	if err != nil {
		return Facts{}, err
	}
	if saved == nil {
		return Facts{}, u.notFound()
	}

	branches, err := u.Git.Branches(ctx)
	if err != nil {
		return Facts{}, err
	}
	var names []string
	for _, branch := range slices.Concat(branches.Local, branches.Remote) {
		if !slices.Contains(names, branch.Name) {
			names = append(names, branch.Name)
		}
	}
	return Facts{Saved: *saved, Branches: names}, nil
}

// ----------------------------------------------------------------------------
// Helpers

func (u UseCase) notFound() error {
	var followUp *domain.FollowUp
	if u.Init != nil {
		init := *u.Init
		init.Code, init.Message = "repository.init", "set wtm up in this repository"
		followUp = &init
	}
	return &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigNotFound, Message: "no wtm config found", FollowUp: followUp, Cause: project.ErrConfigNotFound}
}

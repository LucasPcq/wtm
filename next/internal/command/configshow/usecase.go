// Package configshow reads config.toml as a document: each setting with its value, where it comes from, its default and the values it takes, the problems it holds, the answers it remembers; a read, it changes nothing.
package configshow

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `config show`.
const Name = "config-show"

// The codes of a repository whose config.toml is missing or does not parse.
const (
	CodeConfigNotFound = Name + domain.ConfigNotFound
	CodeConfigInvalid  = Name + domain.ConfigInvalid
)

// A UseCase reads config.toml through Settings and the branches through Git; Init is the command that sets wtm up, offered when there is no config.toml, and Edit the one that opens it in an editor, offered when it does not parse.
type UseCase struct {
	Settings settings.Service
	Git      Git
	Init     *domain.FollowUp
	Edit     *domain.FollowUp
}

// An Outcome is what Run read: one Item, config.toml, and a warning per problem it holds.
type Outcome = domain.Read[Detail]

// An Item is config.toml as read.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run reads config.toml; a read, it takes no lock and runs no operation. An error returned is a config.toml that is missing or does not parse.
func (u UseCase) Run(ctx context.Context, _ Request, _ domain.RunOptions) (Outcome, error) {
	// 1. Read config.toml as written, and the branches.
	facts, err := u.Observe(ctx)
	if err != nil {
		return Outcome{}, err
	}

	// 2. Report it as a document.
	return Report(facts), nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is config show's form: it asks nothing.
func (u UseCase) Questions(context.Context, Request) (Questions, error) {
	return Questions{}, nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads config.toml as written, refusing none or one that does not parse, and the branches.
func (u UseCase) Observe(ctx context.Context) (Facts, error) {
	saved, err := u.Settings.Read()
	if invalid, ok := errors.AsType[*project.InvalidConfigError](err); ok {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigInvalid, Message: "invalid config.toml: " + invalid.Reason.Error(), Params: where(invalid), FollowUp: offered(u.Edit, Name+".edit", "fix it in your editor"), Cause: err}
	}
	if err != nil {
		return Facts{}, err
	}
	if saved == nil {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigNotFound, Message: "no wtm config found", FollowUp: offered(u.Init, "repository.init", "set wtm up in this repository"), Cause: project.ErrConfigNotFound}
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

// offered is the follow-up injected, coded and worded by this command; none when nil.
func offered(followUp *domain.FollowUp, code, message string) *domain.FollowUp {
	if followUp == nil {
		return nil
	}
	offer := *followUp
	offer.Code, offer.Message = code, message
	return &offer
}

// where is where the parse of config.toml stopped, as the params of its refusal: its line and column, the key, and the setting that key is, those it says.
func where(invalid *project.InvalidConfigError) domain.Params {
	params := domain.Params{}
	if invalid.Line > 0 {
		params["line"], params["column"] = strconv.Itoa(invalid.Line), strconv.Itoa(invalid.Column)
	}
	if invalid.Key != "" {
		params["key"] = invalid.Key
	}
	if setting := settingOf(invalid.Key); setting != "" {
		params["setting"] = setting
	}

	return params
}

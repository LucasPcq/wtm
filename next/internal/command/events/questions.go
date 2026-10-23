package events

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of the stream: the repository holding the path Repo, read from where the caller stands when relative, or every repository wtm knows with All, else the repository of the directory the caller stands on.
// A caller that stands nowhere, or on a directory it did not name ($WTM_PROJECT_DIR unset on the command line) in no repository, watches every one instead. Caller is no field: every surface sets it.
type Request struct {
	Repo   string        `json:"repo"`
	All    bool          `json:"all"`
	Caller domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

const (
	pathRepo = "repo"
	pathAll  = "all"
)

// CodeRepoWithAll refuses --repo with --all: the one repository of the first is among every one of the second.
const CodeRepoWithAll = Name + ".repo_with_all"

// CodeNotADirectory refuses a --repo that names no directory.
const CodeNotADirectory = Name + ".not_a_directory"

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case pathRepo:
		next.Repo, err = domain.One[string](path, values)
	case pathAll:
		next.All, err = domain.Yes(path, values)
	default:
		return r, domain.Unknown(path)
	}

	if err != nil {
		return r, err
	}
	return next, nil
}

// Values is the request's answer to the field at path, nil when it leaves the field to its default.
func (r Request) Values(path string) []string {
	switch path {
	case pathRepo:
		return domain.Spelled(r.Repo)
	case pathAll:
		return domain.SaidYes(r.All)
	}

	return nil
}

// Validate returns the problem of a request that names one repository and every one at once.
func (r Request) Validate() error {
	if !r.All || r.Repo == "" {
		return nil
	}

	return domain.Invalid([]domain.FieldError{{Path: pathRepo, Code: CodeRepoWithAll, Template: "{path} watches one repository, all of them already: give one or the other", With: []string{pathAll}}})
}

// relativeFromNowhere is the problem of a relative --repo from a caller in no directory to read it from: a surface's call naming none, or a command line whose working directory is gone.
func relativeFromNowhere(repo string) error {
	return domain.Invalid([]domain.FieldError{{Path: pathRepo, Code: CodeNotADirectory, Template: "{path} {value} is relative, and the caller stands in no directory to read it from", Params: domain.Params{"value": repo}}})
}

// notADirectory is the problem of a --repo that names no directory.
func notADirectory(repo string) error {
	return domain.Invalid([]domain.FieldError{{Path: pathRepo, Code: CodeNotADirectory, Template: "{path} {value} is not a directory", Params: domain.Params{"value": repo}}})
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form; the stream asks nothing, so its form only names its flags.
type Questions struct{}

// Form is the schema of the request: the repository to watch instead of the current one, or every one.
func (Questions) Form() domain.Form {
	return domain.Form{Fields: []domain.Field{
		domain.TextField{
			FieldSpec: domain.FieldSpec{Path: pathRepo, Flag: "--repo", Label: "Repository", Help: "Watch the repository holding this path instead of the current one"},
		},
		domain.ConfirmField{
			FieldSpec: domain.FieldSpec{Path: pathAll, Flag: "--all", Label: "Every repository", Help: "Watch every repository wtm knows, whatever the current directory or $GIT_DIR"},
		},
	}}
}

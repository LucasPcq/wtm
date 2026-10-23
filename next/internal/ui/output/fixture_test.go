package output

import (
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
)

var flags = Flags{"branches": "<branches>", "from": "--from", "env_from": "--env-from"}

func invalid() *domain.Error {
	return domain.Invalid([]domain.FieldError{
		{Path: "branches[0]", Code: "create.checked_out_elsewhere", Template: "branch {branch} is checked out at {path} — run `wtm go {branch}` to jump in", Params: domain.Params{"branch": "feat/x", "path": "/p"}},
		{Path: "branches[1]", Code: "distinct", Template: "{path}: {value} is given twice", Params: domain.Params{"value": "feat/x"}},
		{Path: "env_from", Code: "one_of", Template: "{path} must be one of {accepted}, not {value}", Params: domain.Params{"value": "copy"}, Accepted: []string{"example", "main", "parent"}},
		{Path: "from", Code: "required", Template: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)", Params: domain.Params{"because": "create.parent_required"}},
	})
}

// failingWriter fails every write, as a stream whose reader left does.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("reader gone") }

// recappedPlan is a plan as DrawApplied reads it.
type recappedPlan struct{ domain.Recapped }

// worktreeDetail is the detail of an item's worktree.
type worktreeDetail struct{ path string }

func (d worktreeDetail) Worktree() (string, env.Redacted) {
	return d.path, env.Redacted{}
}

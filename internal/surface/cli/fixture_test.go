package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
	"github.com/LucasPcq/wtm/internal/styles"
)

// The tests of the generated command line share one example, shaped like
// `wtm create`: a request with a field of each kind the flags are generated
// from, the facts read from a fake repository that is also the disk Apply
// writes to, the command itself, and a query beside it.

const (
	commandName  = "make"
	queryName    = "inventory"
	isolated     = "isolated"
	verbatim     = "verbatim"
	hookLine     = "installing dependencies"
	worktreeStep = "worktree"
	failing      = "feat/boom"
	worktreeRoot = "/wt/"
)

const codeBranchTaken kernel.Code = "branch.taken"

var errStep = errors.New("git worktree add: exit status 128")

var (
	makeMessages  = text.Messages{codeBranchTaken: "{branch} is taken"}
	makeExitCodes = map[kernel.Code]int{codeBranchTaken: domain.ExitCodeWorktreeExists}
)

type target struct {
	Base string `json:"base"`
}

type request struct {
	Branches    []string          `json:"branches"`
	From        string            `json:"from"`
	Isolation   string            `json:"isolation"`
	FastForward bool              `json:"fast_forward"`
	Env         map[string]string `json:"env"`
	Target      target            `json:"target"`
}

type facts struct {
	Existing   []string `json:"existing"`
	Remembered string   `json:"remembered,omitempty"`
}

type plan struct {
	Branches  []string `json:"branches"`
	Isolation string   `json:"isolation"`
}

type inventory struct {
	Worktrees []string `json:"worktrees"`
}

// repo is the fake repository: what Observe reads, what Apply changes.
type repo struct {
	mu         sync.Mutex
	existing   []string
	remembered string
	worktrees  []string
	applyErr   error
	taken      string
}

func (r *repo) observe() facts {
	r.mu.Lock()
	defer r.mu.Unlock()
	return facts{Existing: slices.Clone(r.existing), Remembered: r.remembered}
}

func (r *repo) add(branch string) func(context.Context) error {
	return func(context.Context) error {
		if branch == failing {
			return errStep
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		r.worktrees = append(r.worktrees, branch)
		return nil
	}
}

func (r *repo) remove(branch string) func(context.Context) error {
	return func(context.Context) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.worktrees = slices.DeleteFunc(r.worktrees, func(name string) bool { return name == branch })
		return nil
	}
}

func (r *repo) listed() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.worktrees)
}

// exists changes the facts, as another process would between a preview and
// its run.
func (r *repo) exists(branch string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.existing = append(r.existing, branch)
}

func makeCommand(r *repo) kernel.Command[request, facts, plan, string] {
	return kernel.Command[request, facts, plan, string]{
		Name: commandName,
		Observe: func(context.Context, kernel.ObserveScope) (facts, error) {
			return r.observe(), nil
		},
		Forget: func(f facts) facts {
			f.Remembered = ""
			return f
		},
		Fields: []kernel.FieldDef[request, facts]{
			{Spec: kernel.FieldSpec{Path: "branches", Type: kernel.FieldTextList, Label: "Branches", Description: "The branches to make a worktree for", Required: true}},
			{Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect, Label: "From", Description: "The branch a new branch starts from"}},
			{
				Spec: kernel.FieldSpec{Path: "isolation", Type: kernel.FieldSelect, Label: "Isolation", Description: "How the worktree's ports are isolated", Rememberable: true, Constraints: kernel.Constraints{Enum: []string{isolated, verbatim}}},
				Default: func(_ request, f facts) (kernel.Fallback, bool) {
					if f.Remembered != "" {
						return kernel.Fallback{Value: kernel.Text(f.Remembered), Origin: kernel.OriginRemembered}, true
					}
					return kernel.Fallback{Value: kernel.Text(isolated), Origin: kernel.OriginDefault}, true
				},
			},
			{Spec: kernel.FieldSpec{Path: "fast_forward", Type: kernel.FieldBool, Label: "Fast-forward", Description: "Fast-forward the base branch first"}},
			{Spec: kernel.FieldSpec{Path: "env", Type: kernel.FieldDecisions, Label: "Env"}},
			{Spec: kernel.FieldSpec{Path: "target.base", Type: kernel.FieldText, Label: "Base", Description: "The base branch"}},
		},
		Rules: kernel.Rules[request, facts]{}.
			Distinct("branches").
			Exclusive("from", "target.base"),
		Locks: func(req request) []kernel.LockKey {
			keys := []kernel.LockKey{{Scope: kernel.LockRepo, Mode: kernel.LockShared}}
			for _, branch := range req.Branches {
				keys = append(keys, kernel.LockKey{Scope: kernel.LockWorktree, Name: branch, Mode: kernel.LockExclusive})
			}
			return keys
		},
		Plan: func(_ context.Context, req request, _ facts) (kernel.Plan[plan], error) {
			return kernel.Plan[plan]{Detail: plan{Branches: req.Branches, Isolation: req.Isolation}}, nil
		},
		Apply: func(ctx context.Context, in kernel.ApplyInput[request, facts, plan]) (kernel.Outcome[string], error) {
			if r.applyErr != nil {
				return kernel.Outcome[string]{}, r.applyErr
			}
			if r.taken != "" && slices.Contains(in.Request.Branches, r.taken) {
				return kernel.Outcome[string]{}, kernel.Conflict(kernel.Problem{Code: codeBranchTaken, Params: kernel.Params{"branch": r.taken}})
			}
			items := kernel.Each(ctx, kernel.EachParams[string, string]{
				Items:   in.Plan.Detail.Branches,
				Subject: func(branch string) string { return branch },
				Unit: func(branch string) kernel.Unit[string] {
					return kernel.Unit[string]{
						Saga: kernel.Saga[string]{
							Steps:  []kernel.SagaStep{{Name: worktreeStep, Do: r.add(branch), Undo: r.remove(branch)}},
							Commit: func(context.Context) (string, error) { return worktreeRoot + branch, nil },
						},
						Then: []kernel.Phase[string]{{
							Name: string(domain.HookOnCreate),
							Run: func(_ context.Context, path string) (string, error) {
								kernel.Report(in.Emit, branch).Output(hookLine)
								return path, nil
							},
						}},
					}
				},
				Emit:   in.Emit,
				Shield: in.Shield,
			})
			return kernel.Outcome[string]{
				Items:     items,
				FollowUps: []kernel.FollowUp{{Command: commandName, Request: []byte(`{"branches":["feat/next"],"fast_forward":true}`), Code: kernel.CodeInterrupted}},
			}, nil
		},
	}
}

func inventoryQuery(r *repo) kernel.Query[struct{}, inventory] {
	return kernel.Query[struct{}, inventory]{
		Name: queryName,
		Run: func(context.Context, struct{}) (inventory, error) {
			return inventory{Worktrees: r.listed()}, nil
		},
	}
}

// pathDetail is a command's own rendering: the plan's branches, and the path
// of each worktree made.
type pathDetail struct{}

func (pathDetail) Plan(detail json.RawMessage) []styles.PlanRow {
	var p plan
	_ = json.Unmarshal(detail, &p)
	return []styles.PlanRow{{Label: "Worktrees", Values: []string{fmt.Sprintf("%d to make", len(p.Branches))}}}
}

func (pathDetail) Item(w io.Writer, item kernel.Item[json.RawMessage]) {
	var path string
	_ = json.Unmarshal(item.Detail, &path)
	fmt.Fprintf(w, "    %s  %s\n", item.Subject, path)
}

package dispatch_test

import (
	"context"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
)

// The tests share one example, shaped like `wtm create`: a request naming
// branches to create, the facts read from a fake repository that is also the
// disk Apply writes to, and the command itself, declared at the end.

const (
	commandName  = "create"
	isolated     = "isolated"
	verbatim     = "verbatim"
	hookLine     = "installing dependencies"
	worktreeStep = "worktree"
)

const codeBranchTaken kernel.Code = "branch.taken"

type request struct {
	Branches  []string `json:"branches"`
	From      string   `json:"from"`
	Isolation string   `json:"isolation"`
}

type facts struct {
	Existing   []string `json:"existing"`
	Remembered string   `json:"remembered,omitempty"`
}

type plan struct {
	Branches  []string `json:"branches"`
	Isolation string   `json:"isolation"`
}

// repo is the fake repository: what Observe reads, what Apply changes. When
// hold is set, Apply says so on entered and waits for hold to close.
type repo struct {
	mu         sync.Mutex
	existing   []string
	remembered string
	worktrees  []string
	applyErr   error
	hold       chan struct{}
	entered    chan struct{}
}

func (r *repo) observe() facts {
	r.mu.Lock()
	defer r.mu.Unlock()
	return facts{Existing: slices.Clone(r.existing), Remembered: r.remembered}
}

func (r *repo) add(branch string) func(context.Context) error {
	return func(context.Context) error {
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

// held makes the next Apply wait until release is called.
func (r *repo) held() (entered <-chan struct{}, release func()) {
	r.hold, r.entered = make(chan struct{}), make(chan struct{})
	var once sync.Once
	return r.entered, func() { once.Do(func() { close(r.hold) }) }
}

func createCommand(r *repo) kernel.Command[request, facts, plan, string] {
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
			{Spec: kernel.FieldSpec{Path: "branches", Type: kernel.FieldTextList, Label: "Branches", Required: true}},
			{
				Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect, Label: "From", Choices: kernel.ChoicesDynamic},
				Choices: func(_ context.Context, _ request, f facts) (kernel.Choices, error) {
					options := []kernel.Option{{Value: "main", Label: "main"}}
					for _, branch := range f.Existing {
						options = append(options, kernel.Option{Value: branch, Label: branch})
					}
					return kernel.Choices{Options: options}, nil
				},
			},
			{
				Spec: kernel.FieldSpec{Path: "isolation", Type: kernel.FieldSelect, Label: "Isolation", Rememberable: true, Constraints: kernel.Constraints{Enum: []string{isolated, verbatim}}},
				Default: func(_ request, f facts) (kernel.Fallback, bool) {
					if f.Remembered != "" {
						return kernel.Fallback{Value: kernel.Text(f.Remembered), Origin: kernel.OriginRemembered}, true
					}
					return kernel.Fallback{Value: kernel.Text(isolated), Origin: kernel.OriginDefault}, true
				},
			},
		},
		Rules: kernel.Rules[request, facts]{}.
			Distinct("branches").
			NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "branches"}),
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
			if r.hold != nil {
				close(r.entered)
				<-r.hold
			}
			items := kernel.Each(ctx, kernel.EachParams[string, string]{
				Items:   in.Plan.Detail.Branches,
				Subject: func(branch string) string { return branch },
				Unit: func(branch string) kernel.Unit[string] {
					return kernel.Unit[string]{
						Saga: kernel.Saga[string]{
							Steps:  []kernel.SagaStep{{Name: worktreeStep, Do: r.add(branch), Undo: r.remove(branch)}},
							Commit: func(context.Context) (string, error) { return "/wt/" + branch, nil },
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
			return kernel.Outcome[string]{Items: items}, nil
		},
	}
}

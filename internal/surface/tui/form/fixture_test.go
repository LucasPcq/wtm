package form

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/kernel"
)

// The tests share one example, shaped like `wtm create` with a field of every
// type: a request, the facts read from a fake repository that counts what it
// is asked and is also the disk Apply writes to, and the command itself.

const (
	commandName = "make"
	base        = "main"
	isolated    = "isolated"
	verbatim    = "verbatim"
	copied      = "copy"
	linked      = "link"
	envFile     = ".env"
	existsBadge = kernel.Code("test.exists")
	noPush      = kernel.Code("test.no_push")
)

var errListing = errors.New("listing failed")

type request struct {
	Branches  []string          `json:"branches"`
	From      string            `json:"from"`
	Isolation string            `json:"isolation"`
	Push      bool              `json:"push"`
	Message   string            `json:"message"`
	Hooks     []string          `json:"hooks"`
	Order     []string          `json:"order"`
	Env       map[string]string `json:"env"`
	Remember  []string          `json:"remember"`
}

type facts struct {
	Existing   []string `json:"existing"`
	Remembered string   `json:"remembered,omitempty"`
}

type plan struct {
	Branches []string `json:"branches"`
	From     string   `json:"from"`
}

// repo is the fake repository. It counts every Observe and every Choices.
type repo struct {
	mu         sync.Mutex
	existing   []string
	remembered string
	worktrees  []string
	observes   int
	choices    map[string]int
	choicesErr error
	applied    []request
}

func (r *repo) observe() facts {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observes++
	return facts{Existing: slices.Clone(r.existing), Remembered: r.remembered}
}

// changed replaces the branches under the lock: the form observes from its own
// goroutine while a test changes the repository.
func (r *repo) changed(existing ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.existing = existing
}

func (r *repo) asked(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.choices == nil {
		r.choices = map[string]int{}
	}
	r.choices[path]++
}

// io is how many times the command did I/O: observed, or listed choices.
func (r *repo) io() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	total := r.observes
	for _, count := range r.choices {
		total += count
	}
	return total
}

func (r *repo) add(branch string) func(context.Context) error {
	return func(context.Context) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.worktrees = append(r.worktrees, branch)
		return nil
	}
}

func fields(r *repo) []kernel.FieldDef[request, facts] {
	return []kernel.FieldDef[request, facts]{
		{
			Spec: kernel.FieldSpec{Path: "branches", Type: kernel.FieldTextList, Label: "Branches", Required: true, Choices: kernel.ChoicesDynamic, Constraints: kernel.Constraints{Pattern: `^[a-z0-9/-]+$`}},
			Choices: func(_ context.Context, _ request, f facts) (kernel.Choices, error) {
				r.asked("branches")
				options := make([]kernel.Option, 0, len(f.Existing))
				for _, branch := range f.Existing {
					options = append(options, kernel.Option{Value: branch, Badges: []kernel.Badge{{Code: existsBadge}}})
				}
				return kernel.Choices{Options: options}, nil
			},
		},
		{
			Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect, Label: "Source", Choices: kernel.ChoicesSearch, DependsOn: []string{"branches"}},
			Default: func(request, facts) (kernel.Fallback, bool) {
				return kernel.Fallback{Value: kernel.Text(base), Origin: kernel.OriginConfig}, true
			},
			Choices: func(_ context.Context, _ request, f facts) (kernel.Choices, error) {
				r.asked("from")
				if r.choicesErr != nil {
					return kernel.Choices{}, r.choicesErr
				}
				options := []kernel.Option{{Value: base}}
				for _, branch := range f.Existing {
					options = append(options, kernel.Option{Value: branch})
				}
				return kernel.Choices{Options: options, Pinned: base}, nil
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
		{Spec: kernel.FieldSpec{Path: "push", Type: kernel.FieldBool, Label: "Push"}},
		{
			Spec: kernel.FieldSpec{Path: "message", Type: kernel.FieldText, Label: "Message", Required: true, DependsOn: []string{"push"}},
			Skip: func(req request, _ facts) (bool, kernel.Code) { return !req.Push, noPush },
		},
		{
			Spec: kernel.FieldSpec{Path: "hooks", Type: kernel.FieldMultiSelect, Label: "Hooks", Choices: kernel.ChoicesStatic, Constraints: kernel.Constraints{Enum: []string{"install", "migrate"}}},
			Choices: func(context.Context, request, facts) (kernel.Choices, error) {
				r.asked("hooks")
				return kernel.Choices{Options: []kernel.Option{{Value: "install"}, {Value: "migrate"}}}, nil
			},
		},
		{
			Spec: kernel.FieldSpec{Path: "order", Type: kernel.FieldReorder, Label: "Order"},
			Default: func(request, facts) (kernel.Fallback, bool) {
				return kernel.Fallback{Value: kernel.List{"web", "api"}, Origin: kernel.OriginConfig}, true
			},
		},
		{
			Spec: kernel.FieldSpec{Path: "env", Type: kernel.FieldDecisions, Label: "Env files", Constraints: kernel.Constraints{Enum: []string{copied, linked}}},
			Default: func(request, facts) (kernel.Fallback, bool) {
				return kernel.Fallback{Value: kernel.Decisions{envFile: copied}, Origin: kernel.OriginDefault}, true
			},
		},
	}
}

func makeCommand(r *repo) kernel.Command[request, facts, plan, string] {
	defs := fields(r)
	return kernel.Command[request, facts, plan, string]{
		Name: commandName,
		Observe: func(context.Context, kernel.ObserveScope) (facts, error) {
			return r.observe(), nil
		},
		Forget: func(f facts) facts {
			f.Remembered = ""
			return f
		},
		Fields: append(defs, kernel.Remember(kernel.RememberParams[request, facts]{Fields: defs, Label: "Remember"})),
		Rules: kernel.Rules[request, facts]{}.
			Distinct("branches").
			NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "branches"}),
		Plan: func(_ context.Context, req request, _ facts) (kernel.Plan[plan], error) {
			return kernel.Plan[plan]{Detail: plan{Branches: req.Branches, From: req.From}}, nil
		},
		Apply: func(ctx context.Context, in kernel.ApplyInput[request, facts, plan]) (kernel.Outcome[string], error) {
			r.mu.Lock()
			r.applied = append(r.applied, in.Request)
			r.mu.Unlock()
			items := kernel.Each(ctx, kernel.EachParams[string, string]{
				Items:   in.Plan.Detail.Branches,
				Subject: func(branch string) string { return branch },
				Unit: func(branch string) kernel.Unit[string] {
					return kernel.Unit[string]{Saga: kernel.Saga[string]{
						Steps:  []kernel.SagaStep{{Name: "worktree", Do: r.add(branch)}},
						Commit: func(context.Context) (string, error) { return "/wt/" + branch, nil },
					}}
				},
				Emit:   in.Emit,
				Shield: in.Shield,
			})
			return kernel.Outcome[string]{Items: items}, nil
		},
	}
}

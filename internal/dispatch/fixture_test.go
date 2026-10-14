package dispatch_test

import (
	"context"
	"path/filepath"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
)

// The tests share one example, shaped like `wtm create`: a request naming
// branches to create, the facts read from a fake repository that is also the
// disk Apply writes to, and the command itself, declared at the end.

const (
	commandName   = "create"
	isolated      = "isolated"
	verbatim      = "verbatim"
	hookLine      = "installing dependencies"
	worktreeStep  = "worktree"
	allocatePhase = "allocate"
	onCreate      = "on_create"
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
	Base       string   `json:"base,omitempty"`
}

type plan struct {
	Branches  []string `json:"branches"`
	Isolation string   `json:"isolation"`
}

// detail is what a unit did: the worktree's path, the ordinal the allocate
// phase gave it, and the hook that failed, if one did.
type detail struct {
	Path       string `json:"path"`
	Ordinal    *int   `json:"ordinal,omitempty"`
	HookFailed bool   `json:"hook_failed,omitempty"`
}

// repo is the fake repository: what Observe reads, what Apply changes. When
// hold is set, Apply says so on entered and waits for hold to close. A hook
// that runs calls onHook first, and fails with hookErr when it is set; its
// output is kept under logDir when it is set.
type repo struct {
	mu            sync.Mutex
	existing      []string
	remembered    string
	base          string
	worktrees     []string
	ordinals      int
	applyErr      error
	planWarnings  []kernel.Warning
	applyWarnings []kernel.Warning
	hookErr       error
	onHook        func()
	logDir        string
	observed      int
	lockOf        func(request) []kernel.LockKey
	hold          chan struct{}
	entered       chan struct{}
}

func (r *repo) observe() facts {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observed++
	return facts{Existing: slices.Clone(r.existing), Remembered: r.remembered, Base: r.base}
}

func (r *repo) add(branch string) func(context.Context, kernel.Reporter) error {
	return func(context.Context, kernel.Reporter) error {
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

func (r *repo) allocate(_ context.Context, _ kernel.Reporter, d detail) (detail, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ordinals++
	ordinal := r.ordinals
	d.Ordinal = &ordinal
	return d, nil
}

func (r *repo) hook(_ context.Context, report kernel.Reporter, d detail) (detail, error) {
	if r.onHook != nil {
		r.onHook()
	}
	report.Output(hookLine)
	if r.hookErr != nil {
		return d, r.hookErr
	}
	return d, nil
}

func (r *repo) logOf(branch string) string {
	if r.logDir == "" {
		return ""
	}
	return filepath.Join(r.logDir, onCreate+"-"+rules.EncodeBranchSegment(branch)+".log")
}

func (r *repo) listed() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.worktrees)
}

func (r *repo) observations() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.observed
}

// held makes the next Apply wait until release is called.
func (r *repo) held() (entered <-chan struct{}, release func()) {
	r.hold, r.entered = make(chan struct{}), make(chan struct{})
	var once sync.Once
	return r.entered, func() { once.Do(func() { close(r.hold) }) }
}

func createCommand(r *repo) kernel.Command[request, facts, plan, detail] {
	return kernel.Command[request, facts, plan, detail]{
		Name: commandName,
		Observe: func(context.Context) (facts, error) {
			return r.observe(), nil
		},
		Forget: func(f facts) facts {
			f.Remembered = ""
			return f
		},
		Fields: []kernel.FieldDef[request, facts]{
			{
				Spec: kernel.FieldSpec{Path: "branches", Type: kernel.FieldTextList, Label: "Branches", Required: true},
				Default: func(_ request, f facts) (kernel.Fallback, bool) {
					return kernel.Fallback{Value: kernel.List{f.Base}, Origin: kernel.OriginConfig}, f.Base != ""
				},
			},
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
			if r.lockOf != nil {
				return r.lockOf(req)
			}
			return createLocks(req)
		},
		Plan: func(_ context.Context, req request, _ facts) (kernel.Plan[plan], error) {
			return kernel.Plan[plan]{Detail: plan{Branches: req.Branches, Isolation: req.Isolation}, Warnings: r.planWarnings}, nil
		},
		Apply: func(ctx context.Context, in kernel.ApplyInput[request, facts, plan, detail]) (kernel.Outcome[detail], error) {
			if r.applyErr != nil {
				return kernel.Outcome[detail]{}, r.applyErr
			}
			if r.hold != nil {
				close(r.entered)
				<-r.hold
			}
			items := kernel.Each(ctx, kernel.EachParams[string, detail]{
				Items:   in.Plan.Detail.Branches,
				Subject: func(branch string) string { return branch },
				Unit: func(branch string) kernel.Unit[detail] {
					return kernel.Unit[detail]{
						Saga: kernel.Saga[detail]{
							Steps:  []kernel.SagaStep{{Name: worktreeStep, Do: r.add(branch), Undo: r.remove(branch)}},
							Commit: func(context.Context) (detail, error) { return detail{Path: "/wt/" + branch}, nil },
						},
						Then: []kernel.Phase[detail]{
							{Name: allocatePhase, Run: r.allocate},
							{Name: onCreate, Log: r.logOf(branch), Run: r.hook},
						},
					}
				},
				Emit:       in.Emit,
				Shield:     in.Shield,
				Milestones: in.Milestones,
			})
			return kernel.Outcome[detail]{Items: items, Warnings: r.applyWarnings}, nil
		},
	}
}

// createLocks are the locks of create: the repository shared, each branch
// exclusive.
func createLocks(req request) []kernel.LockKey {
	keys := []kernel.LockKey{{Scope: kernel.LockRepo, Mode: kernel.LockShared}}
	for _, branch := range req.Branches {
		keys = append(keys, kernel.LockKey{Scope: kernel.LockWorktree, Name: branch, Mode: kernel.LockExclusive})
	}
	return keys
}

// createEvents says what create changed, as wtm create does today: created
// at the commit, before on_create; updated once allocate gave the ordinal;
// provisioned once the unit is over, whether its hook ran or failed.
func createEvents() dispatch.Events[facts, detail] {
	return dispatch.Events[facts, detail]{
		Committed: func(e dispatch.EventOf[facts, kernel.Committed[detail]]) []domain.Event {
			return []domain.Event{{Type: domain.EventWorktreeCreated, Worktree: identity(e.Facts, e.Milestone.Subject, e.Milestone.Detail)}}
		},
		PhaseDone: func(e dispatch.EventOf[facts, kernel.PhaseDone[detail]]) []domain.Event {
			if e.Milestone.Phase != allocatePhase {
				return nil
			}
			return []domain.Event{{
				Type:     domain.EventWorktreeUpdated,
				Worktree: identity(e.Facts, e.Milestone.Subject, e.Milestone.Detail),
				Changed:  []domain.IdentityField{domain.IdentityOrdinal},
			}}
		},
		Finished: func(e dispatch.EventOf[facts, kernel.Item[detail]]) []domain.Event {
			item := e.Milestone
			if item.Detail.Path == "" {
				return nil
			}
			ok := item.Status == kernel.StatusDone
			return []domain.Event{{Type: domain.EventWorktreeProvisioned, Worktree: identity(e.Facts, item.Subject, item.Detail), OK: &ok}}
		},
	}
}

func identity(f facts, branch string, d detail) *domain.WorktreeIdentity {
	return &domain.WorktreeIdentity{Branch: branch, Path: d.Path, Parent: f.Base, Ordinal: d.Ordinal}
}

// toggles is a request whose fields are a bool and a decision per key, for the
// options of fields that are not text.
type toggles struct {
	Push      bool              `json:"push"`
	NoPush    bool              `json:"no_push"`
	Decisions map[string]string `json:"decisions"`
}

func togglesCommand() kernel.Command[toggles, struct{}, plan, detail] {
	both := func(context.Context, toggles, struct{}) (kernel.Choices, error) {
		return kernel.Choices{Options: []kernel.Option{{Value: "true", Label: "yes"}, {Value: "false", Label: "no"}}}, nil
	}
	decide := func(context.Context, toggles, struct{}) (kernel.Choices, error) {
		return kernel.Choices{Options: []kernel.Option{{Value: "keep", Label: "keep"}, {Value: "replace", Label: "replace"}}}, nil
	}
	return kernel.Command[toggles, struct{}, plan, detail]{
		Name: "toggles",
		Fields: []kernel.FieldDef[toggles, struct{}]{
			{Spec: kernel.FieldSpec{Path: "push", Type: kernel.FieldBool, Label: "Push"}, Choices: both},
			{Spec: kernel.FieldSpec{Path: "no_push", Type: kernel.FieldBool, Label: "Keep local"}},
			{Spec: kernel.FieldSpec{Path: "decisions", Type: kernel.FieldDecisions, Label: "Drifted keys"}, Choices: decide},
		},
		Rules: kernel.Rules[toggles, struct{}]{}.Exclusive("push", "no_push"),
	}
}

// unencodable is facts JSON cannot encode.
type unencodable struct {
	Watch chan int `json:"watch"`
}

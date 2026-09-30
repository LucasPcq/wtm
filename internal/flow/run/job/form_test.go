package job_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	jobflow "github.com/LucasPcq/wtm/internal/flow/run/job"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

func formAnswers(overrides map[string]string) map[string]string {
	answers := map[string]string{
		jobflow.KeyName: "", jobflow.KeyCmd: "", jobflow.KeyKind: string(domain.JobKindService),
		jobflow.KeyStop: "", jobflow.KeyCwd: "", jobflow.KeyPorts: "", jobflow.KeyURL: "",
		jobflow.KeyBindsNoPort: domain.RunJobBindsNoPortNo, jobflow.KeyScope: domain.ScopeValuePerWorktree,
		jobflow.KeyNamespaceName: "", jobflow.KeyNamespaceCreate: "", jobflow.KeyNamespaceRemove: "", jobflow.KeyNamespaceEnv: "",
	}
	for key, value := range overrides {
		answers[key] = value
	}
	return answers
}

func dataConfig() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up -d db"},
		{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		{Name: "seed", Kind: domain.JobKindTask, Cmd: "pnpm seed"},
	}}
}

// The form asks every field a job declares: a hand-added data task that could
// not say what it touches escaped the foreign-data guard.
func TestAddAsksRunsTouchesAndScope(t *testing.T) {
	ctx := context(t)
	prompter := &flowtest.ScriptedPrompter{
		Answers: formAnswers(map[string]string{
			jobflow.KeyName: "reset", jobflow.KeyCmd: "pnpm db:reset", jobflow.KeyKind: string(domain.JobKindTask),
		}),
		Sets: map[string][]string{jobflow.KeyRuns: {"seed"}, jobflow.KeyTouches: {"db"}},
	}

	if _, err := jobflow.Add(jobflow.AddParams{
		Context: ctx, Request: jobflow.AddRequest{Config: dataConfig()}, Prompter: prompter, Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	for _, key := range []string{jobflow.KeyRuns, jobflow.KeyTouches} {
		if !slices.Contains(prompter.Asked, key) {
			t.Errorf("asked %s, want %s asked", prompter.AskedKeys(), key)
		}
	}
	for _, key := range []string{jobflow.KeyStop, jobflow.KeyBindsNoPort, jobflow.KeyScope, jobflow.KeyNamespaceName} {
		if slices.Contains(prompter.Asked, key) {
			t.Errorf("asked %s of a task, which has no stop, no port and no instance to share", key)
		}
	}
	saved := loadConfig(t, ctx.StateDir).Jobs[3]
	if !slices.Equal(saved.Touches, []string{"db"}) || !slices.Equal(saved.Runs, []string{"seed"}) {
		t.Errorf("saved = %+v, want touches [db] and runs [seed]", saved)
	}
}

// touches offers the services, since data lives there; a task holds none.
func TestTouchesOffersTheDeclaredServices(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{
		Answers: formAnswers(map[string]string{jobflow.KeyName: "reset", jobflow.KeyCmd: "x", jobflow.KeyKind: string(domain.JobKindTask)}),
		Sets:    map[string][]string{jobflow.KeyRuns: nil, jobflow.KeyTouches: nil},
	}
	if _, err := jobflow.Add(jobflow.AddParams{
		Context: context(t), Request: jobflow.AddRequest{Config: dataConfig()}, Prompter: prompter, Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	var offered []string
	for _, option := range prompter.Content[jobflow.KeyTouches].Options {
		offered = append(offered, option.Value)
	}
	if !slices.Equal(offered, []string{"db", "api"}) {
		t.Errorf("touches offered %v, want the services db and api", offered)
	}
}

// Sharing a service opens the namespace questions, and their answers become
// the [job.namespace] block.
func TestAddSharedServiceAsksItsNamespace(t *testing.T) {
	ctx := context(t)
	prompter := &flowtest.ScriptedPrompter{
		Answers: formAnswers(map[string]string{
			jobflow.KeyName: "pg", jobflow.KeyCmd: "docker compose up -d pg",
			jobflow.KeyScope:           domain.ScopeValueShared,
			jobflow.KeyNamespaceName:   "app_{worktree}",
			jobflow.KeyNamespaceCreate: "./db-add.sh",
			jobflow.KeyNamespaceRemove: "./db-rm.sh",
			jobflow.KeyNamespaceEnv:    "DB=app_{worktree}",
		}),
		Sets: map[string][]string{jobflow.KeyRuns: nil, jobflow.KeyTouches: nil},
	}
	if _, err := jobflow.Add(jobflow.AddParams{
		Context: ctx, Request: jobflow.AddRequest{Config: dataConfig()}, Prompter: prompter, Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	pg := loadConfig(t, ctx.StateDir).Jobs[3]
	if pg.Scope != domain.JobScopeShared {
		t.Errorf("scope = %q, want shared", pg.Scope)
	}
	ns := pg.Namespace
	if ns == nil || ns.Name != "app_{worktree}" || ns.Create != "./db-add.sh" || ns.Remove != "./db-rm.sh" || ns.Env["DB"] != "app_{worktree}" {
		t.Errorf("namespace = %+v, want every answered field", ns)
	}
}

// An empty namespace name is an answer: a shared service with one set of data.
// Its commands are not asked, since they would carve nothing.
func TestSharedServiceWithoutANamespaceSkipsItsCommands(t *testing.T) {
	ctx := context(t)
	prompter := &flowtest.ScriptedPrompter{
		Answers: formAnswers(map[string]string{
			jobflow.KeyName: "pg", jobflow.KeyCmd: "docker compose up -d pg", jobflow.KeyScope: domain.ScopeValueShared,
		}),
		Sets: map[string][]string{jobflow.KeyRuns: nil, jobflow.KeyTouches: nil},
	}
	if _, err := jobflow.Add(jobflow.AddParams{
		Context: ctx, Request: jobflow.AddRequest{Config: dataConfig()}, Prompter: prompter, Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if slices.Contains(prompter.Asked, jobflow.KeyNamespaceCreate) {
		t.Errorf("asked %s, want the namespace commands skipped", prompter.AskedKeys())
	}
	if pg := loadConfig(t, ctx.StateDir).Jobs[3]; pg.Scope != domain.JobScopeShared || pg.Namespace != nil {
		t.Errorf("pg = %+v, want shared outright", pg)
	}
}

// The edit form opens on what the file says, field by field, so a reader who
// only changes the command sees the rest as it stands.
func TestEditPreFillsEveryFieldFromTheFile(t *testing.T) {
	cfg := dataConfig()
	cfg.Jobs[0].Scope = domain.JobScopeShared
	cfg.Jobs[0].Namespace = &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "./add.sh", Remove: "./rm.sh", Env: map[string]string{"B": "2", "A": "1"}}
	cfg.Jobs[2].Touches = []string{"db"}

	prompter := &flowtest.ScriptedPrompter{
		Answers: formAnswers(map[string]string{jobflow.KeyName: "db", jobflow.KeyCmd: "x", jobflow.KeyScope: domain.ScopeValueShared,
			jobflow.KeyNamespaceName: "app_{worktree}", jobflow.KeyNamespaceCreate: "./add.sh"}),
		Sets: map[string][]string{jobflow.KeyRuns: nil, jobflow.KeyTouches: nil},
	}
	if _, err := jobflow.Edit(jobflow.EditParams{
		Context: context(t), Request: jobflow.EditRequest{Name: "db", Config: cfg}, Prompter: prompter, Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Edit: %v", err)
	}

	content := prompter.Content
	if content[jobflow.KeyScope].Start != domain.ScopeValueShared {
		t.Errorf("scope opens on %q, want shared", content[jobflow.KeyScope].Start)
	}
	for key, want := range map[string]string{
		jobflow.KeyNamespaceName:   "app_{worktree}",
		jobflow.KeyNamespaceCreate: "./add.sh",
		jobflow.KeyNamespaceRemove: "./rm.sh",
		jobflow.KeyNamespaceEnv:    "A=1 B=2",
	} {
		if content[key].Default != want {
			t.Errorf("%s opens on %q, want %q", key, content[key].Default, want)
		}
	}
	for _, option := range content[jobflow.KeyRuns].Options {
		if option.Value == "db" {
			t.Error("runs offers the job itself")
		}
	}

	seed := &flowtest.ScriptedPrompter{Answers: formAnswers(map[string]string{jobflow.KeyName: "seed", jobflow.KeyCmd: "x", jobflow.KeyKind: string(domain.JobKindTask)}),
		Sets: map[string][]string{jobflow.KeyRuns: nil, jobflow.KeyTouches: {"db"}}}
	if _, err := jobflow.Edit(jobflow.EditParams{
		Context: context(t), Request: jobflow.EditRequest{Name: "seed", Config: cfg}, Prompter: seed, Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Edit seed: %v", err)
	}
	for _, option := range seed.Content[jobflow.KeyTouches].Options {
		if option.Value == "db" && !option.Selected {
			t.Error("touches does not pre-check what the file declares")
		}
	}
}

// Unticking every row is an answer and withdraws the list; a step that was
// never asked leaves it standing.
func TestEditUntickingEveryRowWithdrawsTheList(t *testing.T) {
	ctx := context(t)
	cfg := dataConfig()
	cfg.Jobs[2].Touches = []string{"db"}

	if _, err := jobflow.Edit(jobflow.EditParams{
		Context: ctx, Request: jobflow.EditRequest{Name: "seed", Config: cfg},
		Prompter: &flowtest.ScriptedPrompter{
			Answers: formAnswers(map[string]string{jobflow.KeyName: "seed", jobflow.KeyCmd: "x", jobflow.KeyKind: string(domain.JobKindTask)}),
			Sets:    map[string][]string{jobflow.KeyRuns: nil, jobflow.KeyTouches: {}},
		},
		Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if got := loadConfig(t, ctx.StateDir).Jobs[2].Touches; len(got) != 0 {
		t.Errorf("touches = %v, want withdrawn", got)
	}
}

// Answering per worktree takes the namespace with it: the loader refuses a
// namespace on a per-worktree job, so keeping it would write a file wtm then
// refuses to read.
func TestEditUnsharingDropsTheNamespace(t *testing.T) {
	ctx := context(t)
	cfg := dataConfig()
	cfg.Jobs[0].Scope = domain.JobScopeShared
	cfg.Jobs[0].Namespace = &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "./add.sh"}

	if _, err := jobflow.Edit(jobflow.EditParams{
		Context: ctx, Request: jobflow.EditRequest{Name: "db", Config: cfg},
		Prompter: &flowtest.ScriptedPrompter{
			Answers: formAnswers(map[string]string{jobflow.KeyName: "db", jobflow.KeyCmd: "x"}),
			Sets:    map[string][]string{jobflow.KeyRuns: nil, jobflow.KeyTouches: nil},
		},
		Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if db := loadConfig(t, ctx.StateDir).Jobs[0]; db.Scope != domain.JobScopePerWorktree || db.Namespace != nil {
		t.Errorf("db = %+v, want per worktree without a namespace", db)
	}
}

// Unattended, a namespace name with no create command has no safe default:
// the run is refused naming the flag rather than writing a block the loader
// refuses.
func TestAddUnattendedNamespaceWithoutCreateNamesTheFlag(t *testing.T) {
	_, err := jobflow.Add(jobflow.AddParams{
		Context: context(t),
		Request: jobflow.AddRequest{Config: dataConfig(), Initial: domain.JobConfig{
			Name: "pg", Cmd: "docker compose up -d pg", Scope: domain.JobScopeShared,
			Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}"},
		}},
		Prompter: flow.Unattended{}, Presenter: &recorder{},
	})
	if err == nil || !strings.Contains(err.Error(), domain.FlagNamespaceCreate) {
		t.Fatalf("err = %v, want one naming --%s", err, domain.FlagNamespaceCreate)
	}
}

// Unattended, every new step resolves to what the flags gave and nothing more.
func TestAddUnattendedKeepsWhatTheFlagsDeclared(t *testing.T) {
	ctx := context(t)
	initial := domain.JobConfig{
		Name: "pg", Cmd: "docker compose up -d pg", Kind: domain.JobKindService, BindsNoPort: true,
		Runs: []string{"api"}, Scope: domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "./add.sh", Env: map[string]string{"A": "1"}},
	}
	if _, err := jobflow.Add(jobflow.AddParams{
		Context: ctx, Request: jobflow.AddRequest{Config: dataConfig(), Initial: initial},
		Prompter: flow.Unattended{}, Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	pg := loadConfig(t, ctx.StateDir).Jobs[3]
	if !pg.BindsNoPort || !slices.Equal(pg.Runs, []string{"api"}) || pg.Scope != domain.JobScopeShared {
		t.Errorf("pg = %+v, want binds_no_port, runs and scope kept", pg)
	}
	if pg.Namespace == nil || pg.Namespace.Create != "./add.sh" || pg.Namespace.Env["A"] != "1" {
		t.Errorf("namespace = %+v, want it kept", pg.Namespace)
	}
}

// Walking the form with enter on every field changes nothing, and must say so
// rather than report an update.
func TestEditAnsweringEveryPreFillIsUnchanged(t *testing.T) {
	cfg := dataConfig()
	prompter := &flowtest.ScriptedPrompter{
		Answers: formAnswers(map[string]string{jobflow.KeyName: "api", jobflow.KeyCmd: "pnpm dev"}),
		Sets:    map[string][]string{jobflow.KeyRuns: nil, jobflow.KeyTouches: nil},
	}
	presenter := &recorder{}

	if _, err := jobflow.Edit(jobflow.EditParams{
		Context: context(t), Request: jobflow.EditRequest{Name: "api", Config: cfg}, Prompter: prompter, Presenter: presenter,
	}); err != nil {
		t.Fatalf("Edit: %v", err)
	}

	if len(presenter.changed) != 1 || presenter.changed[0].Status != domain.JobActionUnchanged {
		t.Errorf("outcome = %+v, want the job reported unchanged", presenter.changed)
	}
}

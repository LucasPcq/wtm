package teardown

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The data of a removal

// A Namespace is one namespace a worktree made in a shared service, as a plan decides its fate: the shared job, the namespace's name, what becomes of it, and why it stays when it does.
type Namespace struct {
	Job    string     `json:"job"`
	Name   string     `json:"name"`
	Status DataStatus `json:"status"`
	Reason DataReason `json:"reason,omitempty"`
	drop   env.NamespaceRun
}

// DataStatus is what a plan does with a namespace of a removed worktree.
type DataStatus string

// The fates of a namespace.
const (
	DataDrop  DataStatus = "drop"  // its service is up: dropped once the worktree is removed
	DataStart DataStatus = "start" // its service is down: started, the namespace dropped, the service stopped again
	DataDefer DataStatus = "defer" // its service is down: owed until it next starts
	DataKeep  DataStatus = "keep"  // kept, for its Reason
)

// DataStatuses returns every DataStatus.
func DataStatuses() []DataStatus {
	return []DataStatus{DataDrop, DataStart, DataDefer, DataKeep}
}

// DataReason is why a namespace is not dropped with its worktree.
type DataReason string

// The reasons a namespace stays.
const (
	ReasonKeepData    DataReason = "keep_data"    // the request keeps the data
	ReasonSharedName  DataReason = "shared_name"  // another worktree reaches the namespace under the same name: it is that worktree's too
	ReasonServiceDown DataReason = "service_down" // its service is down: owed until it next starts
)

// DataReasons returns every DataReason.
func DataReasons() []DataReason {
	return []DataReason{ReasonKeepData, ReasonSharedName, ReasonServiceDown}
}

// A Choice is what a request says of the data its removals hold: kept whatever its service (KeepData), or the service holding it started when it is down, to drop it now (DropData); neither drops the data of a service up and owes the rest.
type Choice struct {
	KeepData bool
	DropData bool
}

// data are the namespaces the record of w names that a shared job of run.toml would drop: each kept when another worktree reaches it under the same name, dropped when its service is up in the main checkout, owed otherwise.
func (f Facts) data(w worktree.Worktree) []Namespace {
	record := f.Records[w.Branch()]
	var found []Namespace
	for _, name := range record.Namespaces {
		job, declared := f.job(name)
		if !declared || job.Scope != project.Shared {
			continue
		}
		drop, owed, err := env.NamespaceDrop(job, f.jobEnv(w, job))
		if err != nil || !owed {
			continue
		}

		namespace := Namespace{Job: name, Name: drop.Name, Status: DataDefer, Reason: ReasonServiceDown, drop: drop}
		switch {
		case f.reached(reachedParams{Job: job, Name: drop.Name, Except: w.Name()}):
			namespace.Status, namespace.Reason = DataKeep, ReasonSharedName
		case f.up(name):
			namespace.Status, namespace.Reason = DataDrop, ""
		}
		found = append(found, namespace)
	}

	return found
}

// With is the removal under choice: its data kept, or the namespaces its services down owe started instead.
func (r Removal) With(choice Choice) Removal {
	r.Data = slices.Clone(r.Data)
	for i, namespace := range r.Data {
		switch {
		case namespace.Status == DataKeep:
		case choice.KeepData:
			r.Data[i].Status, r.Data[i].Reason = DataKeep, ReasonKeepData
		case choice.DropData && namespace.Status == DataDefer:
			r.Data[i].Status, r.Data[i].Reason = DataStart, ""
		}
	}
	r.Phases = r.phases()

	return r
}

// Down are the shared services, sorted, that removals would owe data to for being down: those a request may start to drop it now.
func Down(removals []Removal) []string {
	var jobs []string
	for _, r := range removals {
		for _, namespace := range r.Data {
			if namespace.Status == DataDefer && !slices.Contains(jobs, namespace.Job) {
				jobs = append(jobs, namespace.Job)
			}
		}
	}
	slices.Sort(jobs)

	return jobs
}

// job is the job of run.toml named name, and whether it declares one.
func (f Facts) job(name string) (project.Job, bool) {
	at := slices.IndexFunc(f.Run.Jobs, func(job project.Job) bool { return job.Name == name })
	if at < 0 {
		return project.Job{}, false
	}
	return f.Run.Jobs[at], true
}

// jobParams are what the job environment of the worktree of branch derives from.
func (f Facts) jobParams(branch string) env.JobParams {
	record := f.Records[branch]
	return env.JobParams{Branch: branch, ProjectDir: f.MainPath, Ordinal: record.Ordinal, Isolation: record.Isolation.Effective(), Run: f.Run}
}

// jobEnv is the environment job got in w: w's job environment, read while it was there, and the job's own ports.
func (f Facts) jobEnv(w worktree.Worktree, job project.Job) map[string]string {
	params := f.jobParams(w.Branch())
	jobEnv, read := f.Envs[w.Name()]
	if !read {
		jobEnv = env.HookEnv(params, nil)
	}
	return withPorts(jobEnv, params.JobPorts(job))
}

type reachedParams struct {
	Job    project.Job
	Name   string
	Except string
}

// reached reports whether a worktree but the one named Except reaches the namespace of Job under Name: dropping it would destroy that worktree's data. The main checkout reaches it as a start there names it, on a branch or not; another worktree on no branch has no record to name it from.
func (f Facts) reached(params reachedParams) bool {
	return slices.ContainsFunc(f.Worktrees, func(w worktree.Worktree) bool {
		jobParams := f.jobParams(w.Branch())
		switch {
		case w.Name() == params.Except:
			return false
		case w.Main:
			jobParams = f.mainParams()
		case w.Branch() == "":
			return false
		}
		theirs, made, _ := env.NamespaceFor(params.Job, env.HookEnv(jobParams, nil))
		return made && theirs.Name == params.Name
	})
}

// up reports whether the shared job of name is up in this repository's main checkout, where it runs: another repository's job of the same name holds none of this one's data.
func (f Facts) up(name string) bool {
	return slices.ContainsFunc(f.Jobs, func(job run.Job) bool { return job.Up && job.Name == name && job.WorkDir == f.MainPath })
}

// withPorts is jobEnv with each of ports spelled, a copy.
func withPorts(jobEnv map[string]string, ports map[string]int) map[string]string {
	spelled := maps.Clone(jobEnv)
	if spelled == nil {
		spelled = map[string]string{}
	}
	for name, port := range ports {
		spelled[name] = strconv.Itoa(port)
	}
	return spelled
}

// Envs reads the job environment of each of worktrees, by name, as its jobs got it, while it is there: what the drop of its namespaces runs with. A worktree whose compose env file cannot be read gets the rest of it.
func (s Service) Envs(f Facts, worktrees []worktree.Worktree) map[string]map[string]string {
	envs := map[string]map[string]string{}
	for _, w := range worktrees {
		if w.Branch() == "" {
			continue
		}
		params := f.jobParams(w.Branch())
		jobEnv, err := env.ReadHookEnv(params, w.Path, s.EnvFiles.Read)
		if err != nil {
			jobEnv = env.HookEnv(params, nil)
		}
		envs[w.Name()] = jobEnv
	}

	return envs
}

// ----------------------------------------------------------------------------
// The data given back

// A Dropped is what became of one namespace of a removed worktree: the shared job, its name, its fate, and why it stays when it does.
type Dropped struct {
	Job    string     `json:"job"`
	Name   string     `json:"name"`
	Status DropStatus `json:"status"`
	Reason DataReason `json:"reason,omitempty"`
}

// DropStatus is what became of a namespace of a removed worktree.
type DropStatus string

// What becomes of a namespace.
const (
	NamespaceDropped  DropStatus = "dropped"  // its remove command ran
	NamespaceDeferred DropStatus = "deferred" // owed to its service, dropped once it is up
	NamespaceKept     DropStatus = "kept"     // kept, for its Reason
	NamespaceFailed   DropStatus = "failed"   // its remove command failed: owed, tried again once its service is up
)

// DropStatuses returns every DropStatus.
func DropStatuses() []DropStatus {
	return []DropStatus{NamespaceDropped, NamespaceDeferred, NamespaceKept, NamespaceFailed}
}

// The codes of what becomes of a removal's data, after the command's name: a namespace owed to a service down, a drop that failed, a namespace kept, the queue of what is owed unwritten, a worktree's hold on the shared services or a service started to drop data that could not be let go of.
const (
	DataDeferred   = ".data_deferred"
	DropFailed     = ".drop_failed"
	DataKept       = ".data_kept"
	OwedUnrecorded = ".owed_unrecorded"
	ReleaseFailed  = ".release_failed"
)

// dropWithin bounds a namespace's remove command, as the legacy wtm does.
const dropWithin = 30 * time.Second

// dropData gives back the data of a worktree removed, between the events of its phase when something is dropped: each namespace dropped from the main checkout with the worktree's environment, the others owed or kept, each that stays warned; the drops owed recorded, and any older debt of a namespace dropped paid.
func (s Service) dropData(ctx context.Context, params RemoveParams) ([]Dropped, []domain.Warning) {
	removal, report := params.Removal, params.Report
	dropping := slices.Contains(removal.Phases, domain.PhaseDropData)
	if dropping {
		report(domain.PhaseStarted{Subject: removal.Worktree, Phase: domain.PhaseDropData})
	}

	var dropped []Dropped
	var warnings []domain.Warning
	var owed, done []env.OwedDrop
	for _, namespace := range removal.Data {
		about := domain.Params{"worktree": removal.Worktree, "job": namespace.Job, "namespace": namespace.Name}
		unlit := params.Unlit[namespace.Job]
		switch {
		case namespace.Status == DataKeep:
			dropped = append(dropped, Dropped{Job: namespace.Job, Name: namespace.Name, Status: NamespaceKept, Reason: namespace.Reason})
			about["reason"] = string(namespace.Reason)
			warnings = append(warnings, domain.Warning{Code: params.Command + DataKept, Message: namespace.Name + " stays in " + namespace.Job + keptWhy(namespace.Reason), Params: about})
		case namespace.Status == DataDefer || namespace.Status == DataStart && unlit != nil:
			why := namespace.Job + " is down"
			if unlit != nil {
				why = namespace.Job + " could not be started (" + unlit.Error() + ")"
			}
			dropped = append(dropped, Dropped{Job: namespace.Job, Name: namespace.Name, Status: NamespaceDeferred, Reason: ReasonServiceDown})
			owed = append(owed, env.Owe(namespace.Job, namespace.drop))
			warnings = append(warnings, domain.Warning{Code: params.Command + DataDeferred, Message: namespace.Name + " stays in " + namespace.Job + ": " + why + "; it is dropped once " + namespace.Job + " is up (the next run up or prune)", Params: about})
		default:
			if err := s.drop(ctx, params.Facts, namespace.Job, namespace.drop, func(line string) { report(domain.Output{Subject: removal.Worktree, Line: line}) }); err != nil {
				dropped = append(dropped, Dropped{Job: namespace.Job, Name: namespace.Name, Status: NamespaceFailed})
				owed = append(owed, env.Owe(namespace.Job, namespace.drop))
				about["cause"] = err.Error()
				warnings = append(warnings, domain.Warning{Code: params.Command + DropFailed, Message: "couldn't drop " + namespace.Name + " from " + namespace.Job + " (" + err.Error() + "): it is tried again once " + namespace.Job + " is up", Params: about})
				continue
			}
			dropped = append(dropped, Dropped{Job: namespace.Job, Name: namespace.Name, Status: NamespaceDropped})
			done = append(done, env.Owe(namespace.Job, namespace.drop))
		}
	}
	if dropping {
		report(domain.PhaseFinished{Subject: removal.Worktree, Phase: domain.PhaseDropData, Failed: slices.ContainsFunc(dropped, func(d Dropped) bool { return d.Status == NamespaceFailed })})
	}

	if err := s.owe(ctx, owed, done); err != nil {
		warnings = append(warnings, domain.Warning{Code: params.Command + OwedUnrecorded, Message: "couldn't record the drops owed in pending-removals.toml (" + err.Error() + "): " + removal.Worktree + "'s namespaces stay until dropped by hand", Params: domain.Params{"worktree": removal.Worktree, "cause": err.Error()}})
	}
	return dropped, warnings
}

func keptWhy(reason DataReason) string {
	if reason == ReasonSharedName {
		return ": another worktree reaches it under the same name"
	}
	return ": --keep-data keeps it"
}

// drop runs the remove command of job's namespace as a chain of one hook, from the main checkout (the worktree is gone by then) with the variables drop keeps, within dropWithin.
func (s Service) drop(ctx context.Context, f Facts, name string, drop env.NamespaceRun, output func(string)) error {
	job, declared := f.job(name)
	if !declared || job.Namespace == nil {
		return errors.New("run.toml no longer declares its remove command")
	}

	ctx, cancel := context.WithTimeout(ctx, dropWithin)
	defer cancel()
	return s.RunHooks(ctx, env.HookChain{
		Hooks:   []project.Hook{{Cmd: job.Namespace.Remove}},
		WorkDir: f.MainPath,
		Env:     drop.Env,
		Unset:   env.NotInherited(f.Run),
		Output:  output,
		Reap:    true,
	})
}

// owe records the drops owed in the queue and takes out of it those done or owed again: one namespace is owed once.
func (s Service) owe(ctx context.Context, owed, done []env.OwedDrop) error {
	if len(owed) == 0 && len(done) == 0 {
		return nil
	}
	settled := append(slices.Clone(owed), done...)
	return s.Owed.Update(context.WithoutCancel(ctx), func(held []env.OwedDrop) []env.OwedDrop {
		kept := slices.DeleteFunc(held, func(drop env.OwedDrop) bool {
			return slices.ContainsFunc(settled, func(s env.OwedDrop) bool { return sameDrop(s, drop) })
		})
		return append(kept, owed...)
	})
}

// sameDrop reports whether a and b owe one namespace: one job's, under one name; an entry the legacy wtm wrote has no name, and is the same worktree's slug and ordinal.
func sameDrop(a, b env.OwedDrop) bool {
	if a.Name != "" && b.Name != "" {
		return a.Job == b.Job && a.Name == b.Name
	}
	return a.Job == b.Job && a.Worktree == b.Worktree && a.Ordinal == b.Ordinal
}

// ----------------------------------------------------------------------------
// The services a batch starts and lets go of

// light starts, in the main checkout, each shared service down that the removals' data asks started, so its namespaces are dropped now; it returns why each that could not start did not, its namespaces then owed.
func (s Service) light(ctx context.Context, params BatchParams) (lit []string, unlit map[string]error) {
	unlit = map[string]error{}
	f := params.Facts
	for _, r := range params.Removals {
		for _, namespace := range r.Data {
			if namespace.Status != DataStart || slices.Contains(lit, namespace.Job) || unlit[namespace.Job] != nil {
				continue
			}
			job, _ := f.job(namespace.Job)
			if _, err := s.Daemon.Start(ctx, s.mainSpec(f, job, params.CorrelationID), func(run.Line) {}); err != nil {
				unlit[namespace.Job] = err
				continue
			}
			lit = append(lit, namespace.Job)
		}
	}

	return lit, unlit
}

// mainEnv is the job environment of the main checkout, read as a run up there reads it.
func (s Service) mainEnv(f Facts) map[string]string {
	params := f.mainParams()
	mainEnv, err := env.ReadHookEnv(params, f.MainPath, s.EnvFiles.Read)
	if err != nil {
		return env.HookEnv(params, nil)
	}
	return mainEnv
}

// mainParams are what the main checkout's job environment derives from, as a run up there derives it: no ordinal.
func (f Facts) mainParams() env.JobParams {
	params := f.jobParams(f.mainBranch())
	params.Ordinal = 0
	return params
}

// mainSpec is the start of job, shared, in the main checkout, as a run up there asks it: the main checkout's environment and the job's ports, its logs, the origin its events name. shortcut: a shared runner's children and the proxy's names are not asked, which only a run up serves; ask them here if a shared service ever needs its URL to take its data back.
func (s Service) mainSpec(f Facts, job project.Job, correlationID string) run.Spec {
	branch := f.mainBranch()
	jobEnv := withPorts(s.mainEnv(f), f.mainParams().JobPorts(job))
	logDir := run.LogDir(s.StateDir, branch)
	return run.Spec{
		Job:     job,
		WorkDir: f.MainPath,
		Env:     jobEnv,
		Unset:   env.NotInherited(f.Run),
		LogDir:  logDir,
		Origin:  &run.Origin{Root: f.MainPath, CommonDir: s.CommonDir, Branch: branch, CorrelationID: correlationID},
		Shared:  &run.Shared{WorkDir: f.MainPath, Env: jobEnv, LogDir: logDir},
	}
}

// mainBranch is the branch of the main checkout, empty when it has none.
func (f Facts) mainBranch() string {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Main })
	if at < 0 {
		return ""
	}
	return f.Worktrees[at].Branch()
}

// release lets go of what the removals went with held in the shared services, once every drop is made (the last claim on a service stops it, which then takes no namespace back): every job of each worktree removed, its claims among them; then each service lit to drop data, stopped again unless another worktree holds it.
func (s Service) release(ctx context.Context, params BatchParams, removed []Removed, lit []string) []domain.Warning {
	var warnings []domain.Warning
	for i, r := range params.Removals {
		if !removed[i].Gone || r.Path == "" {
			continue
		}
		if _, err := s.Daemon.StopAll(ctx, run.StopAll{WorkDir: r.Path, CorrelationID: params.CorrelationID}); err != nil {
			warnings = append(warnings, domain.Warning{Code: params.Command + ReleaseFailed, Message: "couldn't let go of " + r.Worktree + "'s shared services (" + err.Error() + "): they run until stopped", Params: domain.Params{"worktree": r.Worktree, "cause": err.Error()}})
		}
	}
	for _, job := range lit {
		if _, err := s.Daemon.Stop(ctx, run.Stop{Name: job, WorkDir: params.Facts.MainPath, CorrelationID: params.CorrelationID}, nil); err != nil {
			warnings = append(warnings, domain.Warning{Code: params.Command + ReleaseFailed, Message: "couldn't stop " + job + ", started to drop data (" + err.Error() + "): it runs until stopped", Params: domain.Params{"job": job, "cause": err.Error()}})
		}
	}

	return warnings
}

// ----------------------------------------------------------------------------
// The drops owed

// The codes of the drops owed, after the command's name: one withdrawn (a worktree reaches its namespace again under the same name), those still owed to a service down.
const (
	OwedWithdrawn = ".owed_withdrawn"
	StillOwed     = ".owed"
)

// A Debt is a drop owed that a run pays: the shared job, the namespace's name, and the slug of the worktree that owed it.
type Debt struct {
	Job      string `json:"job"`
	Name     string `json:"name"`
	Worktree string `json:"worktree"`
	owed     env.OwedDrop
	drop     env.NamespaceRun
}

// Owing reads the drops owed, for Facts.Owed; a queue that cannot be read owes nothing: a debt that cannot be read cannot be paid either.
func (s Service) Owing() []env.OwedDrop {
	owed, err := s.Owed.Load()
	if err != nil {
		return nil
	}
	return owed
}

// Payable are the drops owed a run pays: each whose shared service is up in the main checkout, or among starting (the shared jobs the run itself starts), and that no worktree reaches again under its name (Pay withdraws those).
func (f Facts) Payable(starting []string) []Debt {
	var debts []Debt
	for _, owed := range f.Owed {
		job, declared := f.job(owed.Job)
		drop, droppable, err := owed.Drop(job)
		if !declared || err != nil || !droppable || f.reached(reachedParams{Job: job, Name: drop.Name}) {
			continue
		}
		if f.up(owed.Job) || slices.Contains(starting, owed.Job) {
			debts = append(debts, Debt{Job: owed.Job, Name: drop.Name, Worktree: owed.Worktree, owed: owed, drop: drop})
		}
	}

	return debts
}

// Unpaid warns, per shared service, of the drops owed that debts leaves owed: <command>.owed.
func (f Facts) Unpaid(command string, debts []Debt) []domain.Warning {
	still := map[string]int{}
	for _, owed := range f.Owed {
		paid := slices.ContainsFunc(debts, func(d Debt) bool { return sameDrop(d.owed, owed) })
		job, _ := f.job(owed.Job)
		drop, _, _ := owed.Drop(job)
		if !paid && !f.reached(reachedParams{Job: job, Name: drop.Name}) {
			still[owed.Job]++
		}
	}

	var warnings []domain.Warning
	for _, job := range slices.Sorted(maps.Keys(still)) {
		count := strconv.Itoa(still[job])
		warnings = append(warnings, domain.Warning{Code: command + StillOwed, Message: count + " namespace(s) still owed to " + job + ", dropped once it is up", Params: domain.Params{"job": job, "count": count}})
	}
	return warnings
}

// PayParams are the command paying, the facts it planned on, and the debts it planned to pay.
type PayParams struct {
	Command string
	Facts   Facts
	Debts   []Debt
}

// A Paid is what became of a debt a run planned to pay, which the run reports as an item of its own: done once dropped, or once another run dropped it first (unchanged); failed with the error it failed on; skipped, not reached, when the run stopped first or its service is not up after all; and its warnings.
type Paid struct {
	Debt     Debt
	Status   domain.Status
	Reason   domain.Reason
	Err      *domain.Error
	Warnings []domain.Warning
}

// The codes of a debt a run could not pay, after the command's name: its drop failed, or the queue could not be written, before the drop (not made) or after a drop that failed (the debt lost).
const (
	OwedLost = ".owed_lost"
)

// Unpaid is each of debts, not reached: what a run that started no job reports of the drops it planned to pay.
func Unpaid(debts []Debt) []Paid {
	paid := make([]Paid, 0, len(debts))
	for _, debt := range debts {
		paid = append(paid, Paid{Debt: debt, Status: domain.StatusSkipped, Reason: domain.ReasonNotReached})
	}
	return paid
}

// Pay withdraws each drop owed a worktree reaches again under its name (its data that worktree's now), then pays each of the debts, in order, while its service is up: it is taken out of the queue, under its lock, then dropped from the main checkout with the variables kept for it. One another run took out first is paid already; one whose drop fails goes back in the queue. It returns what became of each debt, and the warnings of the withdrawals.
func (s Service) Pay(ctx context.Context, params PayParams) ([]Paid, []domain.Warning) {
	if ctx.Err() != nil {
		return Unpaid(params.Debts), nil
	}
	paid := make([]Paid, 0, len(params.Debts))

	warnings := s.withdraw(ctx, params)
	if len(params.Debts) == 0 {
		return paid, warnings
	}
	// The services are asked again: a start that planned to bring one up may have failed to.
	jobs, _ := s.jobs(ctx)
	f := params.Facts
	f.Jobs = jobs
	for _, debt := range params.Debts {
		paid = append(paid, s.pay(ctx, f, params.Command, debt))
	}
	return paid, warnings
}

// pay pays debt: taken out of the queue, then dropped, put back when the drop fails.
func (s Service) pay(ctx context.Context, f Facts, command string, debt Debt) Paid {
	about := domain.Params{"job": debt.Job, "namespace": debt.Name, "worktree": debt.Worktree}
	if ctx.Err() != nil || !f.up(debt.Job) {
		return Paid{Debt: debt, Status: domain.StatusSkipped, Reason: domain.ReasonNotReached}
	}

	claimed, err := s.settle(ctx, debt.owed, true)
	switch {
	case err != nil:
		return Paid{Debt: debt, Status: domain.StatusFailed, Err: &domain.Error{Kind: domain.KindInternal, Code: command + OwedUnrecorded, Message: "couldn't take " + debt.Name + " out of pending-removals.toml: not dropped, still owed", Params: about, Cause: err}}
	case !claimed:
		return Paid{Debt: debt, Status: domain.StatusDone, Reason: domain.ReasonUnchanged}
	}

	err = s.drop(ctx, f, debt.Job, debt.drop, func(string) {})
	if err == nil {
		return Paid{Debt: debt, Status: domain.StatusDone}
	}
	failed := Paid{Debt: debt, Status: domain.StatusFailed, Err: &domain.Error{Kind: domain.KindInternal, Code: command + DropFailed, Message: "couldn't drop " + debt.Name + " from " + debt.Job + ", still owed", Params: about, Cause: err}}
	if _, putErr := s.settle(ctx, debt.owed, false); putErr != nil {
		failed.Warnings = []domain.Warning{{Code: command + OwedLost, Message: "couldn't record " + debt.Name + " again in pending-removals.toml (" + putErr.Error() + "): it is no longer owed, drop it by hand: WTM_NAMESPACE=" + debt.Name + " " + f.removeCommand(debt.Job), Params: about}}
	}
	return failed
}

// removeCommand is the remove command of job's namespace, as run.toml spells it.
func (f Facts) removeCommand(name string) string {
	job, declared := f.job(name)
	if !declared || job.Namespace == nil {
		return "its remove command"
	}
	return job.Namespace.Remove
}

// withdraw takes out of the queue each drop owed a worktree reaches again under its name, warned.
func (s Service) withdraw(ctx context.Context, params PayParams) []domain.Warning {
	f := params.Facts
	var warnings []domain.Warning
	for _, owed := range f.Owed {
		job, _ := f.job(owed.Job)
		drop, droppable, _ := owed.Drop(job)
		if !droppable || !f.reached(reachedParams{Job: job, Name: drop.Name}) {
			continue
		}
		if withdrawn, err := s.settle(ctx, owed, true); err == nil && withdrawn {
			warnings = append(warnings, domain.Warning{Code: params.Command + OwedWithdrawn, Message: drop.Name + " is no longer owed to " + owed.Job + ": a worktree reaches it again under that name", Params: domain.Params{"job": owed.Job, "namespace": drop.Name}})
		}
	}
	return warnings
}

// settle takes owed out of the queue (out), reporting whether it was there, or puts it back.
func (s Service) settle(ctx context.Context, owed env.OwedDrop, out bool) (bool, error) {
	there := false
	err := s.Owed.Update(context.WithoutCancel(ctx), func(held []env.OwedDrop) []env.OwedDrop {
		there = slices.ContainsFunc(held, func(drop env.OwedDrop) bool { return sameDrop(drop, owed) })
		if !out {
			return append(slices.DeleteFunc(held, func(drop env.OwedDrop) bool { return sameDrop(drop, owed) }), owed)
		}
		return slices.DeleteFunc(held, func(drop env.OwedDrop) bool { return sameDrop(drop, owed) })
	})
	return there, err
}

// jobs are the jobs wtm's daemon lists, a daemon of another build's among them.
func (s Service) jobs(ctx context.Context) ([]run.Job, error) {
	jobs, err := s.Daemon.Jobs(ctx)
	if _, skewed := errors.AsType[*run.Skewed](err); skewed {
		return jobs, nil
	}
	return jobs, err
}

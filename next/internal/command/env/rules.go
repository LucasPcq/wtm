package env

import (
	"cmp"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	envs "github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what env decides from, as Observe read them: the project's config, run.toml and the repository's snapshot, where each name asked as a path leads, the meta record of each live branch, each worktree's .env files as read with their sources (or why they could not be), whether its directory is gone, the port a named address announces, and the jobs wtm's daemon lists (or why it could not tell).
type Facts struct {
	worktree.Repository
	Located    map[string]string          // the worktree root each name asked that is no worktree's name leads to, when it leads to one
	Records    map[string]worktree.Meta   // by branch
	Reads      map[string][]envs.FileRead // by worktree name
	ReadErrs   map[string]error           // by worktree name
	Missing    map[string]bool            // by worktree name: its directory is gone
	PublicPort int                        // the port a named address announces; 0 when no proxy serves names
	Jobs       []run.Job
	JobsErr    error
}

// onBranch are the worktrees env may reconcile, the main one first: those on a branch.
func (f Facts) onBranch() []worktree.Worktree {
	return slices.DeleteFunc(f.MainFirst(), func(w worktree.Worktree) bool { return w.Branch() == "" })
}

// asked are the worktrees req asks, each once, those on a branch when it names none or asks for every one.
func (f Facts) asked(req Request) []worktree.Worktree {
	if !req.named() {
		return f.onBranch()
	}

	var found []worktree.Worktree
	for _, name := range req.Worktrees {
		if w, ok := f.subject(name); ok && w.Branch() != "" && !slices.Contains(found, w) {
			found = append(found, w)
		}
	}
	return found
}

// subject is the worktree name names: by its name, else by the worktree root the path it stands for leads to.
func (f Facts) subject(name string) (worktree.Worktree, bool) {
	located, isPath := f.Located[name]
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name || isPath && w.Path == located })
	if at < 0 {
		return worktree.Worktree{}, false
	}

	return f.Worktrees[at], true
}

// picked are the worktrees picked by default, the main one first: those out of step under req's switches, and the one the caller stands in.
func (f Facts) picked(req Request) []string {
	worktrees := f.onBranch()
	current := worktree.Holding(worktrees, req.Caller.Cwd, func(w worktree.Worktree) string { return w.Path })
	var names []string
	for i, w := range worktrees {
		if i == current || f.reconcile(w.Name(), req).changes() > 0 {
			names = append(names, w.Name())
		}
	}

	return names
}

// strategy is where the values of w's .env files come from: req's, else its record's, else the project's, else the default.
func (f Facts) strategy(w worktree.Worktree, req Request) project.EnvStrategy {
	return cmp.Or(req.Strategy, f.Records[w.Branch()].EnvStrategy, f.Config.Env.Strategy, project.DefaultEnvStrategy)
}

// isolation is how w runs beside the others under req: the main checkout always isolated, another as req switches it to, else as its record says, isolated when it says nothing.
func (f Facts) isolation(w worktree.Worktree, req Request) project.Isolation {
	switch {
	case w.Main:
		return project.Isolated
	case req.Isolation != "":
		return req.Isolation
	}
	return f.Records[w.Branch()].Isolation.Effective()
}

// pending reports whether w has yet to choose its isolation: a linked worktree whose record holds no choice, while run.toml declares something a worktree isolates.
func (f Facts) pending(w worktree.Worktree) bool {
	return !w.Main && f.RunErr == nil && f.Records[w.Branch()].Isolation == "" && f.Run.IsolationApplies()
}

// ----------------------------------------------------------------------------
// The decision of each worktree

// A Reconcile is one worktree asked: its name and path, whether it is the main checkout, whether its directory is gone (left as it is), the strategy its values come from, its isolation, whether it has yet to choose one (Pending: its ports and the values wtm owns are left as they are), the isolation the apply records when it changes (Records), whether it holds an ordinal (Numbered: one that needs it and has none is numbered by the apply, which settles its ports then), each .env file, what wtm writes into them (its values withheld unless shown), why its ports cannot be settled when they cannot, and the jobs up in it that read a file that changes, which run on the old values until restarted.
type Reconcile struct {
	Worktree  string              `json:"worktree"`
	Path      string              `json:"path"`
	Main      bool                `json:"main"`
	Missing   bool                `json:"missing"`
	Strategy  project.EnvStrategy `json:"strategy"`
	Isolation project.Isolation   `json:"isolation"`
	Pending   bool                `json:"pending"`
	Records   project.Isolation   `json:"records,omitempty"`
	Numbered  bool                `json:"numbered"`
	Files     []File              `json:"files"`
	Ports     envs.Redacted       `json:"ports"`
	Unsettled string              `json:"unsettled,omitempty"`
	Restart   []string            `json:"restart"`
	settle    settle
}

// A File is one .env file of a worktree: whether it is created (the worktree lacks it and it is rebuilt), its keys not in step with their source, each settled or left, and whether it is written.
type File struct {
	Target  string `json:"target"`
	Created bool   `json:"created"`
	Keys    []Key  `json:"keys"`
	Writes  bool   `json:"writes"`
}

// A Key is a key of a .env file not in step with its source: its status, whether the apply settles it on its source, and, when values are shown, its value here and its source's.
type Key struct {
	Key     string         `json:"key"`
	Status  envs.KeyStatus `json:"status"`
	Settled bool           `json:"settled"`
	Value   *string        `json:"value,omitempty"`
	Source  *string        `json:"source,omitempty"`
}

// settle is what the apply settles a worktree's files from: the reconciliation planned, and what its writes resolve from, again once an unnumbered worktree is numbered.
type settle struct {
	reconcile envs.ReconcileParams
	resolve   envs.ResolveParams
	planned   envs.Reconciled
}

// changes counts what reconciling r changes: each file created, each key settled, each linked value rewritten, each value wtm owns set, and the ports of a worktree to number.
func (r Reconcile) changes() int {
	count := 0
	for _, file := range r.Files {
		if file.Created {
			count++
		}
		for _, key := range file.Keys {
			if key.Settled {
				count++
			}
		}
	}
	for _, owned := range r.Ports.Owned {
		if owned.Changed {
			count++
		}
	}
	if !r.Numbered {
		count++
	}
	if r.Records != "" {
		count++
	}
	return count + len(envs.Plan(r.Ports).Rewrites())
}

// writes reports whether the apply writes a file of r, numbers it or records its isolation.
func (r Reconcile) writes() bool {
	return !r.Missing && (!r.Numbered || r.Records != "" || slices.ContainsFunc(r.Files, func(f File) bool { return f.Writes }))
}

// A KeyAt is a key of a worktree's .env file.
type KeyAt struct {
	Worktree string `json:"worktree"`
	File     string `json:"file"`
	Key      string `json:"key"`
}

// keysOf are the keys of status in the worktrees of reconciles.
func keysOf(reconciles []Reconcile, status envs.KeyStatus) []KeyAt {
	var found []KeyAt
	for _, r := range reconciles {
		for _, f := range r.Files {
			for _, k := range f.Keys {
				if k.Status == status {
					found = append(found, KeyAt{Worktree: r.Worktree, File: f.Target, Key: k.Key})
				}
			}
		}
	}
	return found
}

// reconcile decides what reconciling the worktree name does under req: its keys settled on their source, then what wtm writes over them, the main checkout keeping its addresses spelled with ports when its .env spells them so.
func (f Facts) reconcile(name string, req Request) Reconcile {
	w, _ := f.subject(name)
	r := Reconcile{Worktree: name, Path: w.Path, Main: w.Main, Missing: f.Missing[name] || w.Prunable, Strategy: f.strategy(w, req), Isolation: f.isolation(w, req), Pending: req.Isolation == "" && f.pending(w), Numbered: true, Files: []File{}, Restart: []string{}}
	if req.Isolation != "" && !w.Main && f.Records[w.Branch()].Isolation != req.Isolation {
		r.Records = req.Isolation
	}
	if r.Missing || f.ReadErrs[name] != nil {
		return r
	}

	record := f.Records[w.Branch()]
	resolve := envs.ResolveParams{
		JobParams:  envs.JobParams{Branch: w.Branch(), ProjectDir: f.MainPath, Ordinal: record.Ordinal, Isolation: r.Isolation, Run: f.Run},
		Files:      f.Config.Env.Files,
		Proxy:      f.Global.Proxy,
		PublicPort: f.PublicPort,
	}
	if !w.Main && !r.Pending && record.Ordinal == 0 && envs.NeedsOrdinal(r.Isolation, f.Run) {
		r.Numbered = false
	}
	verbatim := r.Isolation == project.Verbatim
	settled := envs.Settle(envs.SettleParams{
		Resolve:   resolve,
		Reconcile: envs.ReconcileParams{Reads: f.Reads[name], Strategy: r.Strategy, Run: f.Run, Verbatim: verbatim, Restore: verbatim && r.Records != "", Overwrite: req.Overwrite, Prune: req.Prune},
		Main:      w.Main,
		Numbered:  r.Numbered,
		Pending:   r.Pending,
		RunErr:    f.RunErr,
	})
	reconciled := settled.Reconciled
	r.Unsettled = settled.Unsettled
	r.settle = settle{reconcile: settled.Params, resolve: resolve, planned: reconciled}
	r.Files = files(reconciled, req.ShowValues)
	reconciled.Ports.Applied = false
	r.Ports = shown(reconciled.Ports, req.ShowValues)
	r.Restart = f.restart(w, r.Files)
	return r
}

// shown is plan with its values when they are shown, else without them.
func shown(plan envs.Plan, show bool) envs.Redacted {
	if show {
		return envs.Redacted(plan)
	}
	return envs.Redact(plan)
}

// files are the files reconciled as the plan shows them, their values only when shown.
func files(reconciled envs.Reconciled, show bool) []File {
	out := make([]File, 0, len(reconciled.Files))
	for _, rf := range reconciled.Files {
		file := File{Target: rf.Target, Created: rf.Created, Keys: []Key{}, Writes: rf.Changed()}
		add := func(key envs.Key, settled bool) {
			k := Key{Key: key.Name, Status: key.Status, Settled: settled}
			if show {
				k.Value, k.Source = &key.Value, &key.Source
			}
			file.Keys = append(file.Keys, k)
		}
		for _, key := range rf.Settled {
			add(key, true)
		}
		for _, key := range rf.Left {
			add(key, false)
		}
		out = append(out, file)
	}
	return out
}

// restart are the jobs up in w, by name, sorted, whose directory holds a file that changes: they read it when they started.
func (f Facts) restart(w worktree.Worktree, files []File) []string {
	dirs := map[string]bool{}
	for _, file := range files {
		if file.Writes {
			dirs[path.Dir(file.Target)] = true
		}
	}

	restart := []string{}
	for _, job := range f.Jobs {
		declared := slices.IndexFunc(f.Run.Jobs, func(j project.Job) bool { return j.Name == job.Name })
		if job.WorkDir != w.Path || !job.Up || declared < 0 || !dirs[cmp.Or(path.Clean(f.Run.Jobs[declared].Cwd), ".")] || slices.Contains(restart, job.Name) {
			continue
		}
		restart = append(restart, job.Name)
	}
	slices.Sort(restart)
	return restart
}

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts and the resolved request into the plan and its warnings: each worktree asked, those that change and those left as they are.
func Decide(f Facts, req Request) (Plan, []domain.Warning, error) {
	reconciles, _ := f.entries(req)
	if reconciles == nil {
		reconciles = []Reconcile{}
	}

	planned := Plan{Worktrees: reconciles, Overwrite: req.Overwrite, Prune: req.Prune, ShowValues: req.ShowValues}
	return planned, f.warnings(planned), nil
}

// The codes of the warnings: a link of run.toml into a file the project does not declare, a worktree whose ports cannot be settled, a worktree whose directory is gone, one whose files could not be read, and the jobs that run on the old values until restarted, followed by their restart.
const (
	CodePortIgnored  = Name + ".env_port_ignored"
	CodeValueIgnored = Name + ".env_value_ignored"
	CodeUnsettled    = Name + ".ports_not_settled"
	CodeMissing      = Name + ".missing"
	CodeUnreadable   = Name + ".unreadable"
	CodeRestart      = Name + ".restart"
	CodeRestartJob   = Name + ".restart_job"
	CodeClean        = Name + ".clean"
	CodePending      = Name + ".isolation_pending"
	CodeIsolate      = Name + ".isolate"
)

// pathRunStartJob and pathRunStartRestart are the paths of run start's job and restart, which a follow-up of run start answers.
const (
	pathRunStartJob     = "job"
	pathRunStartRestart = "restart"
)

// warnings are what the plan cannot see to or leaves as it is: each link of run.toml into a file the project does not declare, then for each worktree asked whose directory is gone, whose files could not be read, or whose ports cannot be settled, why.
func (f Facts) warnings(planned Plan) []domain.Warning {
	var warnings []domain.Warning
	if f.RunErr == nil {
		_, ignored := envs.Targets(f.Run, f.Config.Env.Files)
		for _, link := range ignored {
			code := CodePortIgnored
			if link.Kind == envs.LinkValue {
				code = CodeValueIgnored
			}
			warnings = append(warnings, domain.Warning{Code: code, Message: string(link.Kind) + " " + link.Key + " in " + link.File + " ignored: not a configured env file — add it to [env] in config.toml or drop the link", Params: domain.Params{"key": link.Key, "file": link.File}})
		}
	}

	for _, r := range planned.Worktrees {
		params := domain.Params{"worktree": r.Worktree}
		switch {
		case r.Missing:
			warnings = append(warnings, domain.Warning{Code: CodeMissing, Message: r.Worktree + "'s directory is gone: left as it is", Params: params, FollowUp: &domain.FollowUp{Command: domain.CommandClean, Answers: on(r.Worktree), Code: CodeClean, Message: "remove what is left of it"}})
		case f.ReadErrs[r.Worktree] != nil:
			params["cause"] = f.ReadErrs[r.Worktree].Error()
			warnings = append(warnings, domain.Warning{Code: CodeUnreadable, Message: "couldn't read " + r.Worktree + "'s .env files: " + params["cause"] + " — left as it is", Params: params})
		case r.Pending:
			isolate := domain.FollowUp{Command: Name, Answers: append(on(r.Worktree), domain.Answered{Path: domain.PathIsolation, Values: []string{string(project.Isolated)}}), Code: CodeIsolate, Message: "give it ports of its own"}
			warnings = append(warnings, domain.Warning{Code: CodePending, Message: r.Worktree + " has yet to choose its isolation: its ports and the values wtm owns are left as they are", Params: params, FollowUp: &isolate})
		case r.Unsettled != "":
			params["cause"] = r.Unsettled
			warnings = append(warnings, domain.Warning{Code: CodeUnsettled, Message: r.Unsettled + " — " + r.Worktree + "'s ports not settled", Params: params})
		}
	}

	return warnings
}

func on(worktree string) []domain.Answered {
	return []domain.Answered{{Path: domain.PathWorktree, Values: []string{worktree}}}
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: every worktree asked, what each one's files become and those left as they are, and the switches of the run; then the recap, which the fingerprint leaves out.
type Plan struct {
	Worktrees  []Reconcile `json:"worktrees"`
	Overwrite  bool        `json:"overwrite"`
	Prune      bool        `json:"prune"`
	ShowValues bool        `json:"show_values"`
	domain.Recapped
}

// Locks are the locks the plan needs: the repository shared, and each worktree it writes exclusive.
func (p Plan) Locks() []domain.Lock {
	var names []string
	for _, r := range p.Worktrees {
		if r.writes() {
			names = append(names, r.Worktree)
		}
	}

	return domain.WorktreeLocks(names)
}

// Fingerprinted is what the plan's fingerprint covers: the plan, its recap left out, and each file as it stands and as the apply writes it, so a file edited since the preview refuses its apply.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	var contents []string
	for _, r := range p.Worktrees {
		for _, file := range r.settle.planned.Files {
			contents = append(contents, r.Worktree+"\x00"+file.Target+"\x00"+string(dotenv.Render(file.Before))+"\x00"+string(dotenv.Render(file.After)))
		}
	}
	return struct {
		Plan     Plan     `json:"plan"`
		Contents []string `json:"contents"`
	}{p, contents}
}

// Locks are the locks taken before planning: the repository shared, and each worktree named exclusive; every worktree's are taken once the plan names those it writes.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(slices.DeleteFunc(slices.Clone(r.Worktrees), func(name string) bool { return strings.TrimSpace(name) == "" }))
}

// ----------------------------------------------------------------------------
// The recap

// The notes of the recap's values: a file created, a key added, overwritten, dropped, or left with its status, a linked value rewritten, a value wtm owns set, a worktree numbered; a worktree in step, one whose directory is gone.
const (
	NoteCreated     = "created"
	NoteAdded       = "added"
	NoteOverwritten = "overwritten"
	NotePruned      = "pruned"
	NoteLeft        = "left"
	NoteRewrite     = "rewrite"
	NoteOwned       = "owned"
	NoteNumbered    = "numbered"
	NoteIsolation   = "isolation"
	NoteInStep      = "in_step"
	NoteMissing     = "missing"
)

// empty reports whether applying the plan changes nothing.
func (p Plan) empty() bool {
	return !slices.ContainsFunc(p.Worktrees, Reconcile.writes)
}

// recap is the plan row by row: a row per worktree that changes, a value per change, the keys it leaves after them; then the worktrees in step, then those left as they are.
func (p Plan) recap() []domain.RecapRow {
	var rows []domain.RecapRow
	var current, left []domain.RecapValue
	for _, r := range p.Worktrees {
		switch {
		case r.Missing:
			left = append(left, domain.RecapValue{Value: r.Worktree, Note: NoteMissing, Text: r.Worktree + " (its directory is gone)"})
		case r.writes() || r.leaves():
			// a row per change, the worktree named on its first: a line each
			for i, value := range r.recap() {
				row := domain.RecapRow{Key: "worktree", Path: domain.PathWorktree, Values: []domain.RecapValue{value}}
				if i == 0 {
					row.Label = r.Worktree
				}
				rows = append(rows, row)
			}
		default:
			current = append(current, domain.RecapValue{Value: r.Worktree, Note: NoteInStep, Text: r.Worktree})
		}
	}

	if len(current) > 0 {
		rows = append(rows, domain.RecapRow{Key: "in_step", Label: "In step", Path: domain.PathWorktree, Values: current})
	}
	if len(left) > 0 {
		rows = append(rows, domain.RecapRow{Key: "left", Label: "Left as is", Path: domain.PathWorktree, Values: left})
	}
	return rows
}

// leaves reports whether r leaves a key out of step.
func (r Reconcile) leaves() bool {
	return slices.ContainsFunc(r.Files, func(f File) bool {
		return slices.ContainsFunc(f.Keys, func(k Key) bool { return !k.Settled })
	})
}

// keyNotes are the notes of a key settled, by its status, and the words of a key left.
var (
	keyNotes  = map[envs.KeyStatus]string{envs.KeyAdd: NoteAdded, envs.KeyConflict: NoteOverwritten, envs.KeyOrphan: NotePruned}
	keyMarks  = map[envs.KeyStatus]string{envs.KeyAdd: "+ ", envs.KeyConflict: "~ ", envs.KeyOrphan: "− "}
	leftWords = map[envs.KeyStatus]string{envs.KeyMissing: "no source gives it a value", envs.KeyConflict: "differs from its source", envs.KeyOrphan: "in no source"}
)

// recap is r's changes, a value each: a worktree numbered, each file created, each key settled, each linked value rewritten and each value wtm owns set, then each key left with why.
func (r Reconcile) recap() []domain.RecapValue {
	var values, left []domain.RecapValue
	if r.Records != "" {
		values = append(values, domain.RecapValue{Value: string(r.Records), Note: NoteIsolation, Text: "now " + string(r.Records)})
	}
	if !r.Numbered {
		values = append(values, domain.RecapValue{Value: r.Worktree, Note: NoteNumbered, Text: "numbered, then its ports settled"})
	}
	for _, file := range r.Files {
		if file.Created {
			values = append(values, domain.RecapValue{Value: file.Target, Note: NoteCreated, Params: domain.Params{"file": file.Target, "from": string(r.Strategy)}, Text: file.Target + "  created from " + string(r.Strategy)})
		}
		for _, key := range file.Keys {
			params := domain.Params{"file": file.Target, "key": key.Key, "status": string(key.Status)}
			if key.Settled {
				values = append(values, domain.RecapValue{Value: key.Key, Note: keyNotes[key.Status], Params: params, Text: file.Target + "  " + keyMarks[key.Status] + key.Key})
				continue
			}
			left = append(left, domain.RecapValue{Value: key.Key, Note: NoteLeft, Params: params, Text: file.Target + "  " + key.Key + " left: " + leftWords[key.Status]})
		}
	}
	for _, entry := range envs.Plan(r.Ports).Rewrites() {
		to := strconv.Itoa(entry.Resolved)
		if entry.Addressing == project.AddressNames && len(entry.Moves) > 0 && entry.Moves[0].Origin != "" {
			to = entry.Moves[0].Origin
		}
		text := entry.File + "  " + entry.Key + " → " + to
		if entry.NewValue != "" {
			text = entry.File + "  " + entry.Key + " " + entry.CurrentValue + " → " + entry.NewValue
		}
		values = append(values, domain.RecapValue{Value: entry.Key, Note: NoteRewrite, Params: domain.Params{"file": entry.File, "key": entry.Key, "to": to}, Text: text})
	}
	for _, owned := range r.Ports.Owned {
		if owned.Changed {
			values = append(values, domain.RecapValue{Value: owned.Key, Note: NoteOwned, Params: domain.Params{"file": owned.File, "key": owned.Key}, Text: owned.File + "  " + owned.Key + " set"})
		}
	}
	return append(values, left...)
}

// ----------------------------------------------------------------------------
// The outcome

// CodeFailed is a worktree whose files could not be written, or that could not be numbered.
const CodeFailed = Name + ".failed"

// restartWarning is the warning that the jobs of r up run on the old values until restarted, followed by the restart of the first one.
func restartWarning(r Reconcile) domain.Warning {
	jobs, verb := strings.Join(r.Restart, ", "), " run"
	if len(r.Restart) == 1 {
		verb = " runs"
	}
	return domain.Warning{
		Code:     CodeRestart,
		Message:  jobs + verb + " on the old values until restarted",
		Params:   domain.Params{"worktree": r.Worktree, "jobs": jobs},
		FollowUp: &domain.FollowUp{Command: domain.CommandRunStart, Code: CodeRestartJob, Message: "restart it", Answers: append(on(r.Worktree), domain.Answered{Path: pathRunStartJob, Values: []string{r.Restart[0]}}, domain.Answered{Path: pathRunStartRestart, Values: domain.SaidYes(true)})},
	}
}

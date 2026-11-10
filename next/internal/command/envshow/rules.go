package envshow

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	envs "github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what env show reads, as Observe read them: the project's config, run.toml and the repository's snapshot, the worktree read, the record of each live branch, its .env files with their sources, and the port a named address announces.
type Facts struct {
	worktree.Repository
	Target     worktree.Worktree
	Records    map[string]worktree.Meta
	Reads      []envs.FileRead
	PublicPort int
	ShowValues bool
}

// record is the target's meta record.
func (f Facts) record() worktree.Meta {
	return f.Records[f.Target.Branch()]
}

// strategy is where the target's values come from: its record's strategy, else the project's, else the default.
func (f Facts) strategy() project.EnvStrategy {
	return cmp.Or(f.record().EnvStrategy, f.Config.Env.Strategy, project.DefaultEnvStrategy)
}

// isolation is how the target runs: the main checkout always isolated, another as its record says, isolated when it says nothing.
func (f Facts) isolation() project.Isolation {
	if f.Target.Main {
		return project.Isolated
	}
	return f.record().Isolation.Effective()
}

// pending reports whether the target has yet to choose its isolation.
func (f Facts) pending() bool {
	return !f.Target.Main && f.RunErr == nil && f.record().Isolation == "" && f.Run.IsolationApplies()
}

// ----------------------------------------------------------------------------
// The detail

// A Detail is a worktree's .env files as env show reports them, app by app, file by file, key by key: the worktree, its env strategy and isolation, whether it has yet to choose one, why its ports are not settled when they are not, each app (a directory holding a declared file), and the keys to check, in the table's order.
type Detail struct {
	Worktree  string              `json:"worktree"`
	Path      string              `json:"path"`
	Main      bool                `json:"main"`
	Strategy  project.EnvStrategy `json:"strategy"`
	Isolation project.Isolation   `json:"isolation"`
	Pending   bool                `json:"pending"`
	Unsettled string              `json:"unsettled,omitempty"`
	Apps      []App               `json:"apps"`
	Checks    []Check             `json:"checks"`
}

// An App is a directory holding declared .env files: the jobs run.toml runs there, how many of its keys there are, follow a job and are to check, and its files.
type App struct {
	Dir       string   `json:"dir"`
	Jobs      []string `json:"jobs"`
	Keys      int      `json:"keys"`
	Following int      `json:"following"`
	ToCheck   int      `json:"to_check"`
	Files     []File   `json:"files"`
}

// Origin is where a worktree's .env file comes from.
type Origin string

// The origins of a file: its template, the main worktree's copy, the parent worktree's, the main worktree's because the parent has none (Fallback), the main checkout's own file (Own), or none, its source lacking it (None).
const (
	OriginTemplate Origin = "template"
	OriginMain     Origin = "main"
	OriginParent   Origin = "parent"
	OriginFallback Origin = "fallback"
	OriginOwn      Origin = "own"
	OriginNone     Origin = "none"
)

// Origins returns every Origin.
func Origins() []Origin {
	return []Origin{OriginTemplate, OriginMain, OriginParent, OriginFallback, OriginOwn, OriginNone}
}

// A File is one declared .env file of the worktree: its path and name, where it comes from, the branch it comes from when it is the parent's, whether the worktree lacks it, and its keys.
type File struct {
	Target  string `json:"target"`
	Name    string `json:"name"`
	Origin  Origin `json:"origin"`
	Parent  string `json:"parent,omitempty"`
	Missing bool   `json:"missing"`
	Keys    []Key  `json:"keys"`
}

// Follows is what a key's value follows.
type Follows string

// What a key follows: a job's port, a job's address, a value of run.toml built on a job, a value wtm owns, nothing although it looks like a port or an address (Nothing), or nothing but its source (Copied).
const (
	FollowsPort    Follows = "port"
	FollowsAddress Follows = "address"
	FollowsValue   Follows = "value"
	FollowsOwned   Follows = "owned"
	FollowsNothing Follows = "nothing"
	FollowsCopied  Follows = "copied"
)

// AllFollows returns every Follows.
func AllFollows() []Follows {
	return []Follows{FollowsPort, FollowsAddress, FollowsValue, FollowsOwned, FollowsNothing, FollowsCopied}
}

// A Key is one key of a file: what it follows and the job and port it follows (or looks like, when it follows nothing), its value in the main checkout and here, nil when absent there, and what wtm writes here, nil when it writes nothing; whether the values are withheld, wtm writing none of them and none shown; how it stands against its source; and whether it is out of step with what wtm writes.
type Key struct {
	Key       string         `json:"key"`
	Follows   Follows        `json:"follows"`
	Job       string         `json:"job,omitempty"`
	Port      string         `json:"port,omitempty"`
	Main      *string        `json:"main"`
	Here      *string        `json:"here"`
	Writes    *string        `json:"writes,omitempty"`
	Withheld  bool           `json:"withheld"`
	Status    envs.KeyStatus `json:"status"`
	OutOfStep bool           `json:"out_of_step"`
}

// A Check is a key to check: one that follows nothing although it looks like a port or an address, with the job and port it looks like (none for an address on no declared port), or one out of step with what wtm writes, with its value here and what wtm writes.
type Check struct {
	File   string    `json:"file"`
	Key    string    `json:"key"`
	Kind   CheckKind `json:"kind"`
	Job    string    `json:"job,omitempty"`
	Port   string    `json:"port,omitempty"`
	Here   *string   `json:"here,omitempty"`
	Writes *string   `json:"writes,omitempty"`
}

// CheckKind is why a key is to check.
type CheckKind string

// The kinds of check.
const (
	CheckUnlinked  CheckKind = "unlinked"
	CheckOutOfStep CheckKind = "out_of_step"
)

// CheckKinds returns every CheckKind.
func CheckKinds() []CheckKind {
	return []CheckKind{CheckUnlinked, CheckOutOfStep}
}

// The codes of the warnings: a worktree yet to choose its isolation, followed by env giving it one; ports that cannot be settled.
const (
	CodePending   = Name + ".isolation_pending"
	CodeIsolate   = Name + ".isolate"
	CodeUnsettled = Name + ".ports_not_settled"
)

// ----------------------------------------------------------------------------
// Show

// Show reports the target's .env files from the facts, and the warnings of what keeps wtm from writing into them.
func Show(f Facts) (Item, []domain.Warning) {
	w := f.Target
	settled := envs.Settle(envs.SettleParams{
		Resolve:   envs.ResolveParams{JobParams: envs.JobParams{Branch: w.Branch(), ProjectDir: f.MainPath, Ordinal: f.record().Ordinal, Isolation: f.isolation(), Run: f.Run}, Files: f.Config.Env.Files, Proxy: f.Global.Proxy, PublicPort: f.PublicPort},
		Reconcile: envs.ReconcileParams{Reads: f.Reads, Strategy: f.strategy(), Run: f.Run, Verbatim: f.isolation() == project.Verbatim},
		Main:      w.Main,
		Numbered:  w.Main || f.record().Ordinal > 0 || !envs.NeedsOrdinal(f.isolation(), f.Run),
		Pending:   f.pending(),
		RunErr:    f.RunErr,
	})

	detail := Detail{Worktree: w.Name(), Path: w.Path, Main: w.Main, Strategy: f.strategy(), Isolation: f.isolation(), Pending: f.pending(), Unsettled: settled.Unsettled, Apps: []App{}, Checks: []Check{}}
	writes := writtenBy(settled.Ports)
	for _, read := range f.Reads {
		file := f.file(read, writes)
		detail.Apps = appOf(detail.Apps, f.Run, file)
		for _, key := range file.Keys {
			if check, ok := checkOf(file.Target, key); ok {
				detail.Checks = append(detail.Checks, check)
			}
		}
	}

	var warnings []domain.Warning
	params := domain.Params{"worktree": w.Name()}
	switch {
	case detail.Pending:
		isolate := domain.FollowUp{Command: domain.CommandEnv, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{w.Name()}}, {Path: domain.PathIsolation, Values: []string{string(project.Isolated)}}}, Code: CodeIsolate, Message: "give it ports of its own"}
		warnings = append(warnings, domain.Warning{Code: CodePending, Message: w.Name() + " has yet to choose its isolation: wtm writes nothing in its .env files until it does", Params: params, FollowUp: &isolate})
	case detail.Unsettled != "":
		params["cause"] = detail.Unsettled
		warnings = append(warnings, domain.Warning{Code: CodeUnsettled, Message: detail.Unsettled + " — what wtm writes is unknown", Params: params})
	}
	return Item{Subject: w.Name(), Status: domain.StatusDone, Detail: &detail}, warnings
}

// written is what wtm writes into one key: the value, and whether the key holds another one.
type written struct {
	value     string
	outOfStep bool
}

// writtenBy is what wtm writes into each key, by file then key: its linked ports and addresses, then its own values.
func writtenBy(plan envs.Plan) map[string]map[string]written {
	byFile := map[string]map[string]written{}
	set := func(file, key string, w written) {
		if byFile[file] == nil {
			byFile[file] = map[string]written{}
		}
		byFile[file][key] = w
	}
	for _, entry := range plan.Entries {
		switch entry.Status {
		case envs.StatusRewrite:
			set(entry.File, entry.Key, written{value: entry.NewValue, outOfStep: true})
		case envs.StatusUnchanged:
			set(entry.File, entry.Key, written{value: entry.CurrentValue})
		}
	}
	for _, owned := range plan.Owned {
		set(owned.File, owned.Key, written{value: owned.Value, outOfStep: owned.Changed})
	}
	return byFile
}

// file reports one file read: where it comes from, and each key of it, of its template and of its source.
func (f Facts) file(read envs.FileRead, writes map[string]map[string]written) File {
	file := File{Target: read.File.Target, Name: path.Base(read.File.Target), Missing: read.Child == nil, Keys: []Key{}}
	file.Origin, file.Parent = f.origin(read)

	parent, main := read.Sources(f.strategy())
	here, inMain := pairs(read.Child), pairs(read.Main)
	if f.Target.Main {
		inMain = here
	}
	for _, k := range envs.Keys(envs.DriftParams{Child: read.Child, Template: read.Template, Parent: parent, Main: main}) {
		key := Key{Key: k.Name, Status: k.Status, Follows: FollowsCopied}
		key.Follows, key.Job, key.Port = f.follows(read.File.Target, k.Name, valueOf(inMain, here, k.Name))
		if w, wtm := writes[read.File.Target][k.Name]; wtm {
			key.Writes, key.OutOfStep = &w.value, w.outOfStep
		}
		key.Main, key.Here = lookup(inMain, k.Name), lookup(here, k.Name)
		if key.Writes == nil && key.Follows != FollowsPort && key.Follows != FollowsAddress && key.Follows != FollowsValue && key.Follows != FollowsOwned && !f.ShowValues {
			key.Withheld = true
			key.Main, key.Here = withheld(key.Main), withheld(key.Here)
		}
		file.Keys = append(file.Keys, key)
	}
	return file
}

// origin is where the target's copy of read comes from, and the parent branch when it is the parent's.
func (f Facts) origin(read envs.FileRead) (Origin, string) {
	if f.Target.Main {
		return OriginOwn, ""
	}
	strategy, source := f.strategy(), f.record().SourceBranch
	switch {
	case strategy == project.EnvExample && read.Template != nil:
		return OriginTemplate, ""
	case strategy == project.EnvParent && read.Parent != nil:
		return OriginParent, source
	case strategy == project.EnvParent && read.Main != nil:
		return OriginFallback, source
	case strategy == project.EnvMain && read.Main != nil:
		return OriginMain, ""
	}
	return OriginNone, ""
}

// follows is what the key of file follows, by run.toml's links, and the job and port it follows or looks like; a key no link follows is copied, unless its value looks like a port or an address, when it follows nothing.
func (f Facts) follows(file, key, value string) (Follows, string, string) {
	for _, link := range f.Run.EnvPorts {
		if link.File == file && link.Key == key {
			if f.Run.Addressing == project.AddressNames || hasScheme(value) {
				return FollowsAddress, link.Job, link.Port
			}
			return FollowsPort, link.Job, link.Port
		}
	}
	for _, link := range f.Run.EnvValues {
		if link.File == file && link.Key == key {
			return FollowsValue, link.Job, ""
		}
	}
	if envs.Owned(project.Run{}, file)[key] {
		return FollowsOwned, "", ""
	}
	if like, looks := envs.LooksLinked(value, f.Run); looks {
		return FollowsNothing, like.Job, like.Port
	}
	return FollowsCopied, "", ""
}

// checkOf is the check of key in file, and whether it has one.
func checkOf(file string, key Key) (Check, bool) {
	switch {
	case key.Follows == FollowsNothing:
		return Check{File: file, Key: key.Key, Kind: CheckUnlinked, Job: key.Job, Port: key.Port}, true
	case key.OutOfStep:
		return Check{File: file, Key: key.Key, Kind: CheckOutOfStep, Job: key.Job, Port: key.Port, Here: key.Here, Writes: key.Writes}, true
	}
	return Check{}, false
}

// appOf is apps with file in its app, the directory holding it, added with the jobs run.toml runs there when it is the first file there.
func appOf(apps []App, run project.Run, file File) []App {
	dir := path.Dir(file.Target)
	at := slices.IndexFunc(apps, func(a App) bool { return a.Dir == dir })
	if at < 0 {
		app := App{Dir: dir, Jobs: []string{}}
		for _, job := range run.Jobs {
			if path.Clean(job.Cwd) == dir {
				app.Jobs = append(app.Jobs, job.Name)
			}
		}
		apps, at = append(apps, app), len(apps)
	}

	app := &apps[at]
	app.Files = append(app.Files, file)
	for _, key := range file.Keys {
		app.Keys++
		if key.Follows == FollowsPort || key.Follows == FollowsAddress || key.Follows == FollowsValue {
			app.Following++
		}
		if _, check := checkOf(file.Target, key); check {
			app.ToCheck++
		}
	}
	return apps
}

func pairs(lines []dotenv.Line) map[string]string {
	values := map[string]string{}
	for _, line := range lines {
		if line.Kind == dotenv.Pair {
			values[line.Key] = line.Value
		}
	}
	return values
}

// valueOf is the key's value in main, else here: the one a link's base port is read in.
func valueOf(main, here map[string]string, key string) string {
	if value, ok := main[key]; ok {
		return value
	}
	return here[key]
}

func lookup(values map[string]string, key string) *string {
	value, ok := values[key]
	if !ok {
		return nil
	}
	return &value
}

// withheld is a value withheld: present, but empty.
func withheld(value *string) *string {
	if value == nil {
		return nil
	}
	empty := ""
	return &empty
}

func hasScheme(value string) bool {
	return strings.Contains(value, "://")
}

package env

import (
	"bytes"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// A FileRead is one .env file the project declares, as read for a worktree: the worktree's own copy and what it is rebuilt and compared from, its template, the parent worktree's copy and the main worktree's, each nil when it does not exist.
type FileRead struct {
	File     project.EnvFile
	Child    []dotenv.Line
	Template []dotenv.Line
	Parent   []dotenv.Line
	Main     []dotenv.Line
}

// Sources are the copies r's values come from under strategy: none with the template's, the main worktree's, or the parent's when it has one, else the main's.
func (r FileRead) Sources(strategy project.EnvStrategy) (parent, main []dotenv.Line) {
	switch {
	case strategy == project.EnvMain:
		return nil, r.Main
	case strategy == project.EnvParent && r.Parent != nil:
		return r.Parent, nil
	case strategy == project.EnvParent:
		return nil, r.Main
	}

	return nil, nil
}

// Scaffold is what r starts from when the worktree lacks it, the copy create would have made: the template under the example strategy, the strategy's source otherwise; nil when there is none.
func (r FileRead) Scaffold(strategy project.EnvStrategy) []dotenv.Line {
	if strategy == project.EnvExample {
		return r.Template
	}
	parent, main := r.Sources(strategy)
	if parent != nil {
		return parent
	}
	return main
}

// ReconcileParams describe a worktree's .env files to reconcile: each one read, the worktree's env strategy, run.toml, whether its isolation is verbatim and whether it just switched to it (Restore: the values wtm wrote go back to the source's), what wtm writes into them (Resolve), and whether a conflict takes its source's value (Overwrite) and an orphan is dropped (Prune).
type ReconcileParams struct {
	Reads     []FileRead
	Strategy  project.EnvStrategy
	Run       project.Run
	Verbatim  bool
	Restore   bool
	Writes    Writes
	Overwrite bool
	Prune     bool
}

// Reconciled is a worktree's .env files reconciled: each file, and the plan of what wtm writes into them, decided over the files once their keys were settled.
type Reconciled struct {
	Files []FileReconciled
	Ports Plan
}

// A FileReconciled is one .env file reconciled: Created when the worktree lacks it and it is rebuilt, its keys not in step with their source, those settled and the others Left as they are, and its lines Before (nil when absent) and After (nil when it stays absent).
type FileReconciled struct {
	Target  string
	Created bool
	Settled []Key
	Left    []Key
	Before  []dotenv.Line
	After   []dotenv.Line
}

// Changed reports whether f has to be written.
func (f FileReconciled) Changed() bool {
	return f.Created || !bytes.Equal(dotenv.Render(f.Before), dotenv.Render(f.After))
}

// Reconcile settles each of params' files: a missing one rebuilt from its scaffold, its keys added from their source, a conflict overwritten and an orphan dropped when asked, then what wtm writes set over them, a file wtm owns a value in created when missing. The keys wtm writes are never compared with a source, but under the verbatim isolation, where it writes none: a worktree that just switched to it gets their source's values back, else the main worktree's, as without wtm.
func Reconcile(params ReconcileParams) Reconciled {
	settledLines := map[string][]dotenv.Line{}
	files := make([]FileReconciled, 0, len(params.Reads))
	for _, read := range params.Reads {
		file := FileReconciled{Target: read.File.Target, Before: read.Child}
		base := read.Child
		if base == nil {
			base = read.Scaffold(params.Strategy)
			file.Created = base != nil
		}

		parent, main := read.Sources(params.Strategy)
		owned, compared := Owned(params.Run, read.File.Target), map[string]bool{}
		if !params.Verbatim || params.Restore {
			compared = owned
		}
		if params.Restore {
			file.Settled = restored(base, owned, cascaded(read, parent, main))
		}
		for _, key := range Keys(DriftParams{Child: base, Template: read.Template, Parent: parent, Main: main, Owned: compared}) {
			switch {
			case key.Status == KeyInStep:
			case params.settles(key):
				file.Settled = append(file.Settled, key)
			default:
				file.Left = append(file.Left, key)
			}
		}

		lines := ApplyKeys(base, file.Settled)
		if lines == nil && params.Writes.Creates(file.Target) {
			lines, file.Created = []dotenv.Line{}, true
		}
		settledLines[file.Target] = lines
		if lines != nil {
			file.After = params.Writes.Edit(file.Target, lines)
		}
		files = append(files, file)
	}

	return Reconciled{Files: files, Ports: params.Writes.Settled(settledLines)}
}

// cascaded are the values a key wtm wrote goes back to under the verbatim isolation: its source's, else the main worktree's, as without wtm, the template giving none.
func cascaded(read FileRead, parent, main []dotenv.Line) map[string]string {
	values := pairs(read.Main)
	for _, source := range []map[string]string{pairs(main), pairs(parent)} {
		for key, value := range source {
			if value != "" {
				values[key] = value
			}
		}
	}
	return values
}

// restored are the keys wtm wrote into lines that go back to their value in values, as conflicts settled.
func restored(lines []dotenv.Line, owned map[string]bool, values map[string]string) []Key {
	var keys []Key
	seen := map[string]bool{}
	for _, line := range lines {
		value, back := values[line.Key]
		if line.Kind != dotenv.Pair || !owned[line.Key] || seen[line.Key] || !back || value == line.Value {
			continue
		}
		seen[line.Key] = true
		keys = append(keys, Key{Name: line.Key, Status: KeyConflict, Value: line.Value, Source: value})
	}
	return keys
}

// settles reports whether key is settled on its source: an addition always, a conflict when overwritten, an orphan when pruned.
func (p ReconcileParams) settles(key Key) bool {
	switch key.Status {
	case KeyAdd:
		return true
	case KeyConflict:
		return p.Overwrite
	case KeyOrphan:
		return p.Prune
	}
	return false
}

// SettleParams describe a worktree whose .env files are reconciled: what its writes resolve from and how its files are reconciled (their Writes left to Settle), whether it is the main checkout, whether it holds the ordinal its ports need (Numbered), whether it has yet to choose its isolation (Pending), and why run.toml could not be read (RunErr).
type SettleParams struct {
	Resolve   ResolveParams
	Reconcile ReconcileParams
	Main      bool
	Numbered  bool
	Pending   bool
	RunErr    error
}

// Settled is a worktree's .env files reconciled with what wtm writes into them, the params they were reconciled with, and why its ports are not settled when they are not ("" when they are).
type Settled struct {
	Reconciled
	Params    ReconcileParams
	Unsettled string
}

// Settle reconciles a worktree's .env files with what wtm writes into them, the one place both env and its read decide it: nothing when run.toml cannot be read or the worktree has yet to choose its isolation; only the values wtm owns when it is not numbered yet, or when a value of run.toml cannot be expanded or needs a proxy that serves none; the main checkout, addressed as without wtm, keeps its addresses spelled with ports when its .env spells them so.
func Settle(params SettleParams) Settled {
	reconcile := params.Reconcile
	if params.RunErr != nil || params.Pending {
		reconcile.Writes = Writes{}
		settled := Settled{Reconciled: Reconcile(reconcile), Params: reconcile}
		if params.RunErr != nil {
			settled.Unsettled = params.RunErr.Error()
		}
		return settled
	}

	resolve := params.Resolve
	var unsettled string
	reconcile.Writes, unsettled = resolved(resolve, params.Numbered)
	reconciled := Reconcile(reconcile)
	if params.Main && reconciled.Ports.SpelledByPorts() {
		resolve.Run.Addressing = project.AddressPorts
		reconcile.Writes, unsettled = resolved(resolve, params.Numbered)
		reconciled = Reconcile(reconcile)
	}
	return Settled{Reconciled: reconciled, Params: reconcile, Unsettled: unsettled}
}

// resolved are the writes resolve gives, only the values wtm owns when the worktree is not numbered, and why its ports are not settled when resolving them fails.
func resolved(resolve ResolveParams, numbered bool) (Writes, string) {
	kept, _ := Targets(resolve.Run, resolve.Files)
	owned := kept
	owned.EnvPorts, owned.EnvValues = nil, nil
	if !numbered {
		resolve.Run = owned
	}
	writes, err := Resolve(resolve)
	if err == nil {
		return writes, ""
	}

	resolve.Run = owned
	writes, _ = Resolve(resolve)
	return writes, err.Error()
}

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

// Reconcile settles each of params' files: a missing one rebuilt from its scaffold, its keys added from their source, a conflict overwritten and an orphan dropped when asked, then what wtm writes set over them, a file wtm owns a value in created when missing. The keys wtm writes are never compared with a source, but under the verbatim isolation, where it writes none: a worktree that just switched to it gets their source's values back.
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
		if !params.Verbatim {
			compared = owned
		}
		for _, key := range Keys(DriftParams{Child: base, Template: read.Template, Parent: parent, Main: main, Owned: compared}) {
			switch {
			case key.Status == KeyInStep:
			case params.settles(key) || params.Restore && owned[key.Name] && key.Status == KeyConflict:
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

// Package settle decides what wtm writes into a new worktree's .env files: its ports, its addresses, its compose project and its [[env]] values.
package settle

import (
	"errors"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/jobenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

type NeedsOrdinalParams struct {
	Isolation run.Isolation
	Run       run.Config
}

func NeedsOrdinal(params NeedsOrdinalParams) bool {
	if params.Isolation.Effective() == run.Verbatim {
		return false
	}
	return len(params.Run.EnvPorts) > 0 || len(params.Run.EnvValues) > 0
}

type ResolveParams struct {
	jobenv.Params
	Files      []project.EnvFile
	Proxy      project.Proxy
	PublicPort int
}

type Writes struct {
	links    []run.EnvPortLink
	owned    []OwnedEntry
	bases    map[portRef]int
	block    int
	worktree worktree
}

func Resolve(params ResolveParams) (Writes, error) {
	if params.Isolation.Effective() == run.Verbatim {
		return Writes{}, nil
	}
	kept, _ := Targets(TargetsParams{Run: params.Run, Files: params.Files})
	owned := ownedWrites(ownedWritesParams{Run: kept, Files: params.Files, ComposeProject: params.ComposeProject()})
	if len(kept.EnvPorts) == 0 && len(kept.EnvValues) == 0 {
		return Writes{owned: owned}, nil
	}
	if err := params.Proxy.Problem(); err != nil {
		return Writes{}, err
	}
	settled := worktree{
		Addressing: effectiveAddressing(kept),
		Jobs:       jobsByName(kept),
		Slug:       branch.Slug(params.Branch),
		Project:    params.Project(),
		PublicPort: params.PublicPort,
		Ordinal:    params.Ordinal,
		Offset:     params.Offset(),
		Shared:     jobenv.SharedJobs(kept),
	}
	values, err := settled.valueWrites(kept.EnvValues)
	if errors.Is(err, errNoProxy) {
		return Writes{}, err
	}
	if err != nil {
		return Writes{}, &ValueError{Err: err}
	}
	return Writes{links: kept.EnvPorts, owned: append(owned, values...), bases: portBases(kept), block: jobenv.OffsetBlock(kept), worktree: settled}, nil
}

func (w Writes) Empty() bool {
	return len(w.links) == 0 && len(w.owned) == 0
}

func (w Writes) Files() []string {
	var files []string
	seen := map[string]bool{}
	add := func(file string) {
		if !seen[file] {
			seen[file] = true
			files = append(files, file)
		}
	}
	for _, link := range w.links {
		add(link.File)
	}
	for _, entry := range w.owned {
		add(entry.File)
	}
	return files
}

func (w Writes) Creates(file string) bool {
	return slices.ContainsFunc(w.owned, func(entry OwnedEntry) bool { return entry.File == file })
}

type EditParams struct {
	File  string
	Lines []dotenv.Line
}

func (w Writes) Edit(params EditParams) []dotenv.Line {
	plan := w.plan(planOfParams{Links: w.linksIn(params.File), Lines: map[string][]dotenv.Line{params.File: params.Lines}})
	lines := applyPorts(params.Lines, plan.Entries)
	for _, entry := range w.owned {
		if entry.File == params.File {
			lines, _ = dotenv.Set(dotenv.SetParams{Lines: lines, Key: entry.Key, Value: entry.Value})
		}
	}
	return lines
}

func (w Writes) Settled(read map[string][]dotenv.Line) Plan {
	if w.Empty() {
		return Plan{}
	}
	plan := w.plan(planOfParams{Links: w.links, Lines: read})
	plan.Owned = w.ownedOver(read)
	plan.Applied = true
	return plan
}

func (w Writes) ownedOver(read map[string][]dotenv.Line) []OwnedEntry {
	if len(w.owned) == 0 {
		return nil
	}
	lines := make(map[string][]dotenv.Line, len(read))
	for file, content := range read {
		lines[file] = content
	}
	owned := make([]OwnedEntry, 0, len(w.owned))
	for _, entry := range w.owned {
		updated, changed := dotenv.Set(dotenv.SetParams{Lines: lines[entry.File], Key: entry.Key, Value: entry.Value})
		lines[entry.File] = updated
		entry.Changed = changed
		owned = append(owned, entry)
	}
	return owned
}

func (w Writes) linksIn(file string) []run.EnvPortLink {
	var links []run.EnvPortLink
	for _, link := range w.links {
		if link.File == file {
			links = append(links, link)
		}
	}
	return links
}

func jobsByName(cfg run.Config) map[string]run.Job {
	byName := make(map[string]run.Job, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		byName[job.Name] = job
	}
	return byName
}

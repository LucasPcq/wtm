// Package env is a worktree's environment: where its .env files come from, what wtm writes into them, the environment and variables its jobs and hooks run with, its hook chain.
package env

import (
	"errors"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// NeedsOrdinal reports whether a worktree needs an ordinal to settle its .env files: the isolation is not verbatim and run.toml links a port or a value into one.
func NeedsOrdinal(isolation project.Isolation, run project.Run) bool {
	if isolation.Effective() == project.Verbatim {
		return false
	}
	return len(run.EnvPorts) > 0 || len(run.EnvValues) > 0
}

// ResolveParams describes the worktree whose .env writes are resolved: its job parameters, the project's declared .env files, the proxy's configuration and the port the run proxy listens on, zero when it serves none.
type ResolveParams struct {
	JobParams
	Files      []project.EnvFile
	Proxy      project.Proxy
	PublicPort int
}

// Writes is what wtm writes into a worktree's .env files: the ports run.toml links, moved for the worktree, and the values wtm owns. Its zero value writes nothing.
type Writes struct {
	links    []project.EnvPortLink
	owned    []OwnedEntry
	bases    map[portRef]int
	block    int
	worktree settling
}

// Resolve returns the writes of a worktree, nothing when its isolation is verbatim. Links to a file the project does not declare are left out. It fails on an invalid proxy port, on a value that needs the proxy when it serves no port, and with a *ValueError on a value of run.toml that cannot be expanded.
func Resolve(params ResolveParams) (Writes, error) {
	if params.Isolation.Effective() == project.Verbatim {
		return Writes{}, nil
	}

	kept, _ := Targets(params.Run, params.Files)
	owned := ownedWrites(kept, params.Files, params.composeProject())
	if len(kept.EnvPorts) == 0 && len(kept.EnvValues) == 0 {
		return Writes{owned: owned}, nil
	}

	if err := params.Proxy.Problem(); err != nil {
		return Writes{}, err
	}
	settled := settling{
		Addressing: effectiveAddressing(kept),
		Jobs:       jobsByName(kept),
		Slug:       worktree.Slug(params.Branch),
		Project:    params.Project(),
		PublicPort: params.PublicPort,
		Ordinal:    params.Ordinal,
		Offset:     params.Offset(),
		Shared:     sharedJobs(kept),
	}

	values, err := settled.valueWrites(kept.EnvValues)
	// A proxy serving no port is the global configuration's doing, not a mistake in run.toml.
	if errors.Is(err, errNoProxy) {
		return Writes{}, err
	}
	if err != nil {
		return Writes{}, &ValueError{Err: err}
	}

	return Writes{links: kept.EnvPorts, owned: append(owned, values...), bases: portBases(kept), block: OffsetBlock(kept), worktree: settled}, nil
}

// Empty reports whether w writes nothing.
func (w Writes) Empty() bool {
	return len(w.links) == 0 && len(w.owned) == 0
}

// Files returns every .env file w writes into, each once, linked ports first.
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

// Creates reports whether w writes file even when it does not exist, because it sets a value wtm owns in it.
func (w Writes) Creates(file string) bool {
	return slices.ContainsFunc(w.owned, func(entry OwnedEntry) bool { return entry.File == file })
}

// Edit returns the lines of file with w's writes into it applied: its linked ports moved, then the values wtm owns set.
func (w Writes) Edit(file string, lines []dotenv.Line) []dotenv.Line {
	plan := w.plan(w.linksIn(file), map[string][]dotenv.Line{file: lines})
	lines = applyPorts(lines, plan.Entries)
	for _, entry := range w.owned {
		if entry.File == file {
			lines, _ = dotenv.Set(dotenv.SetParams{Lines: lines, Key: entry.Key, Value: entry.Value})
		}
	}
	return lines
}

// Settled returns the applied plan of w, decided over read, the lines of each file as they were before Edit; it is the zero Plan when w writes nothing.
func (w Writes) Settled(read map[string][]dotenv.Line) Plan {
	if w.Empty() {
		return Plan{}
	}

	plan := w.plan(w.links, read)
	plan.Owned = w.ownedOver(read)
	plan.Applied = true
	return plan
}

// ownedOver returns the values wtm owns, each marked with whether setting it over read changes its file.
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

// linksIn returns the port links into file.
func (w Writes) linksIn(file string) []project.EnvPortLink {
	var links []project.EnvPortLink
	for _, link := range w.links {
		if link.File == file {
			links = append(links, link)
		}
	}
	return links
}

// jobsByName returns the jobs of cfg indexed by name.
func jobsByName(cfg project.Run) map[string]project.Job {
	byName := make(map[string]project.Job, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		byName[job.Name] = job
	}
	return byName
}

// settling is the worktree whose .env files are settled, as the rewrites read it: how its jobs are addressed, its jobs, its branch slug, the project, the proxy's port, its ordinal and offset, and its shared jobs.
type settling struct {
	Addressing project.Addressing
	Jobs       map[string]project.Job
	Slug       string
	Project    string
	PublicPort int
	Ordinal    int
	Offset     int
	Shared     map[string]bool
}

// names reports whether jobs are addressed by name: the addressing asks for it and the proxy serves a port.
func (w settling) names() bool {
	return w.Addressing == project.AddressNames && w.PublicPort > 0
}

// jobLabel returns the host label of the job named job, empty when it publishes no url.
func (w settling) jobLabel(job string) string {
	return jobHostLabel(w.Jobs[job])
}

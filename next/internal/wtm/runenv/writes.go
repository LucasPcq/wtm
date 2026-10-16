package runenv

import (
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

type ResolveParams struct {
	Params
	Files      []project.EnvFile
	Proxy      project.Proxy
	PublicPort int
}

type Writes struct {
	links   []run.EnvPortLink
	owned   []OwnedEntry
	bases   map[portRef]int
	shared  map[string]bool
	offset  int
	block   int
	origins originContext
}

func Resolve(params ResolveParams) (Writes, error) {
	if _, verbatim := CaseOf(params.Isolation).(Verbatim); verbatim {
		return Writes{}, nil
	}
	kept, _ := Targets(TargetsParams{Run: params.Run, Files: params.Files})
	owned := ownedWrites(ownedWritesParams{Run: kept, Files: params.Files, Values: map[string]string{
		envComposeProject: composeProjectName(composeNameParams{Project: filepath.Base(params.ProjectDir), Worktree: branch.Slug(params.Branch)}),
	}})
	if len(kept.EnvPorts) == 0 && len(kept.EnvValues) == 0 {
		return Writes{owned: owned}, nil
	}
	if err := validateProxy(params.Proxy); err != nil {
		return Writes{}, err
	}
	env := JobEnv(params.Params)
	offset, err := strconv.Atoi(env[envPortOffset])
	if err != nil {
		return Writes{}, err
	}
	origins := originContext{
		Addressing: effectiveAddressing(kept),
		Jobs:       jobsByName(kept),
		Worktree:   env[envWorktree],
		Project:    filepath.Base(params.ProjectDir),
		PublicPort: params.PublicPort,
	}
	values, err := valueWrites(valueWritesParams{Run: kept, Worktree: env[envWorktree], Ordinal: params.Ordinal, Offset: offset, Origins: origins})
	if err != nil {
		return Writes{}, err
	}
	return Writes{
		links:   kept.EnvPorts,
		owned:   append(owned, values...),
		bases:   portBases(kept),
		shared:  sharedJobs(kept),
		offset:  offset,
		block:   offsetBlock(kept),
		origins: origins,
	}, nil
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
	for _, entry := range w.owned {
		if entry.File == file {
			return true
		}
	}
	return false
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

type planOfParams struct {
	Links []run.EnvPortLink
	Lines map[string][]dotenv.Line
}

func (w Writes) plan(params planOfParams) Plan {
	return planPorts(planPortsParams{Links: params.Links, Bases: w.bases, Origins: w.origins, Block: w.block, Offset: w.offset, Lines: params.Lines, Shared: w.shared})
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

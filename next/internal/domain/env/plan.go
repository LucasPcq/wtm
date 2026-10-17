package env

import (
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
)

type Status string

const (
	StatusRewrite      Status = "rewrite"
	StatusUnchanged    Status = "unchanged"
	StatusMissingKey   Status = "missing_key"
	StatusNotFound     Status = "base_not_found"
	StatusAmbiguous    Status = "ambiguous"
	StatusForeignHost  Status = "foreign_host"
	StatusSecureScheme Status = "secure_scheme"
)

type Move struct {
	Port     string `json:"port"`
	Job      string `json:"job"`
	Base     int    `json:"base"`
	Resolved int    `json:"resolved"`
	Origin   string `json:"origin,omitempty"`
}

type Entry struct {
	File         string             `json:"file"`
	Key          string             `json:"key"`
	Port         string             `json:"port"`
	Base         int                `json:"base"`
	Resolved     int                `json:"resolved"`
	Moves        []Move             `json:"moves,omitempty"`
	Addressing   project.Addressing `json:"addressing"`
	Status       Status             `json:"status"`
	CurrentValue string             `json:"current_value,omitempty"`
	NewValue     string             `json:"new_value,omitempty"`
	ForeignHost  string             `json:"foreign_host,omitempty"`
}

type Plan struct {
	Offset     int                `json:"offset"`
	Entries    []Entry            `json:"entries"`
	Addressing project.Addressing `json:"addressing"`
	PublicPort int                `json:"public_port,omitempty"`
	Owned      []OwnedEntry       `json:"owned,omitempty"`
	Applied    bool               `json:"applied"`
}

func (p Plan) Rewrites() []Entry {
	var out []Entry
	for _, entry := range p.Entries {
		if entry.Status == StatusRewrite {
			out = append(out, entry)
		}
	}
	return out
}

func (p Plan) Anomalies() []Entry {
	var out []Entry
	for _, entry := range p.Entries {
		switch entry.Status {
		case StatusMissingKey, StatusNotFound, StatusAmbiguous, StatusForeignHost, StatusSecureScheme:
			out = append(out, entry)
		case StatusRewrite, StatusUnchanged:
		}
	}
	return out
}

type portRef struct {
	Job  string
	Name string
}

func portBases(cfg project.Run) map[portRef]int {
	bases := map[portRef]int{}
	for _, job := range cfg.Jobs {
		for name, base := range job.Ports {
			bases[portRef{Job: job.Name, Name: name}] = base
		}
	}
	return bases
}

type planOfParams struct {
	Links []project.EnvPortLink
	Lines map[string][]dotenv.Line
}

func (w Writes) plan(params planOfParams) Plan {
	plan := Plan{Offset: w.worktree.Offset, Addressing: w.worktree.Addressing, PublicPort: w.worktree.PublicPort}
	for _, group := range w.groups(params.Links) {
		plan.Entries = append(plan.Entries, keyPlan{Writes: w, Group: group}.entry(params.Lines[group.File]))
	}
	return plan
}

type linkGroup struct {
	File  string
	Key   string
	Links []project.EnvPortLink
	Bases []int
}

func (w Writes) groups(links []project.EnvPortLink) []linkGroup {
	var order []string
	byKey := map[string]*linkGroup{}
	for _, link := range links {
		base, declared := w.bases[portRef{Job: link.Job, Name: link.Port}]
		if link.Job == "" || !declared {
			continue
		}
		id := link.File + "\x00" + link.Key
		group, seen := byKey[id]
		if !seen {
			group = &linkGroup{File: link.File, Key: link.Key}
			byKey[id] = group
			order = append(order, id)
		}
		group.Links = append(group.Links, link)
		group.Bases = append(group.Bases, base)
	}
	groups := make([]linkGroup, 0, len(order))
	for _, id := range order {
		groups = append(groups, *byKey[id])
	}
	return groups
}

type keyPlan struct {
	Writes
	Group linkGroup
}

func (k keyPlan) entry(lines []dotenv.Line) Entry {
	merged := k.linkEntry(linkAt{Index: 0, Lines: lines})
	merged.Moves = []Move{k.move(moveParams{Index: 0, Entry: merged})}
	for i := 1; i < len(k.Group.Links); i++ {
		if merged.Status == StatusRewrite {
			lines = applyPorts(lines, []Entry{merged})
		}
		next := k.linkEntry(linkAt{Index: i, Lines: lines})
		merged = fold(merged, next)
		merged.Moves = append(merged.Moves, k.move(moveParams{Index: i, Entry: next}))
	}
	return merged
}

func (k keyPlan) resolved(index int) int {
	return ResolvedPort(ResolvedPortParams{Base: k.Group.Bases[index], Offset: k.worktree.Offset, Shared: k.worktree.Shared[k.Group.Links[index].Job]})
}

type moveParams struct {
	Index int
	Entry Entry
}

func (k keyPlan) move(params moveParams) Move {
	link := k.Group.Links[params.Index]
	move := Move{Port: link.Port, Job: link.Job, Base: k.Group.Bases[params.Index], Resolved: k.resolved(params.Index)}
	if params.Entry.Addressing == project.AddressNames {
		move.Origin = k.worktree.linkOrigin(link)
	}
	return move
}

func fold(into, next Entry) Entry {
	if next.Status == StatusRewrite {
		into.NewValue = next.NewValue
		into.Status = StatusRewrite
		if next.Addressing == project.AddressNames {
			into.Addressing = next.Addressing
		}
		return into
	}
	if into.Status == StatusUnchanged || into.Status == StatusNotFound {
		into.Status = next.Status
	}
	return into
}

type linkAt struct {
	Index int
	Lines []dotenv.Line
}

func (k keyPlan) linkEntry(at linkAt) Entry {
	link := k.Group.Links[at.Index]
	entry := Entry{File: link.File, Key: link.Key, Port: link.Port, Base: k.Group.Bases[at.Index], Resolved: k.resolved(at.Index), Addressing: project.AddressPorts}
	line, found := pairByKey(at.Lines, link.Key)
	if !found {
		entry.Status = StatusMissingKey
		return entry
	}
	entry.CurrentValue = line.Value
	if origin, planned := k.worktree.originEntry(originEntryParams{Entry: entry, Link: link}); planned {
		return origin
	}
	value := reducePortValue(reducePortParams{Value: line.Value, Base: entry.Base, Block: k.block, JobLabel: k.worktree.jobLabel(link.Job), Project: k.worktree.Project})
	offsets := portOffsets(value, entry.Base)
	switch {
	case len(offsets) > 1:
		entry.Status = StatusAmbiguous
	case len(offsets) == 1:
		entry.NewValue = replaceAt(replaceParams{Value: value, At: offsets[0], Width: len(strconv.Itoa(entry.Base)), To: entry.Resolved})
		entry.Status = StatusRewrite
	default:
		entry.Status = alreadyResolved(value, entry)
	}
	if entry.Status == StatusRewrite && entry.NewValue == entry.CurrentValue {
		entry.Status, entry.NewValue = StatusUnchanged, ""
	}
	return entry
}

func alreadyResolved(value string, entry Entry) Status {
	if entry.Resolved == entry.Base {
		return StatusNotFound
	}
	switch len(portOffsets(value, entry.Resolved)) {
	case 0:
		return StatusNotFound
	case 1:
		return StatusUnchanged
	}
	return StatusAmbiguous
}

func pairByKey(lines []dotenv.Line, key string) (dotenv.Line, bool) {
	for _, line := range lines {
		if line.Kind == dotenv.Pair && line.Key == key {
			return line, true
		}
	}
	return dotenv.Line{}, false
}

type Redacted Plan

func Redact(plan Plan) Redacted {
	entries := slices.Clone(plan.Entries)
	for i := range entries {
		entries[i].CurrentValue = ""
		entries[i].NewValue = ""
		entries[i].ForeignHost = ""
	}
	plan.Entries = entries
	return Redacted(plan)
}

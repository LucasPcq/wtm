package env

import (
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
)

// Status is what settling decided for one key of a .env file.
type Status string

// The statuses of an entry: a rewrite or no change, else an anomaly the user is told about — the key is missing, its value holds the base port nowhere or more than once, its url points at a host no job here serves, or it uses https on the job's own port.
const (
	StatusRewrite      Status = "rewrite"
	StatusUnchanged    Status = "unchanged"
	StatusMissingKey   Status = "missing_key"
	StatusNotFound     Status = "base_not_found"
	StatusAmbiguous    Status = "ambiguous"
	StatusForeignHost  Status = "foreign_host"
	StatusSecureScheme Status = "secure_scheme"
)

// A Move is one port a key links to and where it moves for the worktree: from Base to Resolved, or to Origin when the key is addressed by name.
type Move struct {
	Port     string `json:"port"`
	Job      string `json:"job"`
	Base     int    `json:"base"`
	Resolved int    `json:"resolved"`
	Origin   string `json:"origin,omitempty"`
}

// An Entry is one key of a .env file linked to one or more ports, and what settling decided for it. When several ports share a key, Port, Base and Resolved are the first one's and Moves lists them all.
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

// A Plan is what settling a worktree's .env files decides: the port offset, an entry per linked key, the values wtm owns, and whether it was applied.
type Plan struct {
	Offset     int                `json:"offset"`
	Entries    []Entry            `json:"entries"`
	Addressing project.Addressing `json:"addressing"`
	PublicPort int                `json:"public_port,omitempty"`
	Owned      []OwnedEntry       `json:"owned,omitempty"`
	Applied    bool               `json:"applied"`
}

// Rewrites returns the entries whose value is rewritten.
func (p Plan) Rewrites() []Entry {
	var out []Entry
	for _, entry := range p.Entries {
		if entry.Status == StatusRewrite {
			out = append(out, entry)
		}
	}
	return out
}

// Anomalies returns the entries the user is told about: neither rewritten nor unchanged.
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

// A portRef names a port by its job and its name.
type portRef struct {
	Job  string
	Name string
}

// portBases returns the base of every port each job of cfg declares.
func portBases(cfg project.Run) map[portRef]int {
	bases := map[portRef]int{}
	for _, job := range cfg.Jobs {
		for name, base := range job.Ports {
			bases[portRef{Job: job.Name, Name: name}] = base
		}
	}
	return bases
}

// plan decides an entry for each key the links name, over the lines read of each file.
func (w Writes) plan(links []project.EnvPortLink, lines map[string][]dotenv.Line) Plan {
	plan := Plan{Offset: w.worktree.Offset, Addressing: w.worktree.Addressing, PublicPort: w.worktree.PublicPort}
	for _, group := range w.groups(links) {
		plan.Entries = append(plan.Entries, keyPlan{Writes: w, Group: group}.entry(lines[group.File]))
	}
	return plan
}

// A linkGroup is the links that name one key of one file, with the base of each one's port.
type linkGroup struct {
	File  string
	Key   string
	Links []project.EnvPortLink
	Bases []int
}

// groups gathers links by file and key, in the order each key first appears; a link to no job or to a port its job does not declare is left out.
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

// A keyPlan plans one key of a file from the links of its group.
type keyPlan struct {
	Writes
	Group linkGroup
}

// entry plans each link of the key in turn, each over the value the previous ones rewrote, and folds them into one entry.
func (k keyPlan) entry(lines []dotenv.Line) Entry {
	merged := k.linkEntry(0, lines)
	merged.Moves = []Move{k.move(0, merged)}
	for i := 1; i < len(k.Group.Links); i++ {
		if merged.Status == StatusRewrite {
			lines = applyPorts(lines, []Entry{merged})
		}
		next := k.linkEntry(i, lines)
		merged = fold(merged, next)
		merged.Moves = append(merged.Moves, k.move(i, next))
	}
	return merged
}

// resolved returns the port the link at index resolves to in the worktree.
func (k keyPlan) resolved(index int) int {
	return resolvedPort(resolvedPortParams{Base: k.Group.Bases[index], Offset: k.worktree.Offset, Shared: k.worktree.Shared[k.Group.Links[index].Job]})
}

// move returns where the link at index moves, entry being what was planned for it.
func (k keyPlan) move(index int, entry Entry) Move {
	link := k.Group.Links[index]
	move := Move{Port: link.Port, Job: link.Job, Base: k.Group.Bases[index], Resolved: k.resolved(index)}
	if entry.Addressing == project.AddressNames {
		move.Origin = k.worktree.linkOrigin(link)
	}
	return move
}

// fold returns into updated with the entry of the next link of the same key: a rewrite carries its value forward, and an anomaly replaces a status that reported nothing to do.
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

// linkEntry plans the link at index over lines: by its named origin when it can, else by finding its base port in the value, once reduced from any earlier settling, and moving it to the resolved port.
func (k keyPlan) linkEntry(index int, lines []dotenv.Line) Entry {
	link := k.Group.Links[index]
	entry := Entry{File: link.File, Key: link.Key, Port: link.Port, Base: k.Group.Bases[index], Resolved: k.resolved(index), Addressing: project.AddressPorts}
	line, found := pairByKey(lines, link.Key)
	if !found {
		entry.Status = StatusMissingKey
		return entry
	}

	entry.CurrentValue = line.Value
	if origin, planned := k.worktree.originEntry(entry, link); planned {
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

// alreadyResolved returns the status of a value that holds the base port nowhere: unchanged when it holds the resolved port once, ambiguous when more than once, not found otherwise.
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

// pairByKey returns the key=value line of key, and reports whether lines hold one.
func pairByKey(lines []dotenv.Line, key string) (dotenv.Line, bool) {
	for _, line := range lines {
		if line.Kind == dotenv.Pair && line.Key == key {
			return line, true
		}
	}
	return dotenv.Line{}, false
}

// Redacted is a Plan with every value taken out of its entries, safe to print or send where a secret must not appear.
type Redacted Plan

// LeftAlone are the entries of a redacted plan it could not shift, as safe to print or send as the plan.
type LeftAlone []Entry

// LeftAlone returns the entries r could not shift, still redacted.
func (r Redacted) LeftAlone() LeftAlone {
	return LeftAlone(Plan(r).Anomalies())
}

// Redact returns plan without the current, new and foreign values of its entries; plan itself is left untouched.
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

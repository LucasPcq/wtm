package runenv

import (
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
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
	File         string         `json:"file"`
	Key          string         `json:"key"`
	Port         string         `json:"port"`
	Base         int            `json:"base"`
	Resolved     int            `json:"resolved"`
	Moves        []Move         `json:"moves,omitempty"`
	Addressing   run.Addressing `json:"addressing"`
	Status       Status         `json:"status"`
	CurrentValue string         `json:"current_value,omitempty"`
	NewValue     string         `json:"new_value,omitempty"`
	ForeignHost  string         `json:"foreign_host,omitempty"`
}

type Plan struct {
	Offset     int            `json:"offset"`
	Entries    []Entry        `json:"entries"`
	Addressing run.Addressing `json:"addressing"`
	PublicPort int            `json:"public_port,omitempty"`
	Owned      []OwnedEntry   `json:"owned,omitempty"`
	Applied    bool           `json:"applied"`
}

type planPortsParams struct {
	Links   []run.EnvPortLink
	Bases   map[portRef]int
	Origins originContext
	Block   int
	Offset  int
	Lines   map[string][]dotenv.Line
	Shared  map[string]bool
}

func planPorts(params planPortsParams) Plan {
	plan := Plan{Offset: params.Offset, Addressing: params.Origins.Addressing, PublicPort: params.Origins.PublicPort}
	for _, group := range groupLinks(params.Links, params.Bases) {
		plan.Entries = append(plan.Entries, planKey(keyParams{
			Group:   group,
			Offset:  params.Offset,
			Block:   params.Block,
			Lines:   params.Lines[group.File],
			Origins: params.Origins,
			Shared:  params.Shared,
		}))
	}
	return plan
}

type linkGroup struct {
	File  string
	Key   string
	Links []run.EnvPortLink
	Bases []int
}

func groupLinks(links []run.EnvPortLink, bases map[portRef]int) []linkGroup {
	var order []string
	byKey := map[string]*linkGroup{}
	for _, link := range links {
		base, declared := portBaseFor(bases, link)
		if !declared {
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

type keyParams struct {
	Group   linkGroup
	Offset  int
	Block   int
	Lines   []dotenv.Line
	Origins originContext
	Shared  map[string]bool
}

func planKey(params keyParams) Entry {
	merged := planEntry(params.entryParams(params.Lines, 0))
	merged.Moves = []Move{params.move(moveParams{Index: 0, Entry: merged})}
	lines := params.Lines
	for i := 1; i < len(params.Group.Links); i++ {
		if merged.Status == StatusRewrite {
			lines = applyPorts(lines, []Entry{merged})
		}
		next := planEntry(params.entryParams(lines, i))
		merged = fold(merged, next)
		merged.Moves = append(merged.Moves, params.move(moveParams{Index: i, Entry: next}))
	}
	return merged
}

func (p keyParams) resolved(index int) int {
	return resolvedPort(resolvedPortParams{Base: p.Group.Bases[index], Offset: p.Offset, Shared: p.Shared[p.Group.Links[index].Job]})
}

func (p keyParams) entryParams(lines []dotenv.Line, index int) entryParams {
	return entryParams{Link: p.Group.Links[index], Base: p.Group.Bases[index], Resolved: p.resolved(index), Block: p.Block, Lines: lines, Origins: p.Origins}
}

type moveParams struct {
	Index int
	Entry Entry
}

func (p keyParams) move(params moveParams) Move {
	link := p.Group.Links[params.Index]
	move := Move{Port: link.Port, Job: link.Job, Base: p.Group.Bases[params.Index], Resolved: p.resolved(params.Index)}
	if params.Entry.Addressing == run.AddressNames {
		move.Origin = p.Origins.linkOrigin(link)
	}
	return move
}

func (c originContext) linkOrigin(link run.EnvPortLink) string {
	return linkOrigin(linkOriginParams{Job: c.Jobs[link.Job], PortName: link.Port, Worktree: c.Worktree, Project: c.Project, PublicPort: c.PublicPort})
}

func fold(into, next Entry) Entry {
	if next.Status == StatusRewrite {
		into.NewValue = next.NewValue
		into.Status = StatusRewrite
		if next.Addressing == run.AddressNames {
			into.Addressing = next.Addressing
		}
		return into
	}
	if into.Status != StatusRewrite && (into.Status == StatusUnchanged || into.Status == StatusNotFound) {
		into.Status = next.Status
	}
	return into
}

type entryParams struct {
	Link     run.EnvPortLink
	Base     int
	Resolved int
	Block    int
	Lines    []dotenv.Line
	Origins  originContext
}

func planEntry(params entryParams) Entry {
	entry := Entry{File: params.Link.File, Key: params.Link.Key, Port: params.Link.Port, Base: params.Base, Resolved: params.Resolved, Addressing: run.AddressPorts}
	line, found := pairByKey(params.Lines, params.Link.Key)
	if !found {
		entry.Status = StatusMissingKey
		return entry
	}
	entry.CurrentValue = line.Value
	if origin, planned := planOrigin(entry, params); planned {
		return origin
	}
	value := reducePortValue(reducePortParams{Value: line.Value, Base: entry.Base, Block: params.Block, JobLabel: params.Origins.jobLabel(params.Link.Job), Project: params.Origins.Project})
	at := portOffsets(value, entry.Base)
	switch {
	case len(at) > 1:
		entry.Status = StatusAmbiguous
	case len(at) == 1:
		entry.NewValue = replaceAt(replaceParams{Value: value, At: at[0], Width: len(strconv.Itoa(entry.Base)), To: entry.Resolved})
		entry.Status = StatusRewrite
	default:
		entry.Status = alreadyResolved(value, entry)
	}
	if entry.Status == StatusRewrite && entry.NewValue == entry.CurrentValue {
		entry.Status, entry.NewValue = StatusUnchanged, ""
	}
	return entry
}

func planOrigin(entry Entry, params entryParams) (Entry, bool) {
	if !params.Origins.names() {
		return entry, false
	}
	origin := params.Origins.linkOrigin(params.Link)
	if origin == "" {
		return entry, false
	}
	rewrite := rewriteOrigin(rewriteOriginParams{Value: entry.CurrentValue, Origin: origin, JobLabel: params.Origins.jobLabel(params.Link.Job), Project: params.Origins.Project, Base: entry.Base, Resolved: entry.Resolved})
	if rewrite.Fallback {
		return entry, false
	}
	entry.Addressing = run.AddressNames
	entry.Status = rewrite.Status
	entry.NewValue = rewrite.Value
	entry.ForeignHost = rewrite.ForeignHost
	return entry, true
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

func applyPorts(lines []dotenv.Line, entries []Entry) []dotenv.Line {
	byKey := map[string]string{}
	for _, entry := range entries {
		if entry.Status == StatusRewrite {
			byKey[entry.Key] = entry.NewValue
		}
	}
	if len(byKey) == 0 {
		return lines
	}
	out := make([]dotenv.Line, len(lines))
	for i, line := range lines {
		value, rewritten := byKey[line.Key]
		out[i] = line
		if line.Kind == dotenv.Pair && rewritten {
			out[i] = line.WithValue(value)
		}
	}
	return out
}

type reducePortParams struct {
	Value    string
	Base     int
	Block    int
	JobLabel string
	Project  string
}

func reducePortValue(params reducePortParams) string {
	value := reduceOriginValue(reduceOriginParams{Value: params.Value, JobLabel: params.JobLabel, Project: params.Project, Base: params.Base})
	if value != params.Value {
		return value
	}
	block := params.Block
	if block <= 0 {
		block = defaultOffsetBlock
	}
	var at []number
	for _, n := range numbers(params.Value) {
		gap := n.value - params.Base
		if gap >= 0 && gap%block == 0 && gap/block <= collisionHorizon {
			at = append(at, n)
		}
	}
	if len(at) != 1 {
		return params.Value
	}
	return replaceAt(replaceParams{Value: params.Value, At: at[0].at, Width: at[0].width, To: params.Base})
}

type number struct {
	at    int
	width int
	value int
}

func numbers(value string) []number {
	var found []number
	for i := 0; i < len(value); {
		if !isDigitAt(value, i) {
			i++
			continue
		}
		end := i
		for isDigitAt(value, end) {
			end++
		}
		if n, err := strconv.Atoi(value[i:end]); err == nil {
			found = append(found, number{at: i, width: end - i, value: n})
		}
		i = end
	}
	return found
}

func portOffsets(value string, port int) []int {
	needle := strconv.Itoa(port)
	var offsets []int
	for from := 0; ; {
		i := strings.Index(value[from:], needle)
		if i < 0 {
			return offsets
		}
		at := from + i
		if !isDigitAt(value, at-1) && !isDigitAt(value, at+len(needle)) {
			offsets = append(offsets, at)
		}
		from = at + len(needle)
	}
}

func isDigitAt(value string, i int) bool {
	return i >= 0 && i < len(value) && value[i] >= '0' && value[i] <= '9'
}

type replaceParams struct {
	Value string
	At    int
	Width int
	To    int
}

func replaceAt(params replaceParams) string {
	return params.Value[:params.At] + strconv.Itoa(params.To) + params.Value[params.At+params.Width:]
}

func rewrites(plan Plan) []Entry {
	var out []Entry
	for _, entry := range plan.Entries {
		if entry.Status == StatusRewrite {
			out = append(out, entry)
		}
	}
	return out
}

func anomalies(plan Plan) []Entry {
	var out []Entry
	for _, entry := range plan.Entries {
		switch entry.Status {
		case StatusMissingKey, StatusNotFound, StatusAmbiguous, StatusForeignHost, StatusSecureScheme:
			out = append(out, entry)
		case StatusRewrite, StatusUnchanged:
		}
	}
	return out
}

package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ShortPortName is a declared port as a reader names it: the variable without
// its `_PORT` suffix, lower-cased and dashed. `PORT` alone stays `port`.
func ShortPortName(name string) string {
	trimmed := strings.TrimSuffix(name, domain.ReachPortSuffix)
	if trimmed == "" {
		trimmed = name
	}
	return strings.ReplaceAll(strings.ToLower(trimmed), "_", "-")
}

// NamedPorts orders a job's bound ports by name, the order every surface lists
// them in.
func NamedPorts(ports map[string]int) []domain.NamedPort {
	named := make([]domain.NamedPort, 0, len(ports))
	for _, name := range sortedPortNames(ports) {
		named = append(named, domain.NamedPort{Name: name, Port: ports[name]})
	}
	return named
}

// ReachSummary is the one fragment a job line or a pane title carries: the URL,
// the lone port, or how many there are — the detail is the block's.
func ReachSummary(entry domain.ReachEntry) string {
	switch {
	case len(entry.URLs) == 1:
		return entry.URLs[0].URL
	case len(entry.URLs) > 1:
		return fmt.Sprintf(domain.ReachURLsFmt, len(entry.URLs))
	case len(entry.Ports) == 1:
		return fmt.Sprintf(domain.ReachPortFmt, entry.Ports[0].Port)
	case len(entry.Ports) > 1:
		return fmt.Sprintf(domain.ReachPortsFmt, len(entry.Ports))
	}
	return ""
}

type ReachLinesParams struct {
	Entries []domain.ReachEntry
}

type reachRow struct {
	label  string
	values []string
}

// ReachLines is the body of the block that says where every job of a run is
// reached. URLs come first, one row each and labelled by the job that answers
// them — a runner's apps by their own names. Then the jobs reached by port, one
// port per line. A job with nothing to reach — a task, a launcher that binds
// nothing — has no row.
func ReachLines(params ReachLinesParams) []string {
	rows := reachRows(params.Entries)
	return renderReachRows(renderReachParams{Rows: rows, LabelWidth: labelWidthOf(rows)})
}

// ReachWorktree is one worktree's part of the block: the jobs up there, the
// shared services it holds included.
type ReachWorktree struct {
	Name    string
	Entries []domain.ReachEntry
}

type ReachBlockParams struct {
	Worktrees []ReachWorktree
}

// ReachBlock is the whole block, one section per thing a reader looks up. The
// shared services come first and once: a postgres three worktrees hold is one
// process, and listing it under each of them read as three. Each holder's
// namespace hangs under it when there are several worktrees to tell apart.
// Then every worktree's own jobs, titled by the worktree when there are
// several. The label column is common to every section, so they read as one
// list.
func ReachBlock(params ReachBlockParams) []domain.ReachSection {
	multi := len(params.Worktrees) > 1
	shared := sharedServicesOf(params.Worktrees)

	var all []reachRow
	for _, service := range shared {
		all = append(all, service.rows(multi)...)
	}
	for _, worktree := range params.Worktrees {
		all = append(all, reachRows(ownEntries(worktree.Entries))...)
	}
	width := labelWidthOf(all)

	var sections []domain.ReachSection
	for _, host := range sharedHosts(shared) {
		var rows []reachRow
		for _, service := range shared {
			if service.host == host {
				rows = append(rows, service.rows(multi)...)
			}
		}
		sections = append(sections, domain.ReachSection{
			Title:  sharedTitle(sharedTitleParams{Host: host, Worktrees: params.Worktrees}),
			Shared: true,
			Lines:  renderReachRows(renderReachParams{Rows: rows, LabelWidth: width}),
		})
	}
	for _, worktree := range params.Worktrees {
		lines := renderReachRows(renderReachParams{Rows: reachRows(ownEntries(worktree.Entries)), LabelWidth: width})
		if len(lines) == 0 {
			continue
		}
		title := domain.ReachTitle
		if multi {
			title = worktree.Name
		}
		sections = append(sections, domain.ReachSection{Title: title, Worktree: worktree.Name, Lines: lines})
	}
	return sections
}

func ownEntries(entries []domain.ReachEntry) []domain.ReachEntry {
	var own []domain.ReachEntry
	for _, entry := range entries {
		if entry.SharedIn == "" {
			own = append(own, entry)
		}
	}
	return own
}

type sharedHolder struct {
	worktree  string
	namespace string
}

type sharedService struct {
	host    string
	entry   domain.ReachEntry
	holders []sharedHolder
}

// rows lays a shared service out. Held by one worktree, its namespace rides the
// port line as it always has; above several, each holder gets a line of its
// own, since whose slice is whose is the whole question.
func (s sharedService) rows(multi bool) []reachRow {
	entry := s.entry
	if !multi {
		if len(s.holders) > 0 {
			entry.Namespace = s.holders[0].namespace
		}
		return reachRows([]domain.ReachEntry{entry})
	}
	entry.Namespace = ""
	rows := reachRows([]domain.ReachEntry{entry})
	if len(rows) == 0 {
		rows = []reachRow{{label: entry.Job}}
	}
	width := 0
	for _, holder := range s.holders {
		width = max(width, len([]rune(holder.worktree)))
	}
	for _, holder := range s.holders {
		if holder.namespace == "" {
			continue
		}
		rows[len(rows)-1].values = append(rows[len(rows)-1].values, pad(holder.worktree, width)+domain.ReachLabelGap+holder.namespace)
	}
	return rows
}

// sharedServicesOf gathers every worktree's hold on the same service into one:
// a service is its name and the worktree it runs in.
func sharedServicesOf(worktrees []ReachWorktree) []sharedService {
	var services []sharedService
	index := map[[2]string]int{}
	for _, worktree := range worktrees {
		for _, entry := range worktree.Entries {
			if entry.SharedIn == "" {
				continue
			}
			key := [2]string{entry.Job, entry.SharedIn}
			at, seen := index[key]
			if !seen {
				at = len(services)
				index[key] = at
				services = append(services, sharedService{host: entry.SharedIn, entry: entry})
			}
			if len(services[at].entry.Ports) == 0 && len(services[at].entry.URLs) == 0 {
				services[at].entry.Ports, services[at].entry.URLs = entry.Ports, entry.URLs
			}
			services[at].holders = append(services[at].holders, sharedHolder{worktree: worktree.Name, namespace: entry.Namespace})
		}
	}
	return services
}

func sharedHosts(services []sharedService) []string {
	var hosts []string
	seen := map[string]bool{}
	for _, service := range services {
		if !seen[service.host] {
			seen[service.host] = true
			hosts = append(hosts, service.host)
		}
	}
	return hosts
}

type sharedTitleParams struct {
	Host      string
	Worktrees []ReachWorktree
}

// sharedTitle says where a shared service runs — except to the worktree it runs
// in, where "running in main" read oddly from main.
func sharedTitle(params sharedTitleParams) string {
	if len(params.Worktrees) == 1 && params.Worktrees[0].Name == params.Host {
		return domain.ReachSharedHereTitle
	}
	return fmt.Sprintf(domain.ReachSharedTitleFmt, params.Host)
}

func reachRows(entries []domain.ReachEntry) []reachRow {
	var urlRows, portRows []reachRow
	for _, entry := range entries {
		if len(entry.URLs) > 0 {
			for i, url := range entry.URLs {
				value := url.URL
				if i == 0 && entry.Namespace != "" {
					value += domain.ReachDetailSep + entry.Namespace
				}
				urlRows = append(urlRows, reachRow{label: url.Job, values: []string{value}})
			}
			continue
		}
		if len(entry.Ports) == 0 && entry.Namespace == "" {
			continue
		}
		portRows = append(portRows, reachRow{label: entry.Job, values: portCells(entry)})
	}
	return append(urlRows, portRows...)
}

func labelWidthOf(rows []reachRow) int {
	width := 0
	for _, row := range rows {
		width = max(width, len([]rune(row.label)))
	}
	return width
}

type renderReachParams struct {
	Rows       []reachRow
	LabelWidth int
}

// renderReachRows writes a row's values one per line, the label on the first:
// ports laid out in columns across the width scanned as a grid to decode rather
// than a list to read down.
func renderReachRows(params renderReachParams) []string {
	if len(params.Rows) == 0 {
		return nil
	}
	var lines []string
	for _, row := range params.Rows {
		for i, value := range row.values {
			label := ""
			if i == 0 {
				label = row.label
			}
			lines = append(lines, strings.TrimRight(pad(label, params.LabelWidth)+domain.ReachLabelGap+value, " "))
		}
		if len(row.values) == 0 {
			lines = append(lines, row.label)
		}
	}
	return lines
}

// portCells is a port-only job's ports, one per line: a lone one bare, several
// by name with the ports aligned, so the reader knows which is which.
func portCells(entry domain.ReachEntry) []string {
	if len(entry.Ports) == 0 {
		return []string{entry.Namespace}
	}
	if len(entry.Ports) == 1 {
		cell := fmt.Sprintf(domain.ReachPortFmt, entry.Ports[0].Port)
		if entry.Namespace != "" {
			cell += domain.ReachDetailSep + entry.Namespace
		}
		return []string{cell}
	}
	nameWidth := 0
	for _, port := range entry.Ports {
		if port.Name != "" {
			nameWidth = max(nameWidth, len([]rune(ShortPortName(port.Name))))
		}
	}
	cells := make([]string, 0, len(entry.Ports)+1)
	for _, port := range entry.Ports {
		name := ""
		if port.Name != "" {
			name = ShortPortName(port.Name)
		}
		cells = append(cells, pad(name, nameWidth)+domain.ReachLabelGap+fmt.Sprintf(domain.ReachPortFmt, port.Port))
	}
	if entry.Namespace != "" {
		cells = append(cells, entry.Namespace)
	}
	return cells
}

type ReachEntryParams struct {
	Job  string
	URL  string
	Held []domain.JobURLEntry
	// Ports are what the job bound, read only when it answers on no URL.
	Ports     map[string]int
	Namespace string
	SharedIn  string
}

// ReachEntryFor is one started job as the block lists it. A runner answers for
// the apps it holds rather than for itself: their URLs are the ones a reader
// came for, and the ports on the runner are theirs.
func ReachEntryFor(params ReachEntryParams) domain.ReachEntry {
	entry := domain.ReachEntry{Job: params.Job, Namespace: params.Namespace, SharedIn: params.SharedIn}
	switch {
	case len(params.Held) > 0:
		entry.URLs = params.Held
	case params.URL != "":
		entry.URLs = []domain.JobURLEntry{{Job: params.Job, URL: params.URL}}
	default:
		entry.Ports = NamedPorts(params.Ports)
	}
	return entry
}

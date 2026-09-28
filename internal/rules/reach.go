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
	// Width is what the block may take, label column included. Zero takes
	// domain.ReachDefaultWidth.
	Width int
}

type reachRow struct {
	label  string
	values []string
}

// ReachLines is the body of the block that says where every job of a run is
// reached. URLs come first, one row each and labelled by the job that answers
// them — a runner's apps by their own names. Then the jobs reached by port,
// their ports in columns wrapped to the width. A job with nothing to reach —
// a task, a launcher that binds nothing — has no row.
func ReachLines(params ReachLinesParams) []string {
	rows := reachRows(params.Entries)
	return renderReachRows(renderReachParams{Rows: rows, LabelWidth: labelWidthOf(rows), Width: params.Width})
}

// ReachSections splits the block in two: what runs in this worktree, then the
// shared services it holds in another — the one fact a reader has to be told,
// since stopping this worktree leaves those running. The label column is
// shared, so the two read as one list.
func ReachSections(params ReachLinesParams) []domain.ReachSection {
	var own []domain.ReachEntry
	var sharedOrder []string
	shared := map[string][]domain.ReachEntry{}
	for _, entry := range params.Entries {
		if entry.SharedIn == "" {
			own = append(own, entry)
			continue
		}
		if _, seen := shared[entry.SharedIn]; !seen {
			sharedOrder = append(sharedOrder, entry.SharedIn)
		}
		shared[entry.SharedIn] = append(shared[entry.SharedIn], entry)
	}

	width := labelWidthOf(reachRows(params.Entries))
	var sections []domain.ReachSection
	add := func(title string, entries []domain.ReachEntry) {
		lines := renderReachRows(renderReachParams{Rows: reachRows(entries), LabelWidth: width, Width: params.Width})
		if len(lines) > 0 {
			sections = append(sections, domain.ReachSection{Title: title, Lines: lines})
		}
	}
	add(domain.ReachTitle, own)
	for _, worktree := range sharedOrder {
		add(fmt.Sprintf(domain.ReachSharedTitleFmt, worktree), shared[worktree])
	}
	return sections
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
	Width      int
}

func renderReachRows(params renderReachParams) []string {
	if len(params.Rows) == 0 {
		return nil
	}
	width := params.Width
	if width <= 0 {
		width = domain.ReachDefaultWidth
	}
	room := max(width-params.LabelWidth-len(domain.ReachLabelGap), 1)

	var lines []string
	for _, row := range params.Rows {
		for i, line := range wrapCells(row.values, room) {
			label := ""
			if i == 0 {
				label = row.label
			}
			lines = append(lines, strings.TrimRight(pad(label, params.LabelWidth)+domain.ReachLabelGap+line, " "))
		}
	}
	return lines
}

// portCells is a port-only job's ports as cells: a lone one bare, several by
// name so the reader knows which is which.
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
	cells := make([]string, 0, len(entry.Ports)+1)
	for _, port := range entry.Ports {
		if port.Name == "" {
			cells = append(cells, fmt.Sprintf(domain.ReachPortFmt, port.Port))
			continue
		}
		cells = append(cells, fmt.Sprintf(domain.ReachNamedPortFmt, ShortPortName(port.Name), port.Port))
	}
	if entry.Namespace != "" {
		cells = append(cells, entry.Namespace)
	}
	return cells
}

// wrapCells lays cells out in aligned columns, as many per line as the room
// takes. A single cell is never padded, so a URL row ends where its URL does.
func wrapCells(cells []string, room int) []string {
	if len(cells) == 1 {
		return cells
	}
	cellWidth := 0
	for _, cell := range cells {
		cellWidth = max(cellWidth, len([]rune(cell)))
	}
	fits := max((room+len(domain.ReachCellGap))/(cellWidth+len(domain.ReachCellGap)), 1)
	// Balanced over the lines it takes anyway: six ports five-and-one read as a
	// row and a stray, three-and-three as a grid.
	rows := (len(cells) + fits - 1) / fits
	perLine := (len(cells) + rows - 1) / rows

	var lines []string
	for start := 0; start < len(cells); start += perLine {
		end := min(start+perLine, len(cells))
		padded := make([]string, 0, end-start)
		for _, cell := range cells[start:end] {
			padded = append(padded, pad(cell, cellWidth))
		}
		lines = append(lines, strings.Join(padded, domain.ReachCellGap))
	}
	return lines
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

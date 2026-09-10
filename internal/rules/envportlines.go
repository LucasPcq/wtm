package rules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type EnvPortTableParams struct {
	Plan domain.EnvPortPlan
	// Width is the room the surface has to draw in, zero when it cannot measure
	// itself. It is an input rather than a constant because the value column is
	// what is left of it: a named origin runs past fifty characters, and a table
	// that cut them all to the same fixed budget was the reason a single key
	// took two lines on a wide terminal.
	Width int
}

// EnvPortTableLines renders the rewrites of a plan as one aligned table, each
// file introduced by a rule bearing its name. Two things carry the structure and
// both are load-bearing.
//
// The header, because without it the first three columns are unknowns until the
// reader reaches the fourth and has to decode the line backwards. Read across, a
// row now says "this key follows that port, which moves from here to there, and
// becomes this".
//
// The named rule, because the alternative — indenting rows under a bare filename —
// pushes them out of alignment with the header naming their columns. A rule
// separates the groups without spending indentation to do it.
//
// Only the port shows a before and an after: it is the only thing that changes,
// and printing both whole values would double the width for nothing. The value is
// elided, which is not only about width — a DATABASE_URL printed whole puts a
// password on screen.
func EnvPortTableLines(params EnvPortTableParams) []string {
	entries := EnvPortRewrites(params.Plan)
	if len(entries) == 0 {
		return nil
	}

	keyWidth, portWidth, moveWidth := envPortColumnWidths(entries)
	row := func(key, port, move, value string) string {
		return strings.TrimRight(fmt.Sprintf(domain.EnvPortTableRowFmt,
			pad(key, keyWidth), pad(port, portWidth), pad(move, moveWidth), value), " ")
	}

	valueWidth := envPortValueWidth(envPortValueWidthParams{
		Width: params.Width, Key: keyWidth, Port: portWidth, Move: moveWidth,
	})
	rows := []string{row(
		domain.EnvPortHeaderKey,
		domain.EnvPortHeaderFollows,
		domain.EnvPortHeaderPort,
		domain.EnvPortHeaderBecomes,
	)}
	for _, e := range entries {
		rows = append(rows, row(e.Key, e.Port, envPortMove(e),
			ElideEnvValue(ElideEnvValueParams{Value: e.NewValue, Width: valueWidth})))
	}

	return withFileRules(withFileRulesParams{Entries: entries, Rows: rows, Width: widestLine(rows)})
}

type withFileRulesParams struct {
	Entries []domain.EnvPortEntry
	// Rows is the header followed by one line per entry, in Entries order.
	Rows  []string
	Width int
}

// withFileRules reassembles the rendered rows into their file groups, each
// opened by a rule naming the file.
func withFileRules(params withFileRulesParams) []string {
	byKey := map[domain.EnvPortLink]string{}
	for i, e := range params.Entries {
		byKey[domain.EnvPortLink{File: e.File, Key: e.Key}] = params.Rows[i+1]
	}

	lines := []string{params.Rows[0]}
	for i, file := range envPortFiles(params.Entries) {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fileRule(file, params.Width))
		for _, e := range params.Entries {
			if e.File == file {
				lines = append(lines, byKey[domain.EnvPortLink{File: e.File, Key: e.Key}])
			}
		}
	}
	return lines
}

func fileRule(file string, width int) string {
	head := fmt.Sprintf(domain.EnvPortFileRuleFmt, file)
	if fill := width - len([]rune(head)); fill > 0 {
		return head + strings.Repeat(domain.EnvPortRuleRune, fill)
	}
	return head
}

func widestLine(lines []string) int {
	widest := 0
	for _, l := range lines {
		widest = max(widest, len([]rune(l)))
	}
	return widest
}

// EnvPortAnomalyLines names the links wtm refused to act on, in the same shape
// as the table above: the key, the port it was meant to follow, and why nothing
// was done. They are listed one by one rather than folded into a count — a link
// that never matches is a mistake in run.toml, and the user can only fix the one
// they can see.
func EnvPortAnomalyLines(plan domain.EnvPortPlan) []string {
	entries := EnvPortAnomalies(plan)
	if len(entries) == 0 {
		return nil
	}

	keyWidth, portWidth := 0, 0
	for _, e := range entries {
		keyWidth = max(keyWidth, len(e.Key))
		portWidth = max(portWidth, len(e.Port))
	}

	rows := []string{""}
	for _, e := range entries {
		rows = append(rows, strings.TrimRight(fmt.Sprintf(domain.EnvPortAnomalyRowFmt,
			pad(e.Key, keyWidth), pad(e.Port, portWidth), envPortAnomalyReason(e)), " "))
	}

	// Row 0 is a placeholder for the header withFileRules expects; the reasons are
	// prose and need no column names.
	return withFileRules(withFileRulesParams{Entries: entries, Rows: rows, Width: widestLine(rows[1:])})[1:]
}

func envPortAnomalyReason(e domain.EnvPortEntry) string {
	switch e.Status {
	case domain.EnvPortStatusMissingKey:
		return domain.EnvPortReasonMissingKey
	case domain.EnvPortStatusAmbiguous:
		return fmt.Sprintf(domain.EnvPortReasonAmbiguousFmt, e.Base)
	case domain.EnvPortStatusForeignHost:
		return fmt.Sprintf(domain.EnvPortReasonForeignHostFmt, e.ForeignHost)
	case domain.EnvPortStatusSecureScheme:
		return domain.EnvPortReasonSecureScheme
	default:
		return fmt.Sprintf(domain.EnvPortReasonNotFoundFmt, e.Base)
	}
}

// envPortMove is the third column: the port move for a value that keeps a port,
// and a marker for one that takes an address, whose change the value column
// already carries whole.
func envPortMove(e domain.EnvPortEntry) string {
	if e.Addressing == domain.AddressingNames {
		return domain.EnvPortMoveOrigin
	}
	if len(e.Moves) <= 1 {
		return fmt.Sprintf(domain.EnvPortMoveFmt, strconv.Itoa(e.Base), strconv.Itoa(e.Resolved))
	}
	// One row per key, the ports grouped: the key is what the reader recognises
	// in their file, and a value rewritten once is one fact.
	bases := make([]string, 0, len(e.Moves))
	resolved := make([]string, 0, len(e.Moves))
	for _, move := range e.Moves {
		bases = append(bases, strconv.Itoa(move.Base))
		resolved = append(resolved, strconv.Itoa(move.Resolved))
	}
	return fmt.Sprintf(domain.EnvPortMoveFmt,
		strings.Join(bases, domain.CmdListVarSep), strings.Join(resolved, domain.CmdListVarSep))
}

type envPortValueWidthParams struct {
	Width           int
	Key, Port, Move int
}

// envPortValueWidth is what the last column has left once the three fixed ones
// are placed. Below a floor the value says nothing at all, so a surface too
// narrow to hold the table is given a value it can still recognise and left to
// wrap — cutting to four characters would only hide the anomaly.
func envPortValueWidth(params envPortValueWidthParams) int {
	if params.Width <= 0 {
		return domain.EnvValueDisplayWidth
	}
	fixed := params.Key + params.Port + params.Move + 3*len(domain.EnvPortColumnGap)
	return max(params.Width-fixed, domain.EnvValueMinWidth)
}

func envPortColumnWidths(entries []domain.EnvPortEntry) (key, port, move int) {
	key, port, move = len(domain.EnvPortHeaderKey), len(domain.EnvPortHeaderFollows), len(domain.EnvPortHeaderPort)
	for _, e := range entries {
		key = max(key, len(e.Key))
		port = max(port, len(e.Port))
		move = max(move, len([]rune(envPortMove(e))))
	}
	return key, port, move
}

// envPortFiles lists the files the entries touch, sorted so a report of the same
// plan always reads the same way.
func envPortFiles(entries []domain.EnvPortEntry) []string {
	seen := map[string]bool{}
	var files []string
	for _, e := range entries {
		if seen[e.File] {
			continue
		}
		seen[e.File] = true
		files = append(files, e.File)
	}
	sort.Strings(files)
	return files
}

func pad(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-len([]rune(s))))
}

// Pad right-pads a label so a surface can align a column the same way the line
// builders here do.
func Pad(s string, width int) string { return pad(s, width) }

// EnvPortLinkLines describes each link as both the prompt and the recap show it:
// where the key lives, which port it follows, and the base found inside its
// value — the number that answers "why this key?" before the link is written,
// and "what moves?" after.
func EnvPortLinkLines(links []domain.EnvPortLink, bases map[domain.PortRef]int) []string {
	width := 0
	for _, l := range links {
		width = max(width, len([]rune(l.File+domain.EnvPortLinkSeparator+l.Key)))
	}

	lines := make([]string, 0, len(links))
	for _, l := range links {
		base, _ := EnvPortBaseFor(bases, l)
		format := domain.EnvPortLinkFmt
		if l.ByDir {
			format = domain.EnvPortLinkByDirFmt
		}
		lines = append(lines, fmt.Sprintf(format,
			pad(l.File+domain.EnvPortLinkSeparator+l.Key, width), portLabel(l), base))
	}
	return lines
}

// portLabel names the job alongside the port: the name alone does not say which
// one a key follows.
func portLabel(link domain.EnvPortLink) string {
	return link.Job + domain.EnvPortJobSeparator + link.Port
}

// EnvPortNotice is one thing a surface says beside the table: a title and the
// single line explaining it. Two of them exist, and neither repeats per key —
// they are properties of the machine, not of a value.
type EnvPortNotice struct {
	Title string
	Line  string
}

// EnvPortNotices is what the pass could not do silently. A .env freezes an
// address, so the moment a perishable one enters a file is the moment to say so.
func EnvPortNotices(plan domain.EnvPortPlan) []EnvPortNotice {
	if plan.Addressing != domain.AddressingNames || len(plan.Entries) == 0 {
		return nil
	}

	if plan.PublicPort == 0 {
		return []EnvPortNotice{{
			Title: domain.EnvOriginProxyOffTitle,
			Line:  domain.EnvOriginProxyOffLine,
		}}
	}
	if plan.PublicPort == domain.ProxyPrivilegedPort || !writesAddresses(plan) {
		return nil
	}
	return []EnvPortNotice{{
		Title: domain.EnvOriginPortedTitle,
		Line:  fmt.Sprintf(domain.EnvOriginPortedFmt, plan.PublicPort),
	}}
}

// writesAddresses reports whether any entry of the plan holds an address at all,
// settled or about to be written. A project whose every link is a bare port has
// nothing to be told about the proxy's port.
func writesAddresses(plan domain.EnvPortPlan) bool {
	for _, e := range plan.Entries {
		if e.Addressing == domain.AddressingNames {
			return true
		}
	}
	return false
}

// EnvPortOffsetLabel titles the table with the offset the whole worktree runs on,
// so a reader who wonders why every port moved by the same amount has the answer.
func EnvPortOffsetLabel(offset int) string {
	return domain.EnvPortsTitle + " " + domain.EnvPortOffsetPrefix + strconv.Itoa(offset)
}

// EnvPortSettlementNote is the port pass as a create-like recap carries it: a
// count and an offset, never the values. Empty when the run moved nothing.
func EnvPortSettlementNote(settlement domain.EnvPortSettlement) string {
	if settlement.Shifted == 0 {
		return ""
	}
	if !settlement.Applied {
		return fmt.Sprintf(domain.EnvPortsRecapKeptFmt, settlement.Shifted)
	}
	return fmt.Sprintf(domain.EnvPortsRecapShiftedFmt, settlement.Shifted, settlement.Offset)
}

type EnvPortOutcomeParams struct {
	Plan  domain.EnvPortPlan
	Check bool
}

// EnvPortOutcomeLine is what a report says of the port pass beyond its anomalies.
// An applied pass says nothing: the trailing summary already counts it, and a
// second telling is what the table used to be.
func EnvPortOutcomeLine(params EnvPortOutcomeParams) string {
	rewrites := len(EnvPortRewrites(params.Plan))
	if rewrites == 0 {
		return ""
	}
	if params.Check {
		return fmt.Sprintf(domain.EnvPortsWouldShiftFmt, rewrites, params.Plan.Offset)
	}
	if !params.Plan.Applied {
		return fmt.Sprintf(domain.EnvPortsLeftAloneFmt, rewrites)
	}
	return ""
}

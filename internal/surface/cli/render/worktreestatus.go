package render

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

func WriteStatusJSON(w io.Writer, doc domain.StatusDocument) error {
	return encodeJSON(w, doc)
}

type FormatStatusParams struct {
	Document   domain.StatusDocument
	ProjectDir string
	Hyperlinks bool
}

// FormatStatus concludes on whether anything is left to fix, then reads the
// worktree back, and names each problem with the command that clears it. Raw
// body — the caller's frame owns the padding.
func FormatStatus(w io.Writer, params FormatStatusParams) {
	doc := params.Document
	headline := rules.StatusHeadline(doc)
	if len(doc.Problems) == 0 {
		Unchanged(w, headline)
	} else {
		Warning(w, headline)
	}
	Blank(w)
	writeAlignedFields(w, rules.StatusFields(rules.StatusFieldsParams{Document: doc, ProjectDir: params.ProjectDir}))

	if doc.RunConfig {
		Blank(w)
		writeStatusJobs(w, writeStatusJobsParams{Jobs: doc.Jobs, Hyperlinks: params.Hyperlinks})
	}

	if len(doc.Problems) > 0 {
		Blank(w)
		writeStatusProblems(w, doc.Problems)
	}
}

// writeStatusProblems pairs each problem with the command that clears it.
func writeStatusProblems(w io.Writer, problems []domain.StatusProblem) {
	for _, problem := range problems {
		Warning(w, problem.Message)
		NextStep(w, NextStepParams{Command: problem.Fix})
	}
}

type writeStatusJobsParams struct {
	Jobs       []domain.JobSnapshot
	Hyperlinks bool
}

func writeStatusJobs(w io.Writer, params writeStatusJobsParams) {
	if len(params.Jobs) == 0 {
		Unchanged(w, domain.StatusJobsEmpty)
		return
	}
	rows := rules.StatusJobRows(params.Jobs)
	items := make([]AnnounceItem, 0, len(rows))
	for _, row := range rows {
		value := row.Value
		if params.Hyperlinks {
			value = rules.LinkURLs(value)
		}
		items = append(items, AnnounceItem{Label: row.Label, Value: value})
	}
	Announce(w, domain.StatusJobsTitle, items)
}

func WriteStatusAllJSON(w io.Writer, docs []domain.StatusDocument) error {
	return encodeJSON(w, docs)
}

// FormatStatusAll counts the worktrees, lists them as a table — an inventory
// — and then names each problem under the worktree it belongs to, with its fix.
// Raw body — the caller's frame owns the padding.
func FormatStatusAll(w io.Writer, docs []domain.StatusDocument) {
	headline := rules.StatusAllHeadline(docs)
	troubled := rules.StatusTroubled(docs)
	if len(troubled) == 0 {
		Unchanged(w, headline)
	} else {
		Warning(w, headline)
	}
	Blank(w)
	writeStatusTable(w, rules.StatusTable(docs))
	for _, doc := range troubled {
		Blank(w)
		SectionTitle(w, doc.Branch)
		writeStatusProblems(w, doc.Problems)
	}
}

// writeStatusTable pads every cell to its column, the header muted as chrome,
// and marks a row needing attention with the glyph in a margin column of its
// own, so the names stay aligned whatever the row says.
func writeStatusTable(w io.Writer, table domain.StatusTable) {
	widths := make([]int, len(table.Header))
	for i, title := range table.Header {
		widths[i] = utf8.RuneCountInString(title)
	}
	for _, row := range table.Rows {
		for i, cell := range row.Cells {
			widths[i] = max(widths[i], utf8.RuneCountInString(cell))
		}
	}
	fmt.Fprintf(w, "%s%s%s\n", Indent, statusMargin(false), styles.Muted.Render(statusCells(statusCellsParams{Cells: table.Header, Widths: widths})))
	for _, row := range table.Rows {
		fmt.Fprintf(w, "%s%s%s\n", Indent, statusMargin(row.Attention), statusCells(statusCellsParams{Cells: row.Cells, Widths: widths}))
	}
}

func statusMargin(attention bool) string {
	if !attention {
		return statusMarginBlank
	}
	return styles.Warning.Render(domain.GlyphAttention) + "  "
}

const statusMarginBlank = "   "

type statusCellsParams struct {
	Cells  []string
	Widths []int
}

// statusCells leaves the last column unpadded: trailing spaces would only
// widen the line.
func statusCells(params statusCellsParams) string {
	var b strings.Builder
	last := len(params.Cells) - 1
	for i, cell := range params.Cells {
		b.WriteString(cell)
		if i == last {
			break
		}
		b.WriteString(strings.Repeat(" ", params.Widths[i]-utf8.RuneCountInString(cell)+2))
	}
	return b.String()
}

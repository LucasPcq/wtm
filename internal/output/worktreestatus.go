package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
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
	if len(doc.Problems) == 0 {
		Unchanged(w, fmt.Sprintf(domain.StatusHeadlineCleanFmt, doc.Branch))
	} else {
		Warning(w, fmt.Sprintf(domain.StatusHeadlineProblemsFmt, doc.Branch, len(doc.Problems)))
	}
	Blank(w)
	writeAlignedFields(w, rules.StatusFields(rules.StatusFieldsParams{Document: doc, ProjectDir: params.ProjectDir}))

	if doc.RunConfig {
		Blank(w)
		writeStatusJobs(w, writeStatusJobsParams{Jobs: doc.Jobs, Hyperlinks: params.Hyperlinks})
	}

	for _, problem := range doc.Problems {
		Blank(w)
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

// FormatStatusAll counts the worktrees, then gives each one line, and expands
// only the ones with something to fix. Raw body — the caller's frame owns the
// padding.
func FormatStatusAll(w io.Writer, docs []domain.StatusDocument) {
	headline := rules.StatusAllHeadline(docs)
	if rules.StatusTroubled(docs) {
		Warning(w, headline)
	} else {
		Unchanged(w, headline)
	}
	for _, doc := range docs {
		Blank(w)
		writeStatusLine(w, doc)
	}
}

func writeStatusLine(w io.Writer, doc domain.StatusDocument) {
	line := doc.Branch + domain.StatusSummarySeparator + rules.StatusSummary(doc)
	if len(doc.Problems) == 0 {
		Unchanged(w, line)
		return
	}
	Warning(w, line)
	for _, problem := range doc.Problems {
		Message(w, Indent+problem.Message)
		fmt.Fprintln(w, Indent+NextStepLine(NextStepParams{Command: problem.Fix}))
	}
}

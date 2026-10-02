package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func WriteExtractJSON(w io.Writer, result domain.ExtractResult) error {
	return encodeJSON(w, result)
}

type ExtractResultParams struct {
	Result domain.ExtractResult
	// Path is the target as the reader knows it, base_path/<name> when it lives
	// there.
	Path string
	// EnvNote is what the port pass did in a worktree the extraction created —
	// a count and an offset (rules.EnvPortSettlementNote), empty otherwise.
	EnvNote string
}

// PrintExtractResult lists every file it moved: a move takes them out of the
// source, and knowing what left is what the reader acts on next. Raw body.
func PrintExtractResult(w io.Writer, params ExtractResultParams) {
	result := params.Result
	headline, state := domain.ExtractMovedFmt, domain.ExtractSourceCleaned
	if result.Kept {
		headline, state = domain.ExtractCopiedFmt, domain.ExtractSourceKept
	}

	Success(w, fmt.Sprintf(headline, rules.FileCount(len(result.Files)), result.TargetBranch))
	Blank(w)
	writeExtractFiles(w, result.Files)
	Blank(w)
	fields := []domain.RecapField{
		{Label: domain.ExtractLabelSource, Value: result.SourceBranch + " · " + state},
		{Label: domain.CreateRecapLabelPath, Value: params.Path},
	}
	if params.EnvNote != "" {
		fields = append(fields, domain.RecapField{Label: domain.CreateRecapLabelEnv, Value: params.EnvNote})
	}
	writeAlignedFields(w, fields)
	Blank(w)
	NextStep(w, NextStepParams{Command: fmt.Sprintf(domain.GoCommandFmt, result.TargetBranch)})
}

func writeExtractFiles(w io.Writer, files []domain.ExtractFile) {
	for _, file := range files {
		fmt.Fprintf(w, "%s%s%s  %s\n", Indent, Indent, rules.ExtractStatusLabel(file.Status), file.Path)
	}
}

type ExtractConflictsParams struct {
	Result domain.ExtractResult
	Path   string
}

// PrintExtractConflicts is the rebase-style stop: what is left to resolve, and
// that the source still holds everything. Raw body.
func PrintExtractConflicts(w io.Writer, params ExtractConflictsParams) {
	result := params.Result
	Warning(w, fmt.Sprintf(domain.ExtractConflictsFmt, result.TargetBranch))
	Blank(w)
	SectionTitle(w, domain.ExtractConflictsTitle)
	for _, file := range result.Conflicts {
		fmt.Fprintf(w, "%s%s%s\n", Indent, Indent, file)
	}
	Blank(w)
	if len(result.Files) > len(result.Conflicts) {
		Message(w, domain.ExtractConflictsOthersApplied)
	}
	Message(w, fmt.Sprintf(domain.ExtractConflictsSourceSafeFmt, result.SourceBranch, result.TargetBranch))
	Blank(w)
	writeAlignedFields(w, []domain.RecapField{{Label: domain.CreateRecapLabelPath, Value: params.Path}})
	Blank(w)
	NextStep(w, NextStepParams{
		Command: fmt.Sprintf(domain.GoCommandFmt, result.TargetBranch),
		Note:    fmt.Sprintf(domain.ExtractConflictsNextFmt, result.SourceBranch),
	})
}

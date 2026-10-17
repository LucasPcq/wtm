package render

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func ExecResultLine(w io.Writer, result domain.ExecResult) {
	if result.Status == domain.ExecStatusPassed {
		Success(w, rules.ExecResultLabel(result))
		return
	}
	Error(w, rules.ExecResultLabel(result))
}

type ExecConclusionParams struct {
	Command string
	Results []domain.ExecResult
	Elapsed time.Duration
}

func FormatExecConclusion(w io.Writer, params ExecConclusionParams) {
	counts := rules.CountExec(params.Results)
	total := len(params.Results)
	if counts.Passed == total {
		Success(w, fmt.Sprintf(domain.ExecAllPassedFmt, params.Command, rules.ExecWorktreeCount(total), rules.HookDuration(params.Elapsed)))
		return
	}

	Error(w, fmt.Sprintf(domain.ExecNotAllPassedFmt, params.Command, rules.ExecWorktreeCount(total), rules.ExecShortfall(counts)))
	nested := &prefixWriter{w: w, prefix: Indent}
	for _, result := range params.Results {
		if result.Status == domain.ExecStatusPassed {
			continue
		}
		Error(nested, rules.ExecResultLabel(result))
		for _, line := range lastLines(result.Tail, domain.ExecConclusionTailLines) {
			Message(nested, Indent+Indent+line)
		}
		if result.Log != "" && result.Status == domain.ExecStatusFailed && result.Error == "" {
			InfoLine(nested, Indent+domain.ExecLogLabel, result.Log)
		}
	}
	if counts.Passed > 0 {
		Success(nested, fmt.Sprintf(domain.ExecPassedCountFmt, counts.Passed))
	}
}

func FormatExecPrint(w io.Writer, results []domain.ExecResult) {
	printed := 0
	for _, result := range results {
		if result.Output == "" {
			continue
		}
		if printed > 0 {
			Blank(w)
		}
		printed++
		SectionTitle(w, result.Branch)
		for _, line := range strings.Split(strings.TrimRight(result.Output, "\n"), "\n") {
			Message(w, Indent+line)
		}
	}
}

type ExecJSONParams struct {
	Command string
	Results []domain.ExecResult
}

type execJSON struct {
	Command string              `json:"command"`
	Results []domain.ExecResult `json:"results"`
	Failed  []string            `json:"failed"`
}

func WriteExecJSON(w io.Writer, params ExecJSONParams) error {
	results := params.Results
	if results == nil {
		results = []domain.ExecResult{}
	}
	return encodeJSON(w, execJSON{Command: params.Command, Results: results, Failed: rules.ExecFailedBranches(results)})
}

func lastLines(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}

type prefixWriter struct {
	w      io.Writer
	prefix string
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	lines := strings.SplitAfter(string(b), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if _, err := io.WriteString(p.w, p.prefix+line); err != nil {
			return 0, err
		}
	}
	return len(b), nil
}

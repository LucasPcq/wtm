package rules

import (
	"fmt"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type StatusJobsParams struct {
	// Declared is run.toml's jobs, in its order.
	Declared []domain.JobConfig
	// Up is what the daemon or its index holds for the worktree (WorktreeJobs).
	Up []domain.JobSnapshot
	// URLs is where each declared job answers in this worktree.
	URLs map[string]string
}

// StatusJobs is every job a worktree can run, with the state the events
// stream would give it: a declared job nothing holds is stopped, and one still
// running after leaving run.toml keeps its place at the end.
func StatusJobs(params StatusJobsParams) []domain.JobSnapshot {
	jobs := make([]domain.JobSnapshot, 0, len(params.Declared)+len(params.Up))
	for _, declared := range params.Declared {
		job := domain.JobSnapshot{Name: declared.Name, Kind: declared.Kind, State: domain.JobStateStopped}
		if index := slices.IndexFunc(params.Up, func(up domain.JobSnapshot) bool { return up.Name == declared.Name }); index >= 0 {
			job = params.Up[index]
		}
		if url := params.URLs[declared.Name]; url != "" {
			job.URL = url
		}
		jobs = append(jobs, job)
	}
	for _, up := range params.Up {
		if !stillUp(up) || slices.ContainsFunc(params.Declared, func(declared domain.JobConfig) bool { return declared.Name == up.Name }) {
			continue
		}
		jobs = append(jobs, up)
	}
	return jobs
}

// stillUp keeps a job run.toml no longer declares only while it runs: once it
// ended there is nothing to start again, and a fix naming it would be refused.
func stillUp(job domain.JobSnapshot) bool {
	return job.State == domain.JobStateRunning || job.State == domain.JobStateStarting
}

type StatusProblemsParams struct {
	Branch           string
	MissingEnv       []domain.EnvMissingFile
	Jobs             []domain.JobSnapshot
	IsolationPending bool
}

// StatusProblems names each anomaly with the one command that clears it. A
// branch may hold `;` or `$`, and a fix is meant to be run as printed: every
// name is quoted as one shell word.
func StatusProblems(params StatusProblemsParams) []domain.StatusProblem {
	problems := []domain.StatusProblem{}
	for _, file := range params.MissingEnv {
		problems = append(problems, envMissingProblem(envMissingParams{File: file, Branch: params.Branch}))
	}
	if params.IsolationPending {
		problems = append(problems, domain.StatusProblem{
			Code:    domain.StatusProblemIsolationPending,
			Message: domain.StatusProblemIsolationPendingMessage,
			Fix:     fmt.Sprintf(domain.StatusFixIsolationPendingFmt, shellQuote(params.Branch)),
		})
	}
	for _, job := range params.Jobs {
		if job.State != domain.JobStateCrashed {
			continue
		}
		problems = append(problems, domain.StatusProblem{
			Code:    domain.StatusProblemJobCrashed,
			Message: crashedMessage(job),
			Fix:     fmt.Sprintf(domain.StatusFixJobCrashedFmt, shellQuote(params.Branch), shellQuote(job.Name)),
		})
	}
	return problems
}

type envMissingParams struct {
	File   domain.EnvMissingFile
	Branch string
}

// envMissingProblem names the run of `wtm env` that rebuilds the file as
// create would have, from the worktree's own strategy first.
func envMissingProblem(params envMissingParams) domain.StatusProblem {
	file := params.File
	problem := domain.StatusProblem{Code: domain.StatusProblemEnvMissing, Message: fmt.Sprintf(domain.StatusProblemEnvMissingFmt, file.Target)}
	switch {
	case file.Scaffolded:
		problem.Fix = fmt.Sprintf(domain.StatusFixEnvMissingFmt, shellQuote(params.Branch))
	case file.HasTemplate:
		problem.Fix = fmt.Sprintf(domain.StatusFixEnvFromTemplateFmt, shellQuote(params.Branch))
	default:
		problem.Message = fmt.Sprintf(domain.StatusProblemEnvNowhereFmt, file.Target)
		problem.Fix = domain.StatusFixEnvNowhere
	}
	return problem
}

// MissingEnvTargets is the document's list of the files a worktree lacks.
func MissingEnvTargets(files []domain.EnvMissingFile) []string {
	targets := make([]string, 0, len(files))
	for _, file := range files {
		targets = append(targets, file.Target)
	}
	return targets
}

func crashedMessage(job domain.JobSnapshot) string {
	if job.ExitCode == nil {
		return fmt.Sprintf(domain.StatusProblemJobCrashedFmt, job.Name)
	}
	if *job.ExitCode < 0 {
		return fmt.Sprintf(domain.StatusProblemJobKilledFmt, job.Name)
	}
	return fmt.Sprintf(domain.StatusProblemJobCrashedCodeFmt, job.Name, *job.ExitCode)
}

type StatusFieldsParams struct {
	Document   domain.StatusDocument
	ProjectDir string
}

// StatusFields is the readout under the headline: where the worktree is, how
// it runs, and what its .env lacks, counted.
func StatusFields(params StatusFieldsParams) []domain.RecapField {
	doc := params.Document
	path := DisplayPath(DisplayPathParams{Base: params.ProjectDir, Target: doc.Path})
	if doc.Main {
		path = doc.Path + domain.StatusMainSuffix
	}
	fields := []domain.RecapField{
		{Label: domain.StatusFieldPath, Value: path},
		{Label: domain.StatusFieldIsolation, Value: statusIsolation(doc)},
	}
	if !doc.RunConfig {
		fields = append(fields, domain.RecapField{Label: domain.StatusFieldRun, Value: domain.StatusRunConfigAbsent})
	}
	if doc.Addressing != nil {
		fields = append(fields, domain.RecapField{Label: domain.StatusFieldAddressing, Value: string(*doc.Addressing)})
	}
	if doc.RunConfig {
		fields = append(fields, domain.RecapField{Label: domain.StatusFieldPorts, Value: statusPorts(doc)})
	}
	return append(fields, domain.RecapField{Label: domain.StatusFieldEnv, Value: statusEnv(doc.Env)})
}

func isolationPending(doc domain.StatusDocument) bool {
	return slices.ContainsFunc(doc.Problems, func(problem domain.StatusProblem) bool {
		return problem.Code == domain.StatusProblemIsolationPending
	})
}

// statusIsolation reads a worktree that never chose as such: the isolation a
// run would apply to it is not one it has.
func statusIsolation(doc domain.StatusDocument) string {
	if isolationPending(doc) {
		return domain.StatusIsolationNotChosen
	}
	return string(doc.Isolation)
}

// statusPorts says which ports the worktree binds, relative to the main
// checkout's: its own offset, the main's own, or its source's.
func statusPorts(doc domain.StatusDocument) string {
	switch {
	case isolationPending(doc):
		return domain.StatusCellNone
	case doc.Isolation == domain.IsolationVerbatim:
		return domain.StatusPortsSource
	case doc.Main:
		return domain.StatusPortsBase
	case doc.Offset == nil:
		return domain.StatusPortsUnallocated
	default:
		return fmt.Sprintf(domain.StatusPortsOffsetFmt, *doc.Offset)
	}
}

func statusEnv(env domain.StatusEnv) string {
	files := Plural(PluralParams{Count: env.Declared, One: domain.StatusNounFile, Many: domain.StatusNounFiles})
	if len(env.Missing) == 0 {
		return files
	}
	return files + domain.TallySeparator + fmt.Sprintf(domain.StatusEnvMissingFmt, len(env.Missing))
}

// StatusJobRows is one aligned row per job: its state, then where it answers
// or which worktree holds the shared service it reads.
func StatusJobRows(jobs []domain.JobSnapshot) []domain.RecapField {
	width := 0
	for _, job := range jobs {
		width = max(width, len(job.State))
	}
	rows := make([]domain.RecapField, 0, len(jobs))
	for _, job := range jobs {
		detail := job.URL
		if job.Owner != nil {
			detail = strings.TrimSpace(detail + " " + fmt.Sprintf(domain.StatusJobOwnerFmt, job.Owner.Branch))
		}
		value := string(job.State)
		if detail != "" {
			value += strings.Repeat(" ", width-len(job.State)) + "  " + detail
		}
		rows = append(rows, domain.RecapField{Label: job.Name, Value: value})
	}
	return rows
}

// StatusTable is the inventory `status --all` prints: one row per worktree,
// and the run columns only when run.toml declares jobs.
func StatusTable(docs []domain.StatusDocument) domain.StatusTable {
	withRun := slices.ContainsFunc(docs, func(doc domain.StatusDocument) bool { return doc.RunConfig })
	header := []string{domain.StatusColWorktree}
	if withRun {
		header = append(header, domain.StatusColIsolation, domain.StatusColPorts, domain.StatusColJobs)
	}
	table := domain.StatusTable{Header: append(header, domain.StatusColEnv)}
	for _, doc := range docs {
		cells := []string{doc.Branch}
		if withRun {
			cells = append(cells, statusIsolation(doc), statusPorts(doc), statusJobsCell(doc.Jobs))
		}
		table.Rows = append(table.Rows, domain.StatusRow{
			Cells:     append(cells, statusEnvCell(doc.Env)),
			Attention: len(doc.Problems) > 0,
		})
	}
	return table
}

func statusJobsCell(jobs []domain.JobSnapshot) string {
	counts := map[domain.JobState]int{}
	for _, job := range jobs {
		counts[job.State]++
	}
	tally := Tally(
		domain.TallyPart{Count: counts[domain.JobStateRunning], Label: string(domain.JobStateRunning)},
		domain.TallyPart{Count: counts[domain.JobStateStarting], Label: string(domain.JobStateStarting)},
		domain.TallyPart{Count: counts[domain.JobStateCrashed], Label: string(domain.JobStateCrashed)},
		domain.TallyPart{Count: counts[domain.JobStateExited], Label: string(domain.JobStateExited)},
		domain.TallyPart{Count: counts[domain.JobStateStopped], Label: string(domain.JobStateStopped)},
	)
	if tally == "" {
		return domain.StatusCellNone
	}
	return tally
}

// statusEnvCell is the one thing a table row needs of the .env files: whether
// any is missing.
func statusEnvCell(env domain.StatusEnv) string {
	if len(env.Missing) > 0 {
		return fmt.Sprintf(domain.StatusEnvMissingFmt, len(env.Missing))
	}
	return Plural(PluralParams{Count: env.Declared, One: domain.StatusNounFile, Many: domain.StatusNounFiles})
}

// StatusHeadline concludes on one worktree: nothing to fix, or how much.
func StatusHeadline(doc domain.StatusDocument) string {
	if len(doc.Problems) == 0 {
		return fmt.Sprintf(domain.StatusHeadlineCleanFmt, doc.Branch)
	}
	return fmt.Sprintf(domain.StatusHeadlineProblemsFmt, doc.Branch,
		Plural(PluralParams{Count: len(doc.Problems), One: domain.StatusNounProblem, Many: domain.StatusNounProblems}))
}

// StatusAllHeadline counts the worktrees, and how many have something to fix.
func StatusAllHeadline(docs []domain.StatusDocument) string {
	worktrees := Plural(PluralParams{Count: len(docs), One: domain.StatusNounWorktree, Many: domain.StatusNounWorktrees})
	troubled := len(StatusTroubled(docs))
	if troubled == 0 {
		return fmt.Sprintf(domain.StatusAllCleanFmt, worktrees)
	}
	verb := domain.StatusNeedsAttention
	if troubled > 1 {
		verb = domain.StatusNeedAttention
	}
	return fmt.Sprintf(domain.StatusAllProblemsFmt, worktrees, troubled, verb)
}

// StatusTroubled is the worktrees with something to fix, in their order.
func StatusTroubled(docs []domain.StatusDocument) []domain.StatusDocument {
	troubled := []domain.StatusDocument{}
	for _, doc := range docs {
		if len(doc.Problems) > 0 {
			troubled = append(troubled, doc)
		}
	}
	return troubled
}

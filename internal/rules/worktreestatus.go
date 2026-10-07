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
// up after leaving run.toml keeps its place at the end.
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
		if slices.ContainsFunc(params.Declared, func(declared domain.JobConfig) bool { return declared.Name == up.Name }) {
			continue
		}
		jobs = append(jobs, up)
	}
	return jobs
}

type StatusProblemsParams struct {
	Branch           string
	MissingEnv       []domain.EnvMissingFile
	Jobs             []domain.JobSnapshot
	IsolationPending bool
}

// StatusProblems names each anomaly with the one command that clears it.
func StatusProblems(params StatusProblemsParams) []domain.StatusProblem {
	problems := []domain.StatusProblem{}
	for _, file := range params.MissingEnv {
		problems = append(problems, envMissingProblem(envMissingParams{File: file, Branch: params.Branch}))
	}
	if params.IsolationPending {
		problems = append(problems, domain.StatusProblem{
			Code:    domain.StatusProblemIsolationPending,
			Message: fmt.Sprintf(domain.StatusProblemIsolationPendingFmt, params.Branch),
			Fix:     fmt.Sprintf(domain.StatusFixIsolationPendingFmt, params.Branch),
		})
	}
	for _, job := range params.Jobs {
		if job.State != domain.JobStateCrashed {
			continue
		}
		problems = append(problems, domain.StatusProblem{
			Code:    domain.StatusProblemJobCrashed,
			Message: crashedMessage(job),
			Fix:     fmt.Sprintf(domain.StatusFixJobCrashedFmt, params.Branch, job.Name),
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
		problem.Fix = fmt.Sprintf(domain.StatusFixEnvMissingFmt, params.Branch)
	case file.HasTemplate:
		problem.Fix = fmt.Sprintf(domain.StatusFixEnvFromTemplateFmt, params.Branch)
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
		{Label: domain.StatusFieldIsolation, Value: string(doc.Isolation)},
	}
	if !doc.RunConfig {
		fields = append(fields, domain.RecapField{Label: domain.StatusFieldRun, Value: domain.StatusRunConfigAbsent})
	}
	if doc.Addressing != nil {
		fields = append(fields, domain.RecapField{Label: domain.StatusFieldAddressing, Value: string(*doc.Addressing)})
	}
	if doc.RunConfig {
		fields = append(fields, domain.RecapField{Label: domain.StatusFieldOffset, Value: statusOffset(doc.Offset)})
	}
	return append(fields, domain.RecapField{Label: domain.StatusFieldEnv, Value: statusEnv(doc.Env)})
}

func statusOffset(offset *int) string {
	if offset == nil {
		return domain.StatusOffsetUnallocated
	}
	return fmt.Sprintf(domain.StatusOffsetFmt, *offset)
}

func statusEnv(env domain.StatusEnv) string {
	if len(env.Missing) == 0 {
		return fmt.Sprintf(domain.StatusEnvFilesFmt, env.Declared)
	}
	return fmt.Sprintf(domain.StatusEnvMissingCountFmt, env.Declared, len(env.Missing))
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

// StatusSummary is a worktree on one line, for the readout of every worktree:
// how it runs, then its jobs counted by state.
func StatusSummary(doc domain.StatusDocument) string {
	parts := []string{string(doc.Isolation)}
	if doc.RunConfig {
		parts = append(parts, statusOffset(doc.Offset))
	}
	counts := map[domain.JobState]int{}
	for _, job := range doc.Jobs {
		counts[job.State]++
	}
	tally := Tally(
		domain.TallyPart{Count: counts[domain.JobStateRunning], Label: string(domain.JobStateRunning)},
		domain.TallyPart{Count: counts[domain.JobStateStarting], Label: string(domain.JobStateStarting)},
		domain.TallyPart{Count: counts[domain.JobStateCrashed], Label: string(domain.JobStateCrashed)},
		domain.TallyPart{Count: counts[domain.JobStateExited], Label: string(domain.JobStateExited)},
		domain.TallyPart{Count: counts[domain.JobStateStopped], Label: string(domain.JobStateStopped)},
	)
	if tally != "" {
		parts = append(parts, tally)
	}
	return strings.Join(parts, domain.TallySeparator)
}

// StatusAllHeadline counts the worktrees, and how many have something to fix.
func StatusAllHeadline(docs []domain.StatusDocument) string {
	troubled := 0
	for _, doc := range docs {
		if len(doc.Problems) > 0 {
			troubled++
		}
	}
	if !StatusTroubled(docs) {
		return fmt.Sprintf(domain.StatusAllCleanFmt, len(docs))
	}
	return fmt.Sprintf(domain.StatusAllProblemsFmt, len(docs), troubled)
}

func StatusTroubled(docs []domain.StatusDocument) bool {
	return slices.ContainsFunc(docs, func(doc domain.StatusDocument) bool { return len(doc.Problems) > 0 })
}

package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type JobsMissingPortRefParams struct {
	Config domain.RunConfig
	// Exempt names the jobs whose ports are read somewhere other than their
	// command — a compose stack takes them from its file.
	Exempt []string
}

// JobsMissingPortRef names the services declaring a port their command never
// mentions. It proves nothing: a command can read the variable from the
// environment on its own. It only asks the question wtm cannot answer, at the
// one moment the answer is still cheap.
func JobsMissingPortRef(params JobsMissingPortRefParams) []domain.JobCmdFix {
	exempt := make(map[string]bool, len(params.Exempt))
	for _, name := range params.Exempt {
		exempt[name] = true
	}

	var fixes []domain.JobCmdFix
	for _, job := range params.Config.Jobs {
		if !ShouldProbeJob(ShouldProbeJobParams{Kind: job.Kind, Ports: job.Ports, Probe: job.Probe}) || exempt[job.Name] {
			continue
		}
		var missing []string
		for _, name := range sortedPortNames(job.Ports) {
			if !cmdMentions(job.Cmd, name) {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			fixes = append(fixes, domain.JobCmdFix{Job: job.Name, Cmd: job.Cmd, Vars: missing})
		}
	}
	sort.SliceStable(fixes, func(i, j int) bool { return fixes[i].Job < fixes[j].Job })
	return fixes
}

// CmdMissesItsPort reports whether a command still fails to mention any of the
// variables it was flagged for. It is the same reading JobsMissingPortRef does,
// re-applied to a command a surface has since edited.
func CmdMissesItsPort(fix domain.JobCmdFix) bool {
	return len(PortVarsMissingFrom(fix)) > 0
}

// PortVarsMissingFrom is which of the flagged variables the command still does
// not mention, in the order they were flagged.
func PortVarsMissingFrom(fix domain.JobCmdFix) []string {
	var missing []string
	for _, name := range fix.Vars {
		if !cmdMentions(fix.Cmd, name) {
			missing = append(missing, name)
		}
	}
	return missing
}

func cmdMentions(cmd, name string) bool { return strings.Contains(cmd, name) }

type JobsReadingTheirEnvParams struct {
	Config domain.RunConfig
	// ScansByDir is what each directory's .env files declare, keyed by the
	// directory a job's cwd names.
	ScansByDir map[string]domain.EnvPortScan
}

// JobsReadingTheirEnv names the jobs whose every declared port was read from
// their own .env. Such a job demonstrably reads that variable — the value came
// from the file the job itself loads — so the command is not the channel and
// grepping it for the name proves nothing.
//
// Without this, a repository of well-behaved apps was told all of them were
// broken: a Vite config reading process.env.VITE_PORT, a server reading PORT
// from its env module, are the normal 12-factor case, and none of them names
// the variable on the command line.
func JobsReadingTheirEnv(params JobsReadingTheirEnvParams) []string {
	var names []string
	for _, job := range params.Config.Jobs {
		if len(job.Ports) == 0 {
			continue
		}
		scan, found := params.ScansByDir[ScriptJobCwd(job.Cwd)]
		if !found {
			continue
		}
		if declaredInScan(job, scan) {
			names = append(names, job.Name)
		}
	}
	return names
}

func declaredInScan(job domain.JobConfig, scan domain.EnvPortScan) bool {
	for name, base := range job.Ports {
		if scanned, ok := scan.Ports[name]; !ok || scanned != base {
			return false
		}
	}
	return true
}

type ComposeJobsParams struct {
	Config domain.RunConfig
	Files  []string
}

// ComposeJobsFor names the jobs the given compose files back. It is the exempt
// list two callers need — the ports step and the final report — and computing it
// twice let the wizard and the report exempt different jobs.
func ComposeJobsFor(params ComposeJobsParams) []string {
	var jobs []string
	for _, file := range params.Files {
		if job := ComposeJobName(ComposeJobNameParams{Config: params.Config, File: file}); job != "" {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

type PortIsolationLinesParams struct {
	Unported []string
	Ignoring []domain.JobCmdFix
}

// PortIsolationLines names every job that will bind the same port in every
// worktree, and why. A job with no port and a job whose command ignores the one
// it was given fail the same way, so they are listed together.
func PortIsolationLines(params PortIsolationLinesParams) []string {
	if len(params.Unported) == 0 && len(params.Ignoring) == 0 {
		return nil
	}

	width := 0
	for _, job := range params.Unported {
		width = max(width, len([]rune(job)))
	}
	for _, fix := range params.Ignoring {
		width = max(width, len([]rune(fix.Job)))
	}

	var lines []string
	for _, job := range params.Unported {
		lines = append(lines, fmt.Sprintf(domain.PortIsolationLineFmt, pad(job, width), domain.PortIsolationNoPort))
	}
	for _, fix := range params.Ignoring {
		lines = append(lines, fmt.Sprintf(domain.PortIsolationLineFmt, pad(fix.Job, width),
			fmt.Sprintf(domain.PortIsolationIgnoresFmt, strings.Join(fix.Vars, ", "))))
	}
	return append(lines, "", domain.PortIsolationHint)
}

type JobsIsolatedByCommandParams struct {
	Config     domain.RunConfig
	Exempt     []string
	ScansByDir map[string]domain.EnvPortScan
}

// JobsIsolatedByCommand names the services whose ports reach them through their
// command line and nowhere else. They are isolated — while wtm plays that
// command. Their reader launching them directly gets the base port, which is
// the whole cost of that route and the one thing the report has to say.
func JobsIsolatedByCommand(params JobsIsolatedByCommandParams) []string {
	exempt := make(map[string]bool, len(params.Exempt))
	for _, name := range params.Exempt {
		exempt[name] = true
	}
	for _, name := range JobsReadingTheirEnv(JobsReadingTheirEnvParams{Config: params.Config, ScansByDir: params.ScansByDir}) {
		exempt[name] = true
	}

	var names []string
	for _, job := range params.Config.Jobs {
		if exempt[job.Name] || !ShouldProbeJob(ShouldProbeJobParams{Kind: job.Kind, Ports: job.Ports, Probe: job.Probe}) {
			continue
		}
		if len(PortVarsMissingFrom(domain.JobCmdFix{Cmd: job.Cmd, Vars: sortedPortNames(job.Ports)})) == 0 {
			names = append(names, job.Name)
		}
	}
	sort.Strings(names)
	return names
}

// PortCommandOnlyLines is the note those jobs get: what they are, and the one
// thing that would isolate them either way.
func PortCommandOnlyLines(jobs []string) []string {
	if len(jobs) == 0 {
		return nil
	}

	width := 0
	for _, job := range jobs {
		width = max(width, len([]rune(job)))
	}

	lines := make([]string, 0, len(jobs)+2)
	for _, job := range jobs {
		lines = append(lines, fmt.Sprintf(domain.PortCommandOnlyLineFmt, pad(job, width), domain.PortCommandOnlyReason))
	}
	return append(lines, "", domain.PortCommandOnlyHint)
}

func CmdFixWidths(fixes []domain.JobCmdFix) (job, vars int) {
	for _, fix := range fixes {
		job = max(job, len([]rune(fix.Job)))
		vars = max(vars, len([]rune(CmdFixVars(fix))))
	}
	return job, vars
}

// CmdFixVars re-reads the command as it stands, so a row that has been amended
// stops naming the variable it was flagged for and the list reads as a
// checklist.
func CmdFixVars(fix domain.JobCmdFix) string {
	missing := PortVarsMissingFrom(fix)
	if len(missing) == 0 {
		return domain.CmdListReferenced
	}
	return strings.Join(missing, domain.CmdListVarSep)
}

func CmdFixLabel(fix domain.JobCmdFix, jobWidth, varsWidth int) string {
	return fmt.Sprintf(domain.CmdListEntryFmt, pad(fix.Job, jobWidth), pad(CmdFixVars(fix), varsWidth), fix.Cmd)
}

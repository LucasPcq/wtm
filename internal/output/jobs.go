package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// ProfileActionResult is a single profile's outcome emitted by `run profile` commands.
type ProfileActionResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`            // "added", "removed"
	Message string `json:"message,omitempty"` // error detail when Status == "error"
}

// PRCheckoutJSON is the payload emitted by `checkout --output json`.
type PRCheckoutJSON struct {
	Number int    `json:"number"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Author string `json:"author"`
	URL    string `json:"url"`
	Draft  bool   `json:"is_draft"`
	// ExistingBranch reports that a local branch of that name already existed and
	// was checked out as-is, instead of being created from origin.
	ExistingBranch bool `json:"existing_branch"`
	// OriginState is the reused branch's divergence from origin, using the same
	// labels as `list` and `tree`. Empty when the branch was created.
	OriginState string `json:"origin_state,omitempty"`
}

// WriteJobResultsJSON writes the JSON array describing each job outcome.
func WriteJobResultsJSON(w io.Writer, results []domain.JobActionResult) error {
	if results == nil {
		results = []domain.JobActionResult{}
	}
	return encodeJSON(w, results)
}

// WriteWorktreeJobResultsJSON writes what a command did across worktrees. The
// shape follows the arity (LUC-198): one worktree answers with the bare array
// of job results, several with one document each — the only way two jobs called
// `web` can be told apart.
func WriteWorktreeJobResultsJSON(w io.Writer, results []domain.WorktreeJobResults) error {
	if len(results) <= 1 {
		var jobs []domain.JobActionResult
		if len(results) == 1 {
			jobs = results[0].Jobs
		}
		return WriteJobResultsJSON(w, jobs)
	}
	documents := make([]domain.WorktreeJobResults, len(results))
	for index, result := range results {
		if result.Jobs == nil {
			result.Jobs = []domain.JobActionResult{}
		}
		documents[index] = result
	}
	return encodeJSON(w, documents)
}

// WriteJobLogsJSON writes the lines `run logs` read back.
func WriteJobLogsJSON(w io.Writer, entries []domain.JobLogEntry) error {
	if entries == nil {
		entries = []domain.JobLogEntry{}
	}
	return encodeJSON(w, entries)
}

// WriteJobResultJSON writes a single job outcome (start/stop single job).
func WriteJobResultJSON(w io.Writer, result domain.JobActionResult) error {
	return encodeJSON(w, result)
}

// WriteProfileResultJSON writes a single profile outcome.
func WriteProfileResultJSON(w io.Writer, result ProfileActionResult) error {
	return encodeJSON(w, result)
}

// WritePRCheckoutJSON writes the payload for `wtm checkout`.
func WritePRCheckoutJSON(w io.Writer, payload PRCheckoutJSON) error {
	return encodeJSON(w, payload)
}

// ImportResult is what run.toml holds after an import. The command replaces the
// file wholesale, so there is nothing added and nothing skipped to report —
// only what now stands.
type ImportResult struct {
	Jobs     []string `json:"jobs"`
	Profiles []string `json:"profiles"`
	EnvPorts int      `json:"env_ports"`
}

// WriteImportResultJSON writes the import result as JSON.
func WriteImportResultJSON(w io.Writer, result ImportResult) error {
	if result.Jobs == nil {
		result.Jobs = []string{}
	}
	if result.Profiles == nil {
		result.Profiles = []string{}
	}
	return encodeJSON(w, result)
}

// WriteImportResultText emits a raw body; the caller's frame owns the outer
// padding.
func WriteImportResultText(w io.Writer, result ImportResult) {
	if len(result.Jobs) == 0 && len(result.Profiles) == 0 {
		Message(w, domain.ImportEmptyMessage)
		return
	}
	Success(w, fmt.Sprintf(domain.ImportJobsFmt, len(result.Jobs), strings.Join(result.Jobs, domain.CmdListVarSep)))
	if len(result.Profiles) > 0 {
		Success(w, fmt.Sprintf(domain.ImportProfilesFmt, len(result.Profiles), strings.Join(result.Profiles, domain.CmdListVarSep)))
	}
	if result.EnvPorts > 0 {
		Success(w, fmt.Sprintf(domain.ImportEnvPortsFmt, result.EnvPorts))
	}
	Blank(w)
	Message(w, domain.ImportEnvHint)
}

// WriteRunConfigJSON writes the JSON payload for `run list`.
func WriteRunConfigJSON(w io.Writer, cfg domain.RunConfig) error {
	if cfg.Jobs == nil {
		cfg.Jobs = []domain.JobConfig{}
	}
	if cfg.Profiles == nil {
		cfg.Profiles = []domain.ProfileConfig{}
	}
	return encodeJSON(w, cfg)
}

// WriteJobsJSON writes the JSON array of jobs for `run job list`.
func WriteJobsJSON(w io.Writer, jobs []domain.JobConfig) error {
	if jobs == nil {
		jobs = []domain.JobConfig{}
	}
	return encodeJSON(w, jobs)
}

// WriteProfilesJSON writes the JSON array of profiles for `run profile list`.
func WriteProfilesJSON(w io.Writer, profiles []domain.ProfileConfig) error {
	if profiles == nil {
		profiles = []domain.ProfileConfig{}
	}
	return encodeJSON(w, profiles)
}

// WriteRunningJobsJSON writes the JSON payload for `run ps`.
func WriteRunningJobsJSON(w io.Writer, jobs []domain.JobInfo) error {
	if jobs == nil {
		jobs = []domain.JobInfo{}
	}
	return encodeJSON(w, jobs)
}

// FormatRunConfig renders jobs + profiles as two aligned text tables.
// jobPortsCell is what a job declares, in the NAME=PORT form the rest of the
// CLI reads and writes, with a mark on the one it publishes a url under.
func jobPortsCell(job domain.JobConfig) string {
	entries := rules.PortEntries(job.Ports)
	if len(entries) == 0 {
		return ""
	}
	cell := strings.Join(entries, " ")
	if job.URL != nil {
		cell += domain.RunListURLMark
	}
	return cell
}

func FormatRunConfig(cfg domain.RunConfig) string {
	var b strings.Builder

	if len(cfg.Profiles) > 0 {
		b.WriteString(Indent)
		b.WriteString(styles.Bold.Render("PROFILES"))
		b.WriteString("\n")
		nameWidth := 0
		for _, p := range cfg.Profiles {
			if len(p.Name) > nameWidth {
				nameWidth = len(p.Name)
			}
		}
		for _, p := range cfg.Profiles {
			tag := ""
			if p.Default {
				tag = styles.Success.Render("default")
			}
			jobs := styles.Muted.Render(strings.Join(p.Jobs, ", "))
			b.WriteString(fmt.Sprintf("%s%-*s  %-9s  %s\n", Indent, nameWidth, p.Name, tag, jobs))
		}
	}

	if len(cfg.Jobs) > 0 {
		if len(cfg.Profiles) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(Indent)
		b.WriteString(styles.Bold.Render("JOBS"))
		b.WriteString("\n")
		nameWidth, kindWidth, cmdWidth := 0, 0, 0
		for _, j := range cfg.Jobs {
			if len(j.Name) > nameWidth {
				nameWidth = len(j.Name)
			}
			if len(string(j.Kind)) > kindWidth {
				kindWidth = len(string(j.Kind))
			}
			if width := len(j.Cmd); width > cmdWidth {
				cmdWidth = width
			}
		}
		for _, j := range cfg.Jobs {
			kind := styles.Muted.Render(fmt.Sprintf("%-*s", kindWidth, string(j.Kind)))
			cmd := styles.Muted.Render(fmt.Sprintf("%-*s", cmdWidth, j.Cmd))
			// The declared ports last, and unaligned: a compose stack declaring
			// seven of them would otherwise push every command off the screen.
			// They are the bases as written, not the resolved ones — each worktree
			// shifts them by its own offset.
			ports := styles.DashboardValue.Render(jobPortsCell(j))
			b.WriteString(strings.TrimRight(fmt.Sprintf("%s%-*s  %s  %s  %s", Indent, nameWidth, j.Name, kind, cmd, ports), " ") + "\n")
		}
	}

	if len(cfg.Profiles) == 0 && len(cfg.Jobs) == 0 {
		b.WriteString(Indent)
		b.WriteString("No jobs or profiles defined in run.toml.\n")
	}

	return b.String()
}

type FormatRunningJobsParams struct {
	Jobs []domain.JobInfo
	Now  time.Time
}

// FormatRunningJobs renders a table of running (or recently running) jobs. It
// returns a raw body with no outer blank lines; the caller's frame owns the
// outer vertical padding.
func FormatRunningJobs(params FormatRunningJobsParams) string {
	if len(params.Jobs) == 0 {
		return Indent + "No jobs running.\n"
	}

	uptimes := make([]string, len(params.Jobs))
	for i, j := range params.Jobs {
		uptimes[i] = rules.JobUptime(rules.JobUptimeParams{Job: j, Now: params.Now})
	}

	nameW, kindW, statusW, pidW, upW := len("NAME"), len("KIND"), len("STATUS"), len("PID"), len("UPTIME")
	for i, j := range params.Jobs {
		if len(j.Name) > nameW {
			nameW = len(j.Name)
		}
		if len(string(j.Kind)) > kindW {
			kindW = len(string(j.Kind))
		}
		if len(string(j.Status)) > statusW {
			statusW = len(string(j.Status))
		}
		pid := strconv.Itoa(j.PID)
		if len(pid) > pidW {
			pidW = len(pid)
		}
		if len(uptimes[i]) > upW {
			upW = len(uptimes[i])
		}
	}

	var b strings.Builder
	// Rendered without its line break: a style given a string ending in one sees
	// two lines and pads the empty second to the width of the first, which lands
	// as a run of spaces in front of the first job.
	header := fmt.Sprintf("%s%-*s  %-*s  %-*s  %-*s  %-*s  %s",
		Indent,
		nameW, "NAME",
		kindW, "KIND",
		statusW, "STATUS",
		pidW, "PID",
		upW, "UPTIME",
		"WORKTREE",
	)
	b.WriteString(styles.Muted.Render(header))
	b.WriteString("\n")

	for i, j := range params.Jobs {
		status := styleJobStatus(j.Status)
		pid := ""
		if j.PID != 0 {
			pid = strconv.Itoa(j.PID)
		}
		line := fmt.Sprintf("%s%-*s  %-*s  %-*s  %-*s  %-*s  %s\n",
			Indent,
			nameW, j.Name,
			kindW, string(j.Kind),
			statusW+ansiOverhead(status), status,
			pidW, pid,
			upW, uptimes[i],
			styles.Muted.Render(j.WorkDir),
		)
		b.WriteString(line)
	}

	return b.String()
}

func styleJobStatus(status domain.JobStatus) string {
	switch status {
	case domain.JobStatusRunning, domain.JobStatusDetached, domain.JobStatusAttached:
		return styles.Success.Render(string(status))
	case domain.JobStatusCrashed:
		return styles.Warning.Render(string(status))
	default:
		return styles.Muted.Render(string(status))
	}
}

// WriteJobURLsJSON writes the payload for `wtm run url --output json`.
func WriteJobURLsJSON(w io.Writer, entries []domain.JobURLEntry) error {
	if entries == nil {
		entries = []domain.JobURLEntry{}
	}
	return encodeJSON(w, entries)
}

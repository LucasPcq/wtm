package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// IsRunInitialized reports whether the run module counts as initialized: its
// run.toml declares at least one job or profile. An absent run.toml loads as an
// empty RunConfig, so this doubles as the "run.toml missing" check. The
// not-initialized guard on run commands is built on this predicate.
func IsRunInitialized(cfg domain.RunConfig) bool {
	return len(cfg.Jobs) > 0 || len(cfg.Profiles) > 0
}

// IsDetached reports whether the job is a service with a stop command,
// meaning the launcher process exits after starting detached work
// (e.g. docker compose up -d).
func IsDetached(job domain.JobConfig) bool {
	return job.Kind == domain.JobKindService && !IsBlankCommand(job.Stop)
}

// IsAlreadyRunning reads the daemon's refusal to start a job that is already
// up. A repeat start asks for a state the job is already in, so a caller
// starting a profile treats it as a job that is running rather than a failure.
func IsAlreadyRunning(message string) bool {
	return strings.Contains(message, domain.JobAlreadyRunningSuffix)
}

type JobUptimeParams struct {
	Job domain.JobInfo
	Now time.Time
}

// JobUptime reads how long a job has been up, and only answers for one that
// still is: on a job that stopped, StartedAt dates a run that is over, and
// letting it keep counting would read as still running. A start in the future
// (a clock stepped between the daemon and the reader) counts as zero rather
// than counting backwards, and a caller that did not say when now is gets no
// answer at all rather than a 0s reading as a job that just started.
func JobUptime(params JobUptimeParams) string {
	if params.Now.IsZero() || !livedUntilNow(params.Job.Status) || params.Job.StartedAt.IsZero() {
		return ""
	}

	elapsed := max(params.Now.Sub(params.Job.StartedAt), 0)
	switch {
	case elapsed < time.Minute:
		return fmt.Sprintf(domain.JobUptimeSecFmt, int(elapsed.Seconds()))
	case elapsed < time.Hour:
		return fmt.Sprintf(domain.JobUptimeMinFmt, int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf(domain.JobUptimeHourFmt, int(elapsed.Hours()), int(elapsed.Minutes())%60)
	default:
		return fmt.Sprintf(domain.JobUptimeDayFmt, int(elapsed.Hours())/24, int(elapsed.Hours())%24)
	}
}

// livedUntilNow is the condition an uptime needs, and it is not quite IsJobUp: a
// reaped job was running right up to the instant it was reaped, so now minus its
// start is its real lifetime — the twelve days being the whole point of the row.
// A crashed or stopped job died at a moment nobody recorded, and counting to now
// would report an age it never reached.
func livedUntilNow(status domain.JobStatus) bool {
	return IsJobUp(status) || status == domain.JobStatusReaped
}

// DefaultProfile returns the profile marked as default, or the first one.
func DefaultProfile(cfg domain.RunConfig) (domain.ProfileConfig, bool) {
	for _, p := range cfg.Profiles {
		if p.Default {
			return p, true
		}
	}
	if len(cfg.Profiles) > 0 {
		return cfg.Profiles[0], true
	}
	return domain.ProfileConfig{}, false
}

// FindProfile returns a profile by name.
func FindProfile(cfg domain.RunConfig, name string) (domain.ProfileConfig, bool) {
	for _, p := range cfg.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return domain.ProfileConfig{}, false
}

// FindJob returns a job by name.
func FindJob(cfg domain.RunConfig, name string) (domain.JobConfig, bool) {
	for _, j := range cfg.Jobs {
		if j.Name == name {
			return j, true
		}
	}
	return domain.JobConfig{}, false
}

// ProfileJobs returns the JobConfig list for a profile, preserving the
// declared order.
func ProfileJobs(cfg domain.RunConfig, profile domain.ProfileConfig) []domain.JobConfig {
	jobs := make([]domain.JobConfig, 0, len(profile.Jobs))
	for _, name := range profile.Jobs {
		if j, ok := FindJob(cfg, name); ok {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// FilterToProfile returns a new RunConfig containing only the named profile
// and the jobs it references. Returns an error if the profile is not found.
func FilterToProfile(cfg domain.RunConfig, name string) (domain.RunConfig, error) {
	p, ok := FindProfile(cfg, name)
	if !ok {
		return domain.RunConfig{}, fmt.Errorf("profile %q not found", name)
	}

	// The config is copied whole before being narrowed: rebuilding it field by
	// field silently dropped every project-wide setting, the [[env_port]] links
	// included.
	out := cfg
	out.Jobs = ProfileJobs(cfg, p)
	out.Profiles = []domain.ProfileConfig{p}

	kept := make(map[string]bool, len(out.Jobs))
	for _, job := range out.Jobs {
		kept[job.Name] = true
	}
	// A link to a job the filter dropped would not survive ValidateRun on the
	// other side of an import.
	links := make([]domain.EnvPortLink, 0, len(cfg.EnvPorts))
	for _, link := range cfg.EnvPorts {
		if kept[link.Job] {
			links = append(links, link)
		}
	}
	out.EnvPorts = links

	return out, nil
}

// FindExistingDefaultProfile returns the name of the profile currently marked
// default, excluding `exclude`. Returns "" if no other default exists.
func FindExistingDefaultProfile(cfg domain.RunConfig, exclude string) string {
	for _, p := range cfg.Profiles {
		if p.Default && p.Name != exclude {
			return p.Name
		}
	}
	return ""
}

// ApplyDefaultOverride returns a copy of cfg where every profile other than
// keepName has Default=false. Pure — input is not mutated. Use after a wizard
// run when the user (re)confirmed a new default to prevent ValidateRun from
// rejecting two defaults.
func ApplyDefaultOverride(cfg domain.RunConfig, keepName string) domain.RunConfig {
	// The copy is of the whole config, never a hand-listed subset: this result is
	// written straight back to run.toml, and rebuilding it field by field silently
	// dropped every setting the function does not care about — the [[env_port]]
	// links included.
	out := cfg
	out.Profiles = make([]domain.ProfileConfig, len(cfg.Profiles))
	copy(out.Profiles, cfg.Profiles)
	for i, p := range out.Profiles {
		if p.Default && p.Name != keepName {
			out.Profiles[i].Default = false
		}
	}
	return out
}

// MergeResult summarizes what happened during a MergeRunConfigs call.
type MergeResult struct {
	Added   []string
	Skipped []string
}

// MergeRunConfigs merges src into dst. Jobs and profiles in src that share a
// name with an existing entry in dst are skipped (names go into Skipped); new
// entries are appended (names go into Added). dst is never mutated.
func MergeRunConfigs(dst, src domain.RunConfig) (domain.RunConfig, MergeResult) {
	// Everything dst settled is kept by copying it whole — the offset block a
	// project chose to keep its ports apart, its addressing, its probe timeout.
	// A re-run of `run init` is additive, and none of it is the detection's to
	// discard.
	out := dst
	out.EnvPorts = make([]domain.EnvPortLink, len(dst.EnvPorts))
	out.Jobs = make([]domain.JobConfig, len(dst.Jobs))
	out.Profiles = make([]domain.ProfileConfig, len(dst.Profiles))
	copy(out.EnvPorts, dst.EnvPorts)
	if out.PortOffsetBlock == 0 {
		out.PortOffsetBlock = src.PortOffsetBlock
	}
	copy(out.Jobs, dst.Jobs)
	copy(out.Profiles, dst.Profiles)

	existingJobs := make(map[string]bool, len(dst.Jobs))
	for _, j := range dst.Jobs {
		existingJobs[j.Name] = true
	}
	existingProfiles := make(map[string]bool, len(dst.Profiles))
	for _, p := range dst.Profiles {
		existingProfiles[p.Name] = true
	}

	var result MergeResult

	for _, j := range src.Jobs {
		if existingJobs[j.Name] {
			result.Skipped = append(result.Skipped, j.Name)
			continue
		}
		out.Jobs = append(out.Jobs, j)
		result.Added = append(result.Added, j.Name)
		existingJobs[j.Name] = true
	}

	for _, p := range src.Profiles {
		if existingProfiles[p.Name] {
			result.Skipped = append(result.Skipped, p.Name)
			continue
		}
		out.Profiles = append(out.Profiles, p)
		result.Added = append(result.Added, p.Name)
		existingProfiles[p.Name] = true
	}

	return out, result
}

// JobsWithoutProfile is what `run up` starts when the config declares no
// profile at all: every declared job, tasks included, in declared order.
// Keeping only the services here silently dropped the migrations a service
// needs and still reported the run complete — the step counter read [1/1] with
// nothing to say a declared job had been skipped (LUC-208). A job the user does
// not want in `run up` belongs outside the profile they start, not filtered out
// of a run that claims to start everything.
func JobsWithoutProfile(cfg domain.RunConfig) []domain.JobConfig {
	jobs := make([]domain.JobConfig, len(cfg.Jobs))
	copy(jobs, cfg.Jobs)
	return jobs
}

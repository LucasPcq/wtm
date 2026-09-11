package rules

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// IsSharedJob says whether a job serves every profile: a compose stack the
// packages sit on, a migration every one of them needs. Starting a single
// package still needs those, so they enter every profile.
//
// A foreground service at the root is not one of them. It is a fan-out — a
// `turbo run dev` starting the packages itself — and adding it to a per-package
// profile starts the whole repository beside the one package that profile
// names, twice over on the same ports. It gets a profile of its own instead.
func IsSharedJob(job domain.JobConfig) bool {
	return IsRootJob(job) && (job.Kind != domain.JobKindService || IsDetached(job))
}

// IsRootJob says the job runs at the repository root rather than in a package.
func IsRootJob(job domain.JobConfig) bool {
	return job.Cwd == "" || job.Cwd == domain.ProfileRootCwd
}

// IsFanOutJob is a root job that starts packages itself.
func IsFanOutJob(job domain.JobConfig) bool {
	return IsRootJob(job) && !IsSharedJob(job)
}

type ProposeProfilesParams struct {
	Config domain.RunConfig
	// Existing is the split already in run.toml. Non-empty wins whole: a
	// composition the user made by hand is an answer, not a starting point to
	// infer over.
	Existing []domain.ProfileConfig
}

// ProposeProfiles suggests a split for the wizard to edit. It decides nothing
// final — the grouping is an intention, and two repos with the same directory
// shape can want opposite groupings.
//
// Every job enters a profile, tasks included: a profile holding only the
// services would have `wtm run up` start them on a database no migration ever
// touched (LUC-208).
func ProposeProfiles(params ProposeProfilesParams) []domain.ProfileConfig {
	if len(params.Existing) > 0 {
		return params.Existing
	}
	if len(params.Config.Jobs) == 0 {
		return nil
	}

	var shared, fanOut []domain.JobConfig
	var dirs []string
	byDir := map[string][]domain.JobConfig{}
	for _, job := range params.Config.Jobs {
		if IsSharedJob(job) {
			shared = append(shared, job)
			continue
		}
		if IsFanOutJob(job) {
			fanOut = append(fanOut, job)
			continue
		}
		if _, seen := byDir[job.Cwd]; !seen {
			dirs = append(dirs, job.Cwd)
		}
		byDir[job.Cwd] = append(byDir[job.Cwd], job)
	}

	// The global profile takes the packages, not the fan-outs that start them:
	// holding both would start every app twice, on the same ports. A repo with
	// no package job keeps its fan-outs — they are all it has.
	globalJobs := params.Config.Jobs
	if len(dirs) > 0 && len(fanOut) > 0 {
		globalJobs = withoutJobs(globalJobs, fanOut)
	}
	global := domain.ProfileConfig{
		Name:    domain.ProfileAllName,
		Jobs:    JobNames(TasksFirst(globalJobs)),
		Default: true,
	}
	// With no package to tell apart, everything is the global profile and a
	// fan-out has nothing to be distinguished from.
	if len(dirs) <= 1 || len(dirs) > domain.ProfileProposalMaxPackages {
		return []domain.ProfileConfig{global}
	}

	sort.Strings(dirs)
	names := ProfileNamesForDirs(dirs)
	profiles := make([]domain.ProfileConfig, 0, len(dirs)+1)
	for _, dir := range dirs {
		combined := append(append([]domain.JobConfig{}, shared...), byDir[dir]...)
		profiles = append(profiles, domain.ProfileConfig{
			Name: names[dir],
			Jobs: JobNames(TasksFirst(combined)),
		})
	}
	return append(append(profiles, fanOutProfiles(shared, fanOut)...), global)
}

func withoutJobs(jobs, drop []domain.JobConfig) []domain.JobConfig {
	dropped := make(map[string]bool, len(drop))
	for _, job := range drop {
		dropped[job.Name] = true
	}
	kept := make([]domain.JobConfig, 0, len(jobs))
	for _, job := range jobs {
		if !dropped[job.Name] {
			kept = append(kept, job)
		}
	}
	return kept
}

// fanOutProfiles gives every root fan-out a profile of its own, on top of what
// the packages share: it is a way to start the repository, so it is a profile,
// not a passenger in everyone else's.
func fanOutProfiles(shared, fanOut []domain.JobConfig) []domain.ProfileConfig {
	profiles := make([]domain.ProfileConfig, 0, len(fanOut))
	for _, job := range fanOut {
		combined := append(append([]domain.JobConfig{}, shared...), job)
		profiles = append(profiles, domain.ProfileConfig{
			Name: job.Name,
			Jobs: JobNames(TasksFirst(combined)),
		})
	}
	return profiles
}

// ProfileNamesForDirs names one profile per package directory, keyed by that
// directory. The base name alone merges apps/app-1/back with apps/app-2/back
// into a single profile carrying both applications' jobs, so a colliding name
// is widened one path segment at a time until it stands apart.
func ProfileNamesForDirs(dirs []string) map[string]string {
	names := make(map[string]string, len(dirs))
	for _, dir := range dirs {
		names[dir] = profileNameForDir(dir, dirs)
	}
	return names
}

func profileNameForDir(dir string, dirs []string) string {
	segments := dirSegments(dir)
	for take := 1; take < len(segments); take++ {
		suffix := segments[len(segments)-take:]
		if uniqueSuffix(suffix, dir, dirs) {
			return strings.Join(suffix, domain.ProfileNameSegmentSep)
		}
	}
	return strings.Join(segments, domain.ProfileNameSegmentSep)
}

func uniqueSuffix(suffix []string, owner string, dirs []string) bool {
	for _, other := range dirs {
		if other == owner {
			continue
		}
		if hasSuffixSegments(dirSegments(other), suffix) {
			return false
		}
	}
	return true
}

func hasSuffixSegments(segments, suffix []string) bool {
	if len(suffix) > len(segments) {
		return false
	}
	for i := range suffix {
		if segments[len(segments)-len(suffix)+i] != suffix[i] {
			return false
		}
	}
	return true
}

func dirSegments(dir string) []string {
	cleaned := filepath.ToSlash(filepath.Clean(dir))
	var segments []string
	for _, segment := range strings.Split(cleaned, "/") {
		if segment != "" && segment != "." {
			segments = append(segments, segment)
		}
	}
	if len(segments) == 0 {
		return []string{domain.ProfileAllName}
	}
	return segments
}

type ApplyInitAnswersParams struct {
	Config   domain.RunConfig
	Ports    []domain.PortEntry
	Profiles []domain.ProfileConfig
	// ProfilesAsked says the profiles step ran, so an empty list withdraws every
	// profile instead of leaving the proposal standing.
	ProfilesAsked bool
	Cmds          []domain.JobCmdFix
	// Runners is the relation the runner step settled, applied before anything
	// else: whether a service binds a port of its own is read against it.
	Runners []domain.JobRunnerChoice
	// Addressing is the mode the step settled, and AddressingAsked says it ran:
	// a run that never asked leaves the file's own value standing.
	Addressing      domain.Addressing
	AddressingAsked bool
	// URLs and URLsAsked are the URLs step's answer, resolved here rather than
	// by the caller: a job only becomes publishable once its port is applied,
	// which is what this function has just done.
	URLs      []string
	URLsAsked bool
	// NewJobs are the jobs this pass just added, which the URLs step reads to
	// tell a job never offered from one whose url was withdrawn.
	NewJobs []string
}

// ApplyInitAnswers folds the wizard's two composition steps back into the
// config about to be written. Neither is inferred: a port the user corrected
// and a split they composed outrank what detection proposed.
func ApplyInitAnswers(params ApplyInitAnswersParams) domain.RunConfig {
	cfg := ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: params.Config, Choices: params.Runners})
	jobs := make([]domain.JobConfig, len(cfg.Jobs))
	copy(jobs, cfg.Jobs)
	cfg.Jobs = jobs

	// A zero base is an answer too: the user was offered the port and declined
	// it. Writing it would declare a port nothing binds.
	for _, entry := range params.Ports {
		for i, job := range cfg.Jobs {
			if job.Name == entry.Job && len(job.Ports) == 0 {
				cfg.Jobs[i].BindsNoPort = entry.BindsNone
			}
		}
		if entry.Base <= 0 || portEntryWouldCollide(cfg, entry) {
			continue
		}
		for i, job := range cfg.Jobs {
			if job.Name != entry.Job {
				continue
			}
			cfg.Jobs[i].Ports = clonePorts(cfg.Jobs[i].Ports)
			cfg.Jobs[i].Ports[entry.Name] = entry.Base
		}
	}

	for _, fix := range params.Cmds {
		for i, job := range cfg.Jobs {
			if job.Name == fix.Job && fix.Cmd != "" {
				cfg.Jobs[i].Cmd = fix.Cmd
			}
		}
	}

	urls := ResolveURLChoices(ResolveURLChoicesParams{
		Config:    cfg,
		Published: params.URLs,
		Asked:     params.URLsAsked,
		NewJobs:   params.NewJobs,
	})
	for _, choice := range urls {
		for i, job := range cfg.Jobs {
			if job.Name != choice.Job {
				continue
			}
			cfg.Jobs[i].URL = publishedURL(job.URL, choice)
		}
	}

	if params.AddressingAsked && params.Addressing != "" {
		cfg.Addressing = params.Addressing
	}

	if params.ProfilesAsked || len(params.Profiles) > 0 {
		cfg.Profiles = params.Profiles
	}
	return cfg
}

// portEntryWouldCollide guards the one place an answer is written back over a
// decision taken after it was proposed: a service lifted out of its compose
// file takes its ports with it, and the step that offered them read a config
// where it had not moved yet. Writing one back would put the same base on two
// jobs — refused at load, at the very end of a wizard, taking every other
// answer down with it.
func portEntryWouldCollide(cfg domain.RunConfig, entry domain.PortEntry) bool {
	candidate := cfg
	candidate.Jobs = make([]domain.JobConfig, len(cfg.Jobs))
	copy(candidate.Jobs, cfg.Jobs)
	for i, job := range candidate.Jobs {
		if job.Name != entry.Job {
			continue
		}
		if base, declared := job.Ports[entry.Name]; declared && base == entry.Base {
			return false
		}
		candidate.Jobs[i].Ports = clonePorts(job.Ports)
		candidate.Jobs[i].Ports[entry.Name] = entry.Base
	}

	for _, collision := range PortCollisions(candidate) {
		if collision.A.Job == entry.Job && collision.A.Name == entry.Name {
			return true
		}
		if collision.B.Job == entry.Job && collision.B.Name == entry.Name {
			return true
		}
	}
	return false
}

// publishedURL answers a job's url choice without ever writing through the
// existing pointer: the config it came from is the caller's, and a host the
// step never offered is the caller's too.
func publishedURL(current *domain.JobURLConfig, choice domain.JobURLChoice) *domain.JobURLConfig {
	if !choice.Publish {
		return nil
	}
	url := domain.JobURLConfig{Port: choice.Port}
	if current != nil {
		url.Host = current.Host
	}
	return &url
}

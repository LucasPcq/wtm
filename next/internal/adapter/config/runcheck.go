package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

var (
	valuePlaceholder = regexp.MustCompile(`\{[a-zA-Z_][a-zA-Z0-9_.]*\}`)
	hostLabel        = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

// writeProblems are what the legacy wtm refuses in a run.toml that LoadRun lets through (its ValidateRun): a job named twice, with no name, a name holding a space, no command or no kind, a task with a stop command, binds_no_port beside ports or on a task; runs and touches naming itself, an unknown job or one twice, and a runner reachable from itself; a profile with no name, a name holding a space, named twice or naming an unknown job, and several defaults; a [job.url] whose port or host is wrong or whose host another job publishes; and an .env link that names no declared port, is written twice, or holds a placeholder with no answer. SaveRun refuses them, so both binaries read every run.toml next writes.
func writeProblems(run project.Run) []string {
	jobs := map[string]project.Job{}
	var problems []string
	for _, job := range run.Jobs {
		if _, twice := jobs[job.Name]; twice {
			problems = append(problems, fmt.Sprintf("job %q is declared twice", job.Name))
		}
		jobs[job.Name] = job
		problems = append(problems, jobProblems(job)...)
	}

	for _, job := range run.Jobs {
		problems = append(problems, references(job, "runs", job.Runs, jobs)...)
		problems = append(problems, references(job, "touches", job.Touches, jobs)...)
	}
	problems = append(problems, runnerCycles(run.Jobs)...)
	problems = append(problems, profileProblems(run.Profiles, jobs)...)
	problems = append(problems, urlProblems(run.Jobs)...)
	problems = append(problems, envPortProblems(run.EnvPorts, jobs)...)
	return append(problems, envValueProblems(run, jobs)...)
}

func jobProblems(job project.Job) []string {
	var problems []string
	switch {
	case job.Name == "":
		problems = append(problems, "a job has no name")
	case strings.ContainsFunc(job.Name, unicode.IsSpace):
		problems = append(problems, fmt.Sprintf("job %q: a name holds no space", job.Name))
	}
	if job.Cmd == "" {
		problems = append(problems, fmt.Sprintf("job %q: cmd is required", job.Name))
	}
	switch {
	case job.Kind == "":
		problems = append(problems, fmt.Sprintf("job %q: kind is required (service or task)", job.Name))
	case job.Kind == project.Task && job.Stop != "":
		problems = append(problems, fmt.Sprintf("job %q: a task declares no stop command", job.Name))
	case job.Kind == project.Task && job.BindsNoPort:
		problems = append(problems, fmt.Sprintf("job %q: binds_no_port says nothing of a task", job.Name))
	}
	if job.BindsNoPort && len(job.Ports) > 0 {
		problems = append(problems, fmt.Sprintf("job %q: binds_no_port contradicts the ports it declares", job.Name))
	}
	return problems
}

// references are the problems of a job's runs or touches: itself, a job run.toml does not declare, or one named twice.
func references(job project.Job, key string, names []string, jobs map[string]project.Job) []string {
	var problems []string
	seen := map[string]bool{}
	for _, name := range names {
		_, declared := jobs[name]
		switch {
		case name == job.Name:
			problems = append(problems, fmt.Sprintf("job %q: %s itself", job.Name, key))
		case !declared:
			problems = append(problems, fmt.Sprintf("job %q: %s unknown job %q", job.Name, key, name))
		case seen[name]:
			problems = append(problems, fmt.Sprintf("job %q: %s %q twice", job.Name, key, name))
		}
		seen[name] = true
	}
	return problems
}

// runnerCycles are the jobs a runner's runs lead back to.
func runnerCycles(jobs []project.Job) []string {
	runs := map[string][]string{}
	for _, job := range jobs {
		runs[job.Name] = job.Runs
	}
	var problems []string
	for _, job := range jobs {
		seen := map[string]bool{}
		queue := slices.DeleteFunc(slices.Clone(job.Runs), func(name string) bool { return name == job.Name })
		for len(queue) > 0 {
			name := queue[0]
			queue = queue[1:]
			if name == job.Name {
				problems = append(problems, fmt.Sprintf("job %q: runs itself through %s", job.Name, strings.Join(job.Runs, ", ")))
				break
			}
			if !seen[name] {
				seen[name] = true
				queue = append(queue, runs[name]...)
			}
		}
	}
	return problems
}

func profileProblems(profiles []project.Profile, jobs map[string]project.Job) []string {
	var problems []string
	seen := map[string]bool{}
	defaults := 0
	for _, profile := range profiles {
		switch {
		case profile.Name == "":
			problems = append(problems, "a profile has no name")
		case strings.ContainsFunc(profile.Name, unicode.IsSpace):
			problems = append(problems, fmt.Sprintf("profile %q: a name holds no space", profile.Name))
		case seen[profile.Name]:
			problems = append(problems, fmt.Sprintf("profile %q is declared twice", profile.Name))
		}
		seen[profile.Name] = true
		for _, name := range profile.Jobs {
			if _, declared := jobs[name]; !declared {
				problems = append(problems, fmt.Sprintf("profile %q references unknown job %q", profile.Name, name))
			}
		}
		if profile.Default {
			defaults++
		}
	}
	if defaults > 1 {
		problems = append(problems, fmt.Sprintf("%d profiles are the default: one at most", defaults))
	}
	return problems
}

func urlProblems(jobs []project.Job) []string {
	var problems []string
	claimed := map[string]string{}
	for _, job := range jobs {
		if job.URL == nil {
			continue
		}
		if _, declared := job.Ports[job.URL.Port]; !declared {
			problems = append(problems, fmt.Sprintf("job %q: url.port names %s, which the job does not declare", job.Name, job.URL.Port))
		}
		host := job.URL.Host
		if host == "" {
			host = worktree.HostLabel(job.Name)
		}
		if !slices.ContainsFunc(strings.Split(host, "."), func(label string) bool { return !hostLabel.MatchString(label) }) {
			if owner, taken := claimed[host]; taken {
				problems = append(problems, fmt.Sprintf("jobs %q and %q both publish host %q — give one an explicit url.host", owner, job.Name, host))
			}
			claimed[host] = job.Name
			continue
		}
		problems = append(problems, fmt.Sprintf("job %q: url.host %q is not a valid hostname", job.Name, host))
	}
	return problems
}

func envPortProblems(links []project.EnvPortLink, jobs map[string]project.Job) []string {
	var problems []string
	seen := map[project.EnvPortLink]bool{}
	for _, link := range links {
		if link.File == "" {
			problems = append(problems, fmt.Sprintf("env_port %s: file is required", link.Key))
		}
		if !envVarName.MatchString(link.Key) {
			problems = append(problems, fmt.Sprintf("env_port in %s: %q is not a valid environment variable name", link.File, link.Key))
		}
		if _, declared := jobs[link.Job].Ports[link.Port]; !declared {
			problems = append(problems, fmt.Sprintf("env_port %s in %s references port %q of job %q, which does not declare it", link.Key, link.File, link.Port, link.Job))
		}
		if seen[link] {
			problems = append(problems, fmt.Sprintf("env_port %s in %s follows port %s twice", link.Key, link.File, link.Port))
		}
		seen[link] = true
	}
	return problems
}

func envValueProblems(run project.Run, jobs map[string]project.Job) []string {
	type key struct{ file, key string }
	ports := map[key]bool{}
	for _, link := range run.EnvPorts {
		ports[key{link.File, link.Key}] = true
	}

	var problems []string
	seen := map[key]bool{}
	for _, link := range run.EnvValues {
		at := key{link.File, link.Key}
		switch {
		case link.File == "":
			problems = append(problems, fmt.Sprintf("env %s: file is required", link.Key))
		case !envVarName.MatchString(link.Key):
			problems = append(problems, fmt.Sprintf("env in %s: %q is not a valid environment variable name", link.File, link.Key))
		case !valuePlaceholder.MatchString(link.Value):
			problems = append(problems, fmt.Sprintf("env %s in %s holds no placeholder: it would write the same value into every worktree", link.Key, link.File))
		case seen[at]:
			problems = append(problems, fmt.Sprintf("env %s in %s is written twice", link.Key, link.File))
		case ports[at]:
			problems = append(problems, fmt.Sprintf("env %s in %s is also an env_port: one table writes a key", link.Key, link.File))
		}
		seen[at] = true

		job, found := jobs[link.Job]
		if !found {
			problems = append(problems, fmt.Sprintf("env %s in %s: no job named %q", link.Key, link.File, link.Job))
			continue
		}
		for _, placeholder := range valuePlaceholder.FindAllString(link.Value, -1) {
			if !answered(placeholder, job) {
				problems = append(problems, fmt.Sprintf("env %s in %s: unknown placeholder %s", link.Key, link.File, placeholder))
			}
		}
	}
	return problems
}

// answered reports whether a placeholder of an [[env]] value has an answer for its job in every worktree.
func answered(placeholder string, job project.Job) bool {
	switch placeholder {
	case "{worktree}", "{ordinal}":
		return true
	case "{namespace}":
		return job.Namespace != nil
	case "{origin}":
		return job.URL != nil
	}
	name, isPort := strings.CutPrefix(strings.Trim(placeholder, "{}"), "port.")
	_, declared := job.Ports[name]
	return isPort && declared
}

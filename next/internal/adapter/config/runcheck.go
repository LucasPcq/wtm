package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

var (
	valuePlaceholder = regexp.MustCompile(`\{[a-zA-Z_][a-zA-Z0-9_.]*\}`)
	hostLabel        = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

// writeProblems are what the legacy wtm refuses in a run.toml that LoadRun lets through: a job named twice, a [job.url] whose port or host is wrong or whose host another job publishes, and an .env link that names no declared port, is written twice, or holds a placeholder with no answer. SaveRun refuses them, so both binaries read every run.toml next writes.
func writeProblems(run project.Run) []string {
	jobs := map[string]project.Job{}
	var problems []string
	for _, job := range run.Jobs {
		if _, twice := jobs[job.Name]; twice {
			problems = append(problems, fmt.Sprintf("job %q is declared twice", job.Name))
		}
		jobs[job.Name] = job
	}

	problems = append(problems, urlProblems(run.Jobs)...)
	problems = append(problems, envPortProblems(run.EnvPorts, jobs)...)
	return append(problems, envValueProblems(run, jobs)...)
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

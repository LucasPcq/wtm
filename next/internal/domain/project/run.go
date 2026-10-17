package project

import (
	"slices"
	"strings"
)

// Run is the project's run.toml: its jobs and profiles, the .env keys it links to them, how jobs are addressed, and how worktrees are isolated from one another.
type Run struct {
	PortOffsetBlock int // the distance between two worktrees' ports, 0 for the default
	Jobs            []Job
	Profiles        []Profile
	EnvPorts        []EnvPortLink
	EnvValues       []EnvValueLink
	Addressing      Addressing
	Isolation       Isolation
}

// JobKind says whether a job keeps running or runs to its end.
type JobKind string

// The kinds of job.
const (
	Service JobKind = "service"
	Task    JobKind = "task"
)

// Scope says whether a job runs once per worktree or once for every worktree of the project.
type Scope string

// The scopes of a job.
const (
	PerWorktree Scope = ""
	Shared      Scope = "shared"
)

// Job is one job of run.toml: how it starts and stops, where, the ports it declares by name, the address it publishes, its scope, and the namespace it creates.
type Job struct {
	Name      string
	Kind      JobKind
	Cmd       string
	Stop      string
	Cwd       string
	Ports     map[string]int
	URL       *URL
	Scope     Scope
	Namespace *Namespace
}

// URL is the address a job publishes: the name of its port that serves it, and the host it answers to (empty for one derived from the job's name).
type URL struct {
	Port string
	Host string
}

// Namespace is something a job creates per worktree (a database, a bucket): its name, the commands that create and remove it, and the env it runs them with.
type Namespace struct {
	Name   string
	Create string
	Remove string
	Env    map[string]string
}

// Profile is a named set of jobs started together; Default marks the one started when none is named.
type Profile struct {
	Name    string
	Jobs    []string
	Default bool
}

// EnvPortLink sets Key of the .env file File to the port named Port of the job Job.
type EnvPortLink struct {
	File string
	Key  string
	Job  string
	Port string
}

// EnvValueLink sets Key of the .env file File to Value, its placeholders filled from the job Job.
type EnvValueLink struct {
	File  string
	Key   string
	Job   string
	Value string
}

// Addressing is how a worktree's jobs are reached: by their ports, or by names through the proxy.
type Addressing string

// The ways jobs are addressed.
const (
	AddressPorts Addressing = "ports"
	AddressNames Addressing = "names"
)

// hasNamespace reports whether the job declares a namespace it creates.
func (job Job) hasNamespace() bool {
	return job.Namespace != nil && job.Namespace.Name != "" && job.Namespace.Create != ""
}

// RunsCompose reports whether the job starts or stops with Docker Compose.
func (job Job) RunsCompose() bool {
	for _, cmd := range []string{job.Cmd, job.Stop} {
		if strings.Contains(cmd, "docker compose") || strings.Contains(cmd, "docker-compose") {
			return true
		}
	}

	return false
}

// Isolation is whether a new worktree's env is rewritten to keep it apart from the others (its ports, its namespaces, its Compose project) or copied as it is.
type Isolation string

// The isolations.
const (
	Isolated Isolation = "isolated"
	Verbatim Isolation = "verbatim"
)

// Isolations returns every isolation, in the order a choice offers them.
func Isolations() []Isolation {
	return []Isolation{Isolated, Verbatim}
}

// Valid reports whether i is one of Isolations.
func (i Isolation) Valid() bool {
	return slices.Contains(Isolations(), i)
}

// Effective returns the isolation that applies: Isolated when i is unset.
func (i Isolation) Effective() Isolation {
	if i == "" {
		return Isolated
	}

	return i
}

// IsolationApplies reports whether run.toml declares anything a worktree isolates: an env link, or a job with ports, a namespace or Compose.
func (c Run) IsolationApplies() bool {
	isolates := func(job Job) bool { return len(job.Ports) > 0 || job.hasNamespace() || job.RunsCompose() }
	return len(c.EnvPorts) > 0 || len(c.EnvValues) > 0 || slices.ContainsFunc(c.Jobs, isolates)
}

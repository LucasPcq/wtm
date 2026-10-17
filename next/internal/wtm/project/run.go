package project

import (
	"slices"
	"strings"
)

type Run struct {
	PortOffsetBlock int
	Jobs            []Job
	Profiles        []Profile
	EnvPorts        []EnvPortLink
	EnvValues       []EnvValueLink
	Addressing      Addressing
	Isolation       Isolation
}

type JobKind string

const (
	Service JobKind = "service"
	Task    JobKind = "task"
)

type Scope string

const (
	PerWorktree Scope = ""
	Shared      Scope = "shared"
)

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

type URL struct {
	Port string
	Host string
}

type Namespace struct {
	Name   string
	Create string
	Remove string
	Env    map[string]string
}

type Profile struct {
	Name    string
	Jobs    []string
	Default bool
}

type EnvPortLink struct {
	File string
	Key  string
	Job  string
	Port string
}

type EnvValueLink struct {
	File  string
	Key   string
	Job   string
	Value string
}

type Addressing string

const (
	AddressPorts Addressing = "ports"
	AddressNames Addressing = "names"
)

func (job Job) HasNamespace() bool {
	return job.Namespace != nil && job.Namespace.Name != "" && job.Namespace.Create != ""
}

func (job Job) RunsCompose() bool {
	for _, cmd := range []string{job.Cmd, job.Stop} {
		if strings.Contains(cmd, "docker compose") || strings.Contains(cmd, "docker-compose") {
			return true
		}
	}
	return false
}

type Isolation string

const (
	Isolated Isolation = "isolated"
	Verbatim Isolation = "verbatim"
)

func Isolations() []Isolation {
	return []Isolation{Isolated, Verbatim}
}

func (i Isolation) Valid() bool {
	return slices.Contains(Isolations(), i)
}

func (i Isolation) Effective() Isolation {
	if i == "" {
		return Isolated
	}
	return i
}

func (c Run) IsolationApplies() bool {
	isolates := func(job Job) bool { return len(job.Ports) > 0 || job.HasNamespace() || job.RunsCompose() }
	return len(c.EnvPorts) > 0 || len(c.EnvValues) > 0 || slices.ContainsFunc(c.Jobs, isolates)
}

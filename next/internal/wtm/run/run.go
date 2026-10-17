// Package run is what a slice reads of a repository's run.toml: its jobs, profiles, env links and isolation.
package run

import "strings"

type Config struct {
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

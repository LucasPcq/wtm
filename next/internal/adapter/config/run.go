package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

const (
	runFileName    = "run.toml"
	probeWorktree  = "probe"
	namespaceProbe = "{worktree}"
)

var namespaceToken = regexp.MustCompile(`\{[a-zA-Z_]+\}`)

type runFile struct {
	Schema           int           `toml:"schema"`
	PortOffsetBlock  int           `toml:"port_offset_block"`
	PortProbeTimeout int           `toml:"port_probe_timeout"`
	Jobs             []jobFile     `toml:"job"`
	Profiles         []profileFile `toml:"profile"`
	EnvPorts         []struct {
		File string `toml:"file"`
		Key  string `toml:"key"`
		Job  string `toml:"job"`
		Port string `toml:"port"`
	} `toml:"env_port"`
	EnvValues []struct {
		File  string `toml:"file"`
		Key   string `toml:"key"`
		Job   string `toml:"job"`
		Value string `toml:"value"`
	} `toml:"env"`
	Addressing  string `toml:"addressing"`
	Concurrency string `toml:"concurrency"`
	Isolation   string `toml:"isolation"`
}

type jobFile struct {
	Name  string         `toml:"name"`
	Kind  string         `toml:"kind"`
	Cmd   string         `toml:"cmd"`
	Stop  string         `toml:"stop"`
	Cwd   string         `toml:"cwd"`
	Ports map[string]int `toml:"ports"`
	URL   *struct {
		Port string `toml:"port"`
		Host string `toml:"host"`
	} `toml:"url"`
	Probe       *bool    `toml:"probe"`
	BindsNoPort bool     `toml:"binds_no_port"`
	Runs        []string `toml:"runs"`
	Touches     []string `toml:"touches"`
	Scope       string   `toml:"scope"`
	Namespace   *struct {
		Name   string            `toml:"name"`
		Create string            `toml:"create"`
		Remove string            `toml:"remove"`
		Env    map[string]string `toml:"env"`
	} `toml:"namespace"`
}

type profileFile struct {
	Name    string   `toml:"name"`
	Jobs    []string `toml:"jobs"`
	Default bool     `toml:"default"`
}

func LoadRun(stateDir string) (project.Run, error) {
	path := filepath.Join(stateDir, runFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return project.Run{}, nil
	}
	if err != nil {
		return project.Run{}, err
	}
	var file runFile
	if err := decodeStrict(strictParams{Source: path, Data: data, Out: &file}); err != nil {
		return project.Run{}, err
	}
	if problems := file.problems(); len(problems) > 0 {
		return project.Run{}, fmt.Errorf("invalid run config: %s", strings.Join(problems, "; "))
	}
	return file.config(), nil
}

func (f runFile) problems() []string {
	problems := f.portProblems()
	if f.Addressing != "" && f.Addressing != string(project.AddressPorts) && f.Addressing != string(project.AddressNames) {
		problems = append(problems, fmt.Sprintf("unknown addressing %q (expected %q or %q)", f.Addressing, project.AddressPorts, project.AddressNames))
	}
	if f.Isolation != "" && !project.Isolation(f.Isolation).Valid() {
		problems = append(problems, fmt.Sprintf("unknown isolation %q (expected %q or %q)", f.Isolation, project.Isolated, project.Verbatim))
	}
	for _, job := range f.Jobs {
		problems = append(problems, job.problems()...)
	}
	return problems
}

func (j jobFile) problems() []string {
	var problems []string
	if j.Kind != "" && j.Kind != string(project.Service) && j.Kind != string(project.Task) {
		problems = append(problems, fmt.Sprintf("job %q: unknown kind %q (expected service or task)", j.Name, j.Kind))
	}
	return append(problems, j.scopeProblems()...)
}

func (j jobFile) scopeProblems() []string {
	switch {
	case j.Scope != string(project.PerWorktree) && j.Scope != string(project.Shared):
		return []string{fmt.Sprintf("job %q: unknown scope %q (expected %q)", j.Name, j.Scope, project.Shared)}
	case j.Namespace == nil:
		return nil
	case j.Scope != string(project.Shared):
		return []string{fmt.Sprintf(`job %q: [job.namespace] only means something on a job with scope = "shared"`, j.Name)}
	case j.Namespace.Name == "" || j.Namespace.Create == "":
		return []string{fmt.Sprintf("job %q: a [job.namespace] block needs both a name and a create command", j.Name)}
	}
	for _, value := range slices.AppendSeq([]string{j.Namespace.Name}, maps.Values(j.Namespace.Env)) {
		expanded := strings.NewReplacer(namespaceProbe, probeWorktree, "{ordinal}", "0").Replace(value)
		if leftover := namespaceToken.FindString(expanded); leftover != "" {
			return []string{fmt.Sprintf("job %q: unknown placeholder in a [job.namespace] value: %s", j.Name, leftover)}
		}
	}
	return nil
}

func (f runFile) config() project.Run {
	config := project.Run{
		PortOffsetBlock: f.PortOffsetBlock,
		Addressing:      project.Addressing(f.Addressing),
		Isolation:       project.Isolation(f.Isolation),
	}
	for _, job := range f.Jobs {
		config.Jobs = append(config.Jobs, job.job())
	}
	for _, profile := range f.Profiles {
		config.Profiles = append(config.Profiles, project.Profile{Name: profile.Name, Jobs: profile.Jobs, Default: profile.Default})
	}
	for _, link := range f.EnvPorts {
		config.EnvPorts = append(config.EnvPorts, project.EnvPortLink{File: link.File, Key: link.Key, Job: link.Job, Port: link.Port})
	}
	for _, link := range f.EnvValues {
		config.EnvValues = append(config.EnvValues, project.EnvValueLink{File: link.File, Key: link.Key, Job: link.Job, Value: link.Value})
	}
	return config
}

func (j jobFile) job() project.Job {
	job := project.Job{Name: j.Name, Kind: project.JobKind(j.Kind), Cmd: j.Cmd, Stop: j.Stop, Cwd: j.Cwd, Ports: j.Ports, Scope: project.Scope(j.Scope)}
	if j.URL != nil {
		job.URL = &project.URL{Port: j.URL.Port, Host: j.URL.Host}
	}
	if j.Namespace != nil {
		job.Namespace = &project.Namespace{Name: j.Namespace.Name, Create: j.Namespace.Create, Remove: j.Namespace.Remove, Env: j.Namespace.Env}
	}
	return job
}

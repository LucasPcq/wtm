package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/run"
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

func LoadRun(stateDir string) (run.Config, error) {
	path := filepath.Join(stateDir, runFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return run.Config{}, nil
	}
	if err != nil {
		return run.Config{}, err
	}
	var file runFile
	if err := decodeStrict(strictParams{Source: path, Data: data, Out: &file}); err != nil {
		return run.Config{}, err
	}
	if problems := file.problems(); len(problems) > 0 {
		return run.Config{}, fmt.Errorf("invalid run config: %s", strings.Join(problems, "; "))
	}
	return file.config(), nil
}

func (f runFile) problems() []string {
	problems := f.portProblems()
	if f.Addressing != "" && f.Addressing != string(run.AddressPorts) && f.Addressing != string(run.AddressNames) {
		problems = append(problems, fmt.Sprintf("unknown addressing %q (expected %q or %q)", f.Addressing, run.AddressPorts, run.AddressNames))
	}
	if f.Isolation != "" && !run.Isolation(f.Isolation).Valid() {
		problems = append(problems, fmt.Sprintf("unknown isolation %q (expected %q or %q)", f.Isolation, run.Isolated, run.Verbatim))
	}
	for _, job := range f.Jobs {
		problems = append(problems, job.problems()...)
	}
	return problems
}

func (j jobFile) problems() []string {
	var problems []string
	if j.Kind != "" && j.Kind != string(run.Service) && j.Kind != string(run.Task) {
		problems = append(problems, fmt.Sprintf("job %q: unknown kind %q (expected service or task)", j.Name, j.Kind))
	}
	return append(problems, j.scopeProblems()...)
}

func (j jobFile) scopeProblems() []string {
	switch {
	case j.Scope != string(run.PerWorktree) && j.Scope != string(run.Shared):
		return []string{fmt.Sprintf("job %q: unknown scope %q (expected %q)", j.Name, j.Scope, run.Shared)}
	case j.Namespace == nil:
		return nil
	case j.Scope != string(run.Shared):
		return []string{fmt.Sprintf(`job %q: [job.namespace] only means something on a job with scope = "shared"`, j.Name)}
	case j.Namespace.Name == "" || j.Namespace.Create == "":
		return []string{fmt.Sprintf("job %q: a [job.namespace] block needs both a name and a create command", j.Name)}
	}
	for _, value := range append([]string{j.Namespace.Name}, mapValues(j.Namespace.Env)...) {
		expanded := strings.NewReplacer(namespaceProbe, probeWorktree, "{ordinal}", "0").Replace(value)
		if leftover := namespaceToken.FindString(expanded); leftover != "" {
			return []string{fmt.Sprintf("job %q: unknown placeholder in a [job.namespace] value: %s", j.Name, leftover)}
		}
	}
	return nil
}

func (f runFile) config() run.Config {
	config := run.Config{
		PortOffsetBlock: f.PortOffsetBlock,
		Addressing:      run.Addressing(f.Addressing),
		Isolation:       run.Isolation(f.Isolation),
	}
	for _, job := range f.Jobs {
		config.Jobs = append(config.Jobs, job.job())
	}
	for _, profile := range f.Profiles {
		config.Profiles = append(config.Profiles, run.Profile{Name: profile.Name, Jobs: profile.Jobs, Default: profile.Default})
	}
	for _, link := range f.EnvPorts {
		config.EnvPorts = append(config.EnvPorts, run.EnvPortLink{File: link.File, Key: link.Key, Job: link.Job, Port: link.Port})
	}
	for _, link := range f.EnvValues {
		config.EnvValues = append(config.EnvValues, run.EnvValueLink{File: link.File, Key: link.Key, Job: link.Job, Value: link.Value})
	}
	return config
}

func (j jobFile) job() run.Job {
	job := run.Job{Name: j.Name, Kind: run.JobKind(j.Kind), Cmd: j.Cmd, Stop: j.Stop, Cwd: j.Cwd, Ports: j.Ports, Scope: run.Scope(j.Scope)}
	if j.URL != nil {
		job.URL = &run.URL{Port: j.URL.Port, Host: j.URL.Host}
	}
	if j.Namespace != nil {
		job.Namespace = &run.Namespace{Name: j.Namespace.Name, Create: j.Namespace.Create, Remove: j.Namespace.Remove, Env: j.Namespace.Env}
	}
	return job
}

func mapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

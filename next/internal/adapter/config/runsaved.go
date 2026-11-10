package config

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

const (
	runSchema         = 1
	runSchemaFileName = "run.schema.json"
)

//go:embed run.schema.json
var runSchemaJSON []byte

// SavedRun returns the project's run.toml as written, with the digest of its bytes; a missing run.toml is an empty run config with no digest, one a newer wtm wrote a *domain.SchemaNewer, and one that does not parse or validate an error naming every problem, as LoadRun says.
func (p Project) SavedRun() (project.SavedRun, error) {
	path := filepath.Join(p.StateDir, runFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return project.SavedRun{}, nil
	}
	if err != nil {
		return project.SavedRun{}, err
	}

	if err := refuseNewer(path, data, runSchema); err != nil {
		return project.SavedRun{}, err
	}
	run, err := parseRun(path, data)
	if err != nil {
		return project.SavedRun{}, err
	}

	return project.SavedRun{Run: run, Digest: digest(data)}, nil
}

// CheckRun refuses, writing nothing, a run config SaveRun would refuse: one LoadRun or the legacy wtm would refuse.
func (p Project) CheckRun(run project.Run) error {
	_, err := p.checked(run)
	return err
}

// checked is run.toml as SaveRun writes run, refused when LoadRun or the legacy wtm would refuse it.
func (p Project) checked(run project.Run) ([]byte, error) {
	data, err := renderRun(run)
	if err != nil {
		return nil, err
	}
	if problems := writeProblems(run); len(problems) > 0 {
		return nil, fmt.Errorf("invalid run config: %s", strings.Join(problems, "; "))
	}
	if _, err := parseRun(filepath.Join(p.StateDir, runFileName), data); err != nil {
		return nil, err
	}
	return data, nil
}

// SaveRun writes run.toml whole from rewrite.Run, laid out as the legacy wtm writes it so both read it, then the JSON schema beside it, and returns what it wrote. A run config LoadRun or the legacy wtm would refuse is never written; a file whose bytes are no longer those rewrite replaces, created or removed since included, is refused with project.ErrRunChanged, and one a newer wtm wrote with the statefile's schema error.
func (p Project) SaveRun(ctx context.Context, rewrite project.RunRewrite) (domain.Written, error) {
	path := filepath.Join(p.StateDir, runFileName)
	data, err := p.checked(rewrite.Run)
	if err != nil {
		return domain.Written{}, err
	}

	written, err := statefile.Update(ctx, statefile.UpdateParams{
		Path: path,
		Mutate: func(current []byte) (statefile.Change, error) {
			if digest(current) != rewrite.Replaces {
				return nil, project.ErrRunChanged
			}
			if err := refuseNewer(path, current, runSchema); err != nil {
				return nil, err
			}
			return statefile.Write{Data: data}, nil
		},
	})
	if err != nil {
		return domain.Written{}, err
	}

	_, err = statefile.Update(ctx, statefile.UpdateParams{
		Path:   filepath.Join(p.StateDir, schemasDirName, runSchemaFileName),
		Mutate: func([]byte) (statefile.Change, error) { return statefile.Write{Data: runSchemaJSON}, nil },
	})
	return written, err
}

// writtenRun is run.toml as it is written, its keys and their order those of the legacy wtm. A pointer leaves out a zero setting the TOML encoder would otherwise write, which the schema refuses.
type writtenRun struct {
	PortOffsetBlock  *int           `toml:"port_offset_block,omitempty"`
	PortProbeTimeout *int           `toml:"port_probe_timeout,omitempty"`
	Addressing       string         `toml:"addressing,omitempty"`
	Concurrency      string         `toml:"concurrency,omitempty"`
	Isolation        string         `toml:"isolation,omitempty"`
	Jobs             []writtenJob   `toml:"job"`
	Profiles         []profileFile  `toml:"profile,omitempty"`
	EnvPorts         []writtenPort  `toml:"env_port,omitempty"`
	EnvValues        []writtenValue `toml:"env,omitempty"`
}

type writtenJob struct {
	Name        string            `toml:"name"`
	Kind        string            `toml:"kind"`
	Cmd         string            `toml:"cmd"`
	Stop        string            `toml:"stop,omitempty"`
	Cwd         string            `toml:"cwd,omitempty"`
	Ports       map[string]int    `toml:"ports,omitempty"`
	URL         *writtenURL       `toml:"url,omitempty"`
	Probe       *bool             `toml:"probe,omitempty"`
	BindsNoPort bool              `toml:"binds_no_port,omitempty"`
	Runs        []string          `toml:"runs,omitempty"`
	Touches     []string          `toml:"touches,omitempty"`
	Scope       string            `toml:"scope,omitempty"`
	Namespace   *writtenNamespace `toml:"namespace,omitempty"`
}

type writtenURL struct {
	Port string `toml:"port"`
	Host string `toml:"host,omitempty"`
}

type writtenNamespace struct {
	Name   string            `toml:"name"`
	Create string            `toml:"create,omitempty"`
	Remove string            `toml:"remove,omitempty"`
	Env    map[string]string `toml:"env,omitempty"`
}

type writtenPort struct {
	File string `toml:"file"`
	Key  string `toml:"key"`
	Job  string `toml:"job"`
	Port string `toml:"port"`
}

type writtenValue struct {
	File  string `toml:"file"`
	Key   string `toml:"key"`
	Job   string `toml:"job"`
	Value string `toml:"value"`
}

// renderRun returns run.toml for run, its first line pointing editors to its schema.
func renderRun(run project.Run) ([]byte, error) {
	file := writtenRun{
		PortOffsetBlock:  nonZero(run.PortOffsetBlock),
		PortProbeTimeout: nonZero(run.PortProbeTimeout),
		Addressing:       string(run.Addressing),
		Concurrency:      string(run.Concurrency),
		Isolation:        string(run.Isolation),
		Jobs:             []writtenJob{},
	}
	for _, job := range run.Jobs {
		file.Jobs = append(file.Jobs, writtenJobOf(job))
	}
	for _, profile := range run.Profiles {
		file.Profiles = append(file.Profiles, profileFile{Name: profile.Name, Jobs: profile.Jobs, Default: profile.Default})
	}
	for _, link := range run.EnvPorts {
		file.EnvPorts = append(file.EnvPorts, writtenPort(link))
	}
	for _, link := range run.EnvValues {
		file.EnvValues = append(file.EnvValues, writtenValue(link))
	}

	var buf bytes.Buffer
	buf.WriteString("#:schema ./schemas/" + runSchemaFileName + "\n\n")
	if err := toml.NewEncoder(&buf).Encode(file); err != nil {
		return nil, fmt.Errorf("encode run config: %w", err)
	}
	return buf.Bytes(), nil
}

func writtenJobOf(job project.Job) writtenJob {
	written := writtenJob{
		Name: job.Name, Kind: string(job.Kind), Cmd: job.Cmd, Stop: job.Stop, Cwd: job.Cwd, Ports: job.Ports,
		BindsNoPort: job.BindsNoPort, Runs: job.Runs, Touches: job.Touches, Scope: string(job.Scope),
	}
	if job.NoProbe {
		written.Probe = new(false)
	}
	if job.URL != nil {
		written.URL = &writtenURL{Port: job.URL.Port, Host: job.URL.Host}
	}
	if job.Namespace != nil {
		written.Namespace = &writtenNamespace{Name: job.Namespace.Name, Create: job.Namespace.Create, Remove: job.Namespace.Remove, Env: job.Namespace.Env}
	}
	return written
}

func nonZero(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

package rules

import (
	"fmt"
	"maps"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// JobPatch is one non-interactive edit of a job: a nil pointer is a field the
// caller left alone, a pointer to "" is that field explicitly cleared. Ports
// merge into the table the job already has — ClearPorts is the only way to
// empty it.
type JobPatch struct {
	Name       *string
	Cmd        *string
	Kind       *string
	Stop       *string
	Cwd        *string
	URLPort    *string
	URLHost    *string
	Ports      map[string]int
	ClearPorts bool
	// Runs replaces the whole list, an empty one dropping the relation: a
	// runner's children are read as a set, never merged one at a time.
	Runs        *[]string
	BindsNoPort *bool
	// Touches replaces the whole list, like Runs.
	Touches *[]string
}

// Empty reports a patch that would change nothing, which is how a runner tells
// "edit these fields" from "open the wizard".
func (p JobPatch) Empty() bool {
	return p.Name == nil && p.Cmd == nil && p.Kind == nil && p.Stop == nil &&
		p.Cwd == nil && p.URLPort == nil && p.URLHost == nil &&
		p.Runs == nil && p.BindsNoPort == nil && p.Touches == nil &&
		len(p.Ports) == 0 && !p.ClearPorts
}

type ApplyJobPatchParams struct {
	Current domain.JobConfig
	Patch   JobPatch
}

// ApplyJobPatch returns Current with the patch applied. Errors name the flag
// that carried the offending value, so the CLI never has to reword them.
func ApplyJobPatch(params ApplyJobPatchParams) (domain.JobConfig, error) {
	job := params.Current
	patch := params.Patch

	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" {
			return domain.JobConfig{}, fmt.Errorf("--%s cannot be empty", domain.FlagName)
		}
		job.Name = name
	}
	if patch.Cmd != nil {
		if strings.TrimSpace(*patch.Cmd) == "" {
			return domain.JobConfig{}, fmt.Errorf("--%s cannot be empty — a job without a command cannot run", domain.FlagCmd)
		}
		job.Cmd = *patch.Cmd
	}
	if patch.Kind != nil {
		kind := domain.JobKind(strings.TrimSpace(*patch.Kind))
		if kind != domain.JobKindService && kind != domain.JobKindTask {
			return domain.JobConfig{}, fmt.Errorf("--%s %q is neither %s nor %s", domain.FlagKind, *patch.Kind, domain.JobKindService, domain.JobKindTask)
		}
		job.Kind = kind
	}
	if patch.Stop != nil {
		job.Stop = *patch.Stop
	}
	if patch.Cwd != nil {
		job.Cwd = *patch.Cwd
	}

	if patch.Runs != nil {
		job.Runs = trimmedNames(*patch.Runs)
	}
	if patch.BindsNoPort != nil {
		job.BindsNoPort = *patch.BindsNoPort
	}
	if patch.Touches != nil {
		job.Touches = trimmedNames(*patch.Touches)
	}

	ports, err := patchedPorts(job.Ports, patch)
	if err != nil {
		return domain.JobConfig{}, err
	}
	job.Ports = ports

	url, err := patchedURL(job.URL, patch)
	if err != nil {
		return domain.JobConfig{}, err
	}
	job.URL = url

	return job, nil
}

// trimmedNames drops the empty entries a shell splits out of `--runs ""`, which
// is how the relation is withdrawn.
func trimmedNames(names []string) []string {
	var out []string
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func patchedPorts(current map[string]int, patch JobPatch) (map[string]int, error) {
	if patch.ClearPorts && len(patch.Ports) > 0 {
		return nil, fmt.Errorf("--%s empties the table, so it cannot be combined with --%s", domain.FlagPortClear, domain.FlagPort)
	}
	if patch.ClearPorts {
		return nil, nil
	}
	if len(patch.Ports) == 0 {
		return current, nil
	}
	merged := make(map[string]int, len(current)+len(patch.Ports))
	maps.Copy(merged, current)
	maps.Copy(merged, patch.Ports)
	return merged, nil
}

func patchedURL(current *domain.JobURLConfig, patch JobPatch) (*domain.JobURLConfig, error) {
	if patch.URLPort == nil && patch.URLHost == nil {
		return current, nil
	}

	var port, host string
	if current != nil {
		port, host = current.Port, current.Host
	}
	if patch.URLPort != nil {
		port = strings.TrimSpace(*patch.URLPort)
		if port == "" {
			host = ""
		}
	}
	if patch.URLHost != nil {
		host = strings.TrimSpace(*patch.URLHost)
	}
	if port == "" && host != "" {
		return nil, fmt.Errorf("--%s names the host but nothing is published — add --%s", domain.FlagURLHost, domain.FlagURLPort)
	}
	return ParseJobURL(strings.TrimSpace(port + " " + host))
}

// RenameJobRefs rewrites everything that names a renamed job — the profiles
// that start it and the env_port links that follow its ports — so the rename
// does not leave a reference to a name the file no longer declares, which
// ValidateRun refuses to save.
func RenameJobRefs(cfg domain.RunConfig, from, to string) domain.RunConfig {
	if from == to {
		return cfg
	}

	out := cfg
	out.Profiles = make([]domain.ProfileConfig, len(cfg.Profiles))
	copy(out.Profiles, cfg.Profiles)
	for i, p := range out.Profiles {
		jobs := make([]string, len(p.Jobs))
		copy(jobs, p.Jobs)
		for j, ref := range jobs {
			if ref == from {
				jobs[j] = to
			}
		}
		out.Profiles[i].Jobs = jobs
	}

	// A job is named by the runners that start it and the tasks that touch it
	// too: renaming it without them leaves a config that refuses to load.
	out.Jobs = make([]domain.JobConfig, len(cfg.Jobs))
	copy(out.Jobs, cfg.Jobs)
	for i, job := range out.Jobs {
		out.Jobs[i].Runs = renamedIn(job.Runs, from, to)
		out.Jobs[i].Touches = renamedIn(job.Touches, from, to)
	}

	if len(cfg.EnvValues) > 0 {
		out.EnvValues = make([]domain.EnvValueLink, len(cfg.EnvValues))
		copy(out.EnvValues, cfg.EnvValues)
		for i, link := range out.EnvValues {
			if link.Job == from {
				out.EnvValues[i].Job = to
			}
		}
	}

	out.EnvPorts = make([]domain.EnvPortLink, len(cfg.EnvPorts))
	copy(out.EnvPorts, cfg.EnvPorts)
	for i, link := range out.EnvPorts {
		if link.Job == from {
			out.EnvPorts[i].Job = to
		}
	}

	return out
}

// renamedIn copies names with one of them renamed, keeping nil as nil.
func renamedIn(names []string, from, to string) []string {
	if len(names) == 0 {
		return names
	}
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = name
		if name == from {
			out[i] = to
		}
	}
	return out
}

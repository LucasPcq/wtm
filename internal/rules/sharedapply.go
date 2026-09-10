package rules

import (
	"fmt"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ApplySharedServicesParams struct {
	Config domain.RunConfig
	// Shared is what the scope step answered. Asked says the question was put at
	// all: an empty list from a run that asked withdraws every sharing, where one
	// that never asked leaves the config exactly as it stands.
	Shared []domain.SharedComposeService
	Asked  bool
	// Scans say what each file declares, which is what names the services that
	// stay in a file's own job once one is lifted out of it.
	Scans map[string]domain.ComposeScan
	// Bindings say which service declares which port variable, so a lifted one
	// takes its ports with it.
	Bindings   map[string][]domain.ComposePortBinding
	ComposeCmd string
}

// ApplySharedServices carries the scope step's answers into a configuration that
// already exists. Building them is not enough: a re-init on a project whose
// compose file already has a job never rebuilds it, so the answer reached
// nothing and the service came back per-worktree on the next run.
//
// It is the write-side counterpart of ServiceScopeChoices, and it goes both
// ways — a service unshared has its lifted job removed and its file's job given
// back the whole stack.
func ApplySharedServices(params ApplySharedServicesParams) domain.RunConfig {
	cfg := params.Config
	for _, file := range sortedScanFiles(params.Scans) {
		cfg = applyFileSharing(applyFileSharingParams{
			Config: cfg, File: file, Params: params,
		})
	}
	return cfg
}

type applyFileSharingParams struct {
	Config domain.RunConfig
	File   string
	Params ApplySharedServicesParams
}

func applyFileSharing(params applyFileSharingParams) domain.RunConfig {
	cfg := params.Config
	fileJob := ComposeJobName(ComposeJobNameParams{Config: cfg, File: params.File})

	wanted := map[string]domain.SharedComposeService{}
	for _, shared := range params.Params.Shared {
		if shared.File == params.File {
			wanted[shared.Service] = shared
		}
	}

	// A run that never put the question withdraws nothing and touches a file
	// that shares nothing at all — but it still normalises what the config
	// already declares shared. Moving a lifted service's ports onto it is not
	// changing an answer, it is making the file agree with the scope it carries;
	// left undone, run.toml refuses to load.
	if !params.Params.Asked && len(wanted) == 0 {
		return cfg
	}

	// Withdrawn first: a service that stops being shared has to give its name
	// back before the file's job is recomputed around what is left.
	for _, service := range params.Params.Scans[params.File].Services {
		if !params.Params.Asked {
			break
		}
		if _, still := wanted[service.Name]; still {
			continue
		}
		if job, found := jobByName(cfg, service.Name); found && IsShared(job) {
			cfg, _ = RemoveJob(cfg, service.Name)
		}
	}

	for _, service := range params.Params.Scans[params.File].Services {
		shared, want := wanted[service.Name]
		if !want {
			continue
		}
		cfg = upsertSharedJob(upsertSharedJobParams{
			Config: cfg, Shared: shared, ComposeCmd: params.Params.ComposeCmd,
		})
	}

	cfg = moveLiftedPorts(moveLiftedPortsParams{
		Config: cfg, File: params.File, Host: fileJob,
		Services: params.Params.Scans[params.File].Services, Shared: wanted,
		Bindings: params.Params.Bindings[params.File],
	})

	return rewriteFileJob(rewriteFileJobParams{
		Config: cfg, File: params.File, Job: fileJob,
		Services: params.Params.Scans[params.File].Services,
		Shared:   wanted, ComposeCmd: params.Params.ComposeCmd,
	})
}

type upsertSharedJobParams struct {
	Config     domain.RunConfig
	Shared     domain.SharedComposeService
	ComposeCmd string
}

func upsertSharedJob(params upsertSharedJobParams) domain.RunConfig {
	cfg := params.Config
	flag := DockerComposeFileFlag(params.Shared.File)

	if index, found := jobIndex(cfg, params.Shared.Service); found {
		jobs := make([]domain.JobConfig, len(cfg.Jobs))
		copy(jobs, cfg.Jobs)
		jobs[index].Scope = domain.JobScopeShared
		// A namespace already written by hand outranks a recipe the wizard offered:
		// the config speaks, detection does not.
		if jobs[index].Namespace == nil {
			jobs[index].Namespace = params.Shared.Namespace
		}
		cfg.Jobs = jobs
		return cfg
	}

	cfg.Jobs = append(cfg.Jobs, domain.JobConfig{
		Name:      params.Shared.Service,
		Kind:      domain.JobKindService,
		Cmd:       fmt.Sprintf("%s %sup -d %s", params.ComposeCmd, flag, params.Shared.Service),
		Stop:      fmt.Sprintf("%s %sstop %s", params.ComposeCmd, flag, params.Shared.Service),
		Cwd:       ".",
		Scope:     domain.JobScopeShared,
		Namespace: params.Shared.Namespace,
	})
	return cfg
}

type JoinSharedProfilesParams struct {
	Config domain.RunConfig
	Shared []domain.SharedComposeService
}

// JoinSharedProfiles puts every lifted job in the profiles that start the job it
// was taken out of. It runs after the profiles are settled, not while the jobs
// are: the wizard re-proposes them from the config on disk, which discarded an
// insertion made any earlier — and a profile that no longer starts the database
// leaves every worktree pointing at one that was never created.
func JoinSharedProfiles(params JoinSharedProfilesParams) domain.RunConfig {
	cfg := params.Config
	for _, shared := range params.Shared {
		cfg = joinProfilesOf(cfg, ComposeJobName(ComposeJobNameParams{Config: cfg, File: shared.File}), shared.Service)
	}
	return cfg
}

// joinProfilesOf puts one lifted job in every profile that starts the job it was
// taken out of, right after it.
func joinProfilesOf(cfg domain.RunConfig, host, name string) domain.RunConfig {
	if host == "" {
		return cfg
	}
	profiles := make([]domain.ProfileConfig, len(cfg.Profiles))
	copy(profiles, cfg.Profiles)

	for i, profile := range profiles {
		position := -1
		for j, job := range profile.Jobs {
			if job == name {
				position = -2
				break
			}
			if job == host {
				position = j
			}
		}
		if position < 0 {
			continue
		}
		jobs := append([]string{}, profile.Jobs...)
		profiles[i].Jobs = append(jobs[:position+1], append([]string{name}, jobs[position+1:]...)...)
	}
	cfg.Profiles = profiles
	return cfg
}

type rewriteFileJobParams struct {
	Config     domain.RunConfig
	File       string
	Job        string
	Services   []domain.ComposeService
	Shared     map[string]domain.SharedComposeService
	ComposeCmd string
}

// rewriteFileJob makes the file's own job start only what stayed in it. Left
// alone it would bring the lifted services up a second time, behind the backs
// of the jobs that now own them.
func rewriteFileJob(params rewriteFileJobParams) domain.RunConfig {
	cfg := params.Config
	index, found := jobIndex(cfg, params.Job)
	if !found {
		return cfg
	}

	var stays []string
	for _, service := range params.Services {
		if _, lifted := params.Shared[service.Name]; !lifted {
			stays = append(stays, service.Name)
		}
	}

	// Nothing left to run: a job that started the file anyway would raise every
	// lifted service again.
	if len(params.Shared) > 0 && len(stays) == 0 {
		next, _ := RemoveJob(cfg, params.Job)
		return next
	}

	jobs := make([]domain.JobConfig, len(cfg.Jobs))
	copy(jobs, cfg.Jobs)
	jobs[index].Cmd = composeUpCmd(composeUpParams{
		ComposeCmd: params.ComposeCmd, File: params.File,
		Services: stays, Lifted: len(params.Shared) > 0,
	})
	cfg.Jobs = jobs
	return cfg
}

func jobIndex(cfg domain.RunConfig, name string) (int, bool) {
	for i, job := range cfg.Jobs {
		if job.Name == name {
			return i, true
		}
	}
	return 0, false
}

func jobByName(cfg domain.RunConfig, name string) (domain.JobConfig, bool) {
	index, found := jobIndex(cfg, name)
	if !found {
		return domain.JobConfig{}, false
	}
	return cfg.Jobs[index], true
}

func sortedScanFiles(scans map[string]domain.ComposeScan) []string {
	files := make([]string, 0, len(scans))
	for file := range scans {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

type moveLiftedPortsParams struct {
	Config   domain.RunConfig
	File     string
	Host     string
	Services []domain.ComposeService
	Shared   map[string]domain.SharedComposeService
	Bindings []domain.ComposePortBinding
}

// moveLiftedPorts hands a lifted service the port variables it actually binds,
// and takes them off the job it came out of. Rewriting the command was not
// enough: the two then declared the same base, which reads as a collision and
// had wtm refuse to load run.toml at all.
//
// The [[env_port]] links naming those variables follow, or they would still
// point at a job that no longer declares them — the other half of the same
// refusal.
func moveLiftedPorts(params moveLiftedPortsParams) domain.RunConfig {
	if len(params.Shared) == 0 || params.Host == "" {
		return params.Config
	}

	owners := map[string]string{}
	for _, binding := range params.Bindings {
		if _, lifted := params.Shared[binding.Service]; lifted && binding.Var != "" {
			owners[binding.Var] = binding.Service
		}
	}
	if len(owners) == 0 {
		return params.Config
	}

	cfg := params.Config
	jobs := make([]domain.JobConfig, len(cfg.Jobs))
	copy(jobs, cfg.Jobs)

	for name, service := range owners {
		host, hostFound := jobIndex(cfg, params.Host)
		target, targetFound := jobIndex(cfg, service)
		if !hostFound || !targetFound {
			continue
		}
		base, declared := jobs[host].Ports[name]
		if !declared {
			continue
		}
		jobs[host].Ports = withoutPort(jobs[host].Ports, name)
		jobs[target].Ports = withPort(jobs[target].Ports, name, base)
	}
	cfg.Jobs = jobs

	return repointEnvPorts(cfg, params.Host, owners)
}

func withoutPort(ports map[string]int, name string) map[string]int {
	out := make(map[string]int, len(ports))
	for key, value := range ports {
		if key != name {
			out[key] = value
		}
	}
	return out
}

func withPort(ports map[string]int, name string, base int) map[string]int {
	out := make(map[string]int, len(ports)+1)
	for key, value := range ports {
		out[key] = value
	}
	out[name] = base
	return out
}

// repointEnvPorts follows a variable to the job that now declares it. A link
// left on the old one names a port that job no longer has, which the loader
// refuses — naming the right job in the message, which is how this was found.
func repointEnvPorts(cfg domain.RunConfig, host string, owners map[string]string) domain.RunConfig {
	links := make([]domain.EnvPortLink, len(cfg.EnvPorts))
	copy(links, cfg.EnvPorts)
	for i, link := range links {
		service, moved := owners[link.Port]
		if moved && link.Job == host {
			links[i].Job = service
		}
	}
	cfg.EnvPorts = links
	return cfg
}

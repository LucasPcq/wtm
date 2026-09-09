package rules

import (
	"fmt"
	"sort"
	"strings"

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
	Scans      map[string]domain.ComposeScan
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
	if !params.Asked {
		return params.Config
	}

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

	// Withdrawn first: a service that stops being shared has to give its name
	// back before the file's job is recomputed around what is left.
	for _, service := range params.Params.Scans[params.File].Services {
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
			Config: cfg, Shared: shared, ComposeCmd: params.Params.ComposeCmd, Profiles: fileJob,
		})
	}

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
	// Profiles names the file's own job, whose profiles the lifted one joins:
	// a service taken out of the stack a profile started must keep starting
	// with it, or the profile silently stops bringing its database up.
	Profiles string
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
	return joinProfilesOf(cfg, params.Profiles, params.Shared.Service)
}

// joinProfilesOf puts the lifted job in every profile that starts the job it was
// taken out of, right after it. A profile that used to bring a stack up must
// keep bringing all of it up.
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
	flag := DockerComposeFileFlag(params.File)
	jobs[index].Cmd = strings.TrimRight(
		fmt.Sprintf("%s %sup -d %s", params.ComposeCmd, flag, strings.Join(namesLifted(params.Shared, stays), " ")), " ")
	cfg.Jobs = jobs
	return cfg
}

// namesLifted is the list a file's job spells out, empty when nothing was taken
// from it — an untouched project keeps the plain `up -d` it always had.
func namesLifted(shared map[string]domain.SharedComposeService, stays []string) []string {
	if len(shared) == 0 {
		return nil
	}
	return stays
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

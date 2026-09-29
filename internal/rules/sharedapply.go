package rules

import (
	"fmt"
	"slices"
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
	Scans map[string]domain.ComposeScan
	// Bindings say which service declares which port variable, so a lifted one
	// takes its ports with it.
	Bindings   map[string][]domain.ComposePortBinding
	ComposeCmd string
}

// SharedServicesOutcome is the config the scope answers produced, and what they
// took out of it that the reader has to hear about.
type SharedServicesOutcome struct {
	Config domain.RunConfig
	// Withdrawn are the jobs removed from run.toml: a lifted service given back
	// to its file, or a file's job left with nothing to run.
	Withdrawn []string
	// Unlinked are the .env keys whose link could follow nothing any more.
	Unlinked []string
	// Renamed are the lines naming a service lifted under another name.
	Renamed []string
}

// ApplySharedServices carries the scope step's answers into a configuration that
// already exists. Building them is not enough: a re-init on a project whose
// compose file already has a job never rebuilds it, so the answer reached
// nothing and the service came back per-worktree on the next run.
//
// It goes both ways. A service no longer shared gives its job back to the file's
// own — rebuilt when every service had been lifted — and every reference to it
// follows there, rather than being deleted with it.
func ApplySharedServices(params ApplySharedServicesParams) SharedServicesOutcome {
	outcome := SharedServicesOutcome{Config: params.Config}
	for _, file := range sortedScanFiles(params.Scans) {
		applied := applyFileSharing(applyFileSharingParams{
			Config: outcome.Config, File: file, Params: params,
		})
		outcome.Config = applied.Config
		outcome.Withdrawn = append(outcome.Withdrawn, applied.Withdrawn...)
		outcome.Unlinked = append(outcome.Unlinked, applied.Unlinked...)
		outcome.Renamed = append(outcome.Renamed, applied.Renamed...)
	}
	return outcome
}

type applyFileSharingParams struct {
	Config domain.RunConfig
	File   string
	Params ApplySharedServicesParams
}

func applyFileSharing(params applyFileSharingParams) SharedServicesOutcome {
	outcome := SharedServicesOutcome{Config: params.Config}
	services := params.Params.Scans[params.File].Services
	fileJob := ComposeJobName(ComposeJobNameParams{Config: outcome.Config, File: params.File})
	composeCmd := params.Params.ComposeCmd
	if job, found := jobByName(outcome.Config, fileJob); found {
		if invoked := composeInvocation(job, params.File); invoked != "" {
			composeCmd = invoked
		}
	}

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
		return outcome
	}

	// Nothing was shared and nothing is: the file's job starts the whole file,
	// however the config spells it, and a re-init has no reason to overrule it.
	untouched := len(wanted) == 0 && !sharesAny(sharesAnyParams{Config: outcome.Config, File: params.File, Services: services})

	withdrawn := withdrawnSharedJobs(withdrawnSharedJobsParams{
		Config: outcome.Config, File: params.File, Services: services,
		Wanted: wanted, Asked: params.Params.Asked,
	})
	// Every service had been lifted, so the file has no job left to take back
	// the ones given up. It is rebuilt first: without it the withdrawn jobs'
	// references had nowhere to go and were deleted with them.
	if len(withdrawn) > 0 && fileJob == "" {
		outcome.Config, fileJob = addFileJob(addFileJobParams{
			Config: outcome.Config, File: params.File, ComposeCmd: composeCmd,
		})
	}
	for _, name := range withdrawn {
		var unlinked []string
		outcome.Config, unlinked = withdrawShared(withdrawSharedParams{Config: outcome.Config, Job: name, Into: fileJob})
		outcome.Withdrawn = append(outcome.Withdrawn, name)
		outcome.Unlinked = append(outcome.Unlinked, unlinked...)
	}

	for _, service := range services {
		shared, want := wanted[service.Name]
		if !want {
			continue
		}
		var renamed string
		outcome.Config, renamed = upsertSharedJob(upsertSharedJobParams{
			Config: outcome.Config, Shared: shared, ComposeCmd: composeCmd,
		})
		if renamed != "" {
			outcome.Renamed = append(outcome.Renamed, renamed)
		}
	}

	outcome.Config = moveLiftedPorts(moveLiftedPortsParams{
		Config: outcome.Config, File: params.File, Host: fileJob,
		Services: services, Shared: wanted,
		Bindings: params.Params.Bindings[params.File],
	})

	if untouched {
		return outcome
	}
	rewritten := rewriteFileJob(rewriteFileJobParams{
		Config: outcome.Config, File: params.File, Job: fileJob,
		Services: services, Shared: wanted, ComposeCmd: composeCmd,
	})
	outcome.Config = rewritten.Config
	outcome.Withdrawn = append(outcome.Withdrawn, rewritten.Withdrawn...)
	outcome.Unlinked = append(outcome.Unlinked, rewritten.Unlinked...)
	return outcome
}

type withdrawnSharedJobsParams struct {
	Config   domain.RunConfig
	File     string
	Services []domain.ComposeService
	Wanted   map[string]domain.SharedComposeService
	Asked    bool
}

func withdrawnSharedJobs(params withdrawnSharedJobsParams) []string {
	if !params.Asked {
		return nil
	}
	var names []string
	for _, service := range params.Services {
		if _, still := params.Wanted[service.Name]; still {
			continue
		}
		name := LiftedJobName(LiftedJobNameParams{Config: params.Config, File: params.File, Service: service.Name})
		if job, found := jobByName(params.Config, name); found && IsShared(job) {
			names = append(names, name)
		}
	}
	return names
}

type LiftedJobNameParams struct {
	Config  domain.RunConfig
	File    string
	Service string
}

// LiftedJobName is the job carrying a compose service lifted out of its file,
// empty when there is none. The name alone is not enough: a script job may
// already answer to the service's, and treating it as the service turned a
// `pnpm db:seed` into a shared service. A job running the file comes first,
// then one lifted under another name, then a shared job declared by hand.
func LiftedJobName(params LiftedJobNameParams) string {
	needle := DockerComposeFileFlag(params.File)
	for _, job := range params.Config.Jobs {
		if job.Name == params.Service && jobRunsComposeFile(job, needle) {
			return job.Name
		}
	}
	for _, job := range params.Config.Jobs {
		fields := strings.Fields(job.Cmd)
		if IsShared(job) && jobRunsComposeFile(job, needle) && len(fields) > 0 && fields[len(fields)-1] == params.Service {
			return job.Name
		}
	}
	for _, job := range params.Config.Jobs {
		if job.Name == params.Service && IsShared(job) {
			return job.Name
		}
	}
	return ""
}

type addFileJobParams struct {
	Config     domain.RunConfig
	File       string
	ComposeCmd string
}

// addFileJob declares the file's own job again, under the name the first init
// gave it. Its command is settled by rewriteFileJob, from what stays.
func addFileJob(params addFileJobParams) (domain.RunConfig, string) {
	cfg := params.Config
	name := freeJobName(cfg, jobNameFromComposeFile(params.File))
	cfg.Jobs = append(slices.Clone(cfg.Jobs), domain.JobConfig{
		Name: name,
		Kind: domain.JobKindService,
		Cmd:  composeUpCmd(composeUpParams{ComposeCmd: params.ComposeCmd, File: params.File}),
		Stop: composeStopCmd(composeStopParams{ComposeCmd: params.ComposeCmd, File: params.File}),
		Cwd:  ".",
	})
	return cfg, name
}

// freeJobName is base, or base-N for the first N no job answers to — the
// suffix BuildDockerJobs gives a second file declaring the same service.
func freeJobName(cfg domain.RunConfig, base string) string {
	name := base
	for n := 2; ; n++ {
		if _, taken := jobIndex(cfg, name); !taken {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, n)
	}
}

type withdrawSharedParams struct {
	Config domain.RunConfig
	Job    string
	Into   string
}

// withdrawShared gives a lifted service back to its file's job: its ports, its
// url when the file's job publishes none, what it touches, and every reference
// naming it. Only a value reading its namespace cannot follow — the file's job
// has none — and that is what it returns, to be said.
func withdrawShared(params withdrawSharedParams) (domain.RunConfig, []string) {
	cfg := params.Config
	from, fromFound := jobIndex(cfg, params.Job)
	into, intoFound := jobIndex(cfg, params.Into)
	if !fromFound || !intoFound {
		return cfg, nil
	}

	jobs := slices.Clone(cfg.Jobs)
	for name, base := range jobs[from].Ports {
		if _, declared := jobs[into].Ports[name]; !declared {
			jobs[into].Ports = withPort(jobs[into].Ports, name, base)
		}
	}
	if jobs[into].URL == nil {
		jobs[into].URL = jobs[from].URL
	}
	for _, touched := range jobs[from].Touches {
		if touched != params.Into && !slices.Contains(jobs[into].Touches, touched) {
			jobs[into].Touches = append(slices.Clone(jobs[into].Touches), touched)
		}
	}
	cfg.Jobs = jobs

	var unlinked []string
	kept := make([]domain.EnvValueLink, 0, len(cfg.EnvValues))
	for _, link := range cfg.EnvValues {
		if link.Job == params.Job && strings.Contains(link.Value, domain.EnvValueTokenNamespace) {
			unlinked = append(unlinked, link.Key)
			continue
		}
		kept = append(kept, link)
	}
	cfg.EnvValues = kept
	if len(kept) == 0 {
		cfg.EnvValues = nil
	}

	cfg = RenameJobRefs(RenameJobRefsParams{Config: cfg, From: params.Job, To: params.Into})
	cfg, _ = RemoveJob(cfg, params.Job)
	return cfg, unlinked
}

// composeInvocation is how a job already calls compose on a file — `docker
// compose`, `docker-compose`, a wrapper — empty when its cmd does not.
func composeInvocation(job domain.JobConfig, file string) string {
	before, _, found := strings.Cut(job.Cmd, " "+DockerComposeFileFlag(file))
	if !found {
		return ""
	}
	return strings.TrimSpace(before)
}

type sharesAnyParams struct {
	Config   domain.RunConfig
	File     string
	Services []domain.ComposeService
}

func sharesAny(params sharesAnyParams) bool {
	for _, service := range params.Services {
		name := LiftedJobName(LiftedJobNameParams{Config: params.Config, File: params.File, Service: service.Name})
		if job, found := jobByName(params.Config, name); found && IsShared(job) {
			return true
		}
	}
	return false
}

type upsertSharedJobParams struct {
	Config     domain.RunConfig
	Shared     domain.SharedComposeService
	ComposeCmd string
}

// upsertSharedJob returns the line to say when the service had to be lifted
// under another name.
func upsertSharedJob(params upsertSharedJobParams) (domain.RunConfig, string) {
	cfg := params.Config
	flag := DockerComposeFileFlag(params.Shared.File)
	lifted := LiftedJobName(LiftedJobNameParams{Config: cfg, File: params.Shared.File, Service: params.Shared.Service})

	if index, found := jobIndex(cfg, lifted); found && lifted != "" {
		jobs := slices.Clone(cfg.Jobs)
		jobs[index].Scope = domain.JobScopeShared
		// A namespace already written by hand outranks a recipe the wizard offered:
		// the config speaks, detection does not.
		if jobs[index].Namespace == nil {
			jobs[index].Namespace = params.Shared.Namespace
		}
		cfg.Jobs = jobs
		return cfg, ""
	}

	name := freeJobName(cfg, params.Shared.Service)
	cfg.Jobs = append(slices.Clone(cfg.Jobs), domain.JobConfig{
		Name:      name,
		Kind:      domain.JobKindService,
		Cmd:       fmt.Sprintf("%s %sup -d %s", params.ComposeCmd, flag, params.Shared.Service),
		Stop:      fmt.Sprintf("%s %sstop %s", params.ComposeCmd, flag, params.Shared.Service),
		Cwd:       ".",
		Scope:     domain.JobScopeShared,
		Namespace: params.Shared.Namespace,
	})
	if name == params.Shared.Service {
		return cfg, ""
	}
	return cfg, fmt.Sprintf(domain.ComposeSharedRenamedFmt, params.Shared.File, params.Shared.Service, name, params.Shared.Service)
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
		lifted := LiftedJobName(LiftedJobNameParams{Config: cfg, File: shared.File, Service: shared.Service})
		if lifted == "" {
			continue
		}
		cfg = joinProfilesOf(cfg, ComposeJobName(ComposeJobNameParams{Config: cfg, File: shared.File}), lifted)
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

// rewriteFileJob makes the file's own job start and stop only what stayed in
// it. Left alone it would bring the lifted services up a second time, behind the
// backs of the jobs that now own them — or, once one is given back, leave it out
// of the stack for good.
func rewriteFileJob(params rewriteFileJobParams) SharedServicesOutcome {
	cfg := params.Config
	index, found := jobIndex(cfg, params.Job)
	if !found {
		return SharedServicesOutcome{Config: cfg}
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
		return retireFileJob(params)
	}

	lifted := len(params.Shared) > 0
	jobs := slices.Clone(cfg.Jobs)
	jobs[index].Cmd = composeUpCmd(composeUpParams{
		ComposeCmd: params.ComposeCmd, File: params.File,
		Services: stays, Lifted: lifted,
	})
	if generatedComposeStop(jobs[index].Stop, params.File) {
		jobs[index].Stop = composeStopCmd(composeStopParams{
			ComposeCmd: params.ComposeCmd, File: params.File,
			Services: stays, Lifted: lifted,
		})
	}
	cfg.Jobs = jobs
	return SharedServicesOutcome{Config: cfg}
}

// generatedComposeStop says the stop command is one wtm wrote, so it may be
// rewritten with the stack. One written by hand is the reader's.
func generatedComposeStop(stop, file string) bool {
	needle := DockerComposeFileFlag(file)
	return strings.HasSuffix(stop, needle+"down --remove-orphans") || strings.Contains(stop, needle+"rm -s -f ")
}

// retireFileJob removes a file's job whose every service was lifted, handing
// what named it to the jobs that took them over. A .env link on one of its own
// ports cannot follow — the port went nowhere — and is returned to be said.
func retireFileJob(params rewriteFileJobParams) SharedServicesOutcome {
	var heirs []string
	for _, service := range params.Services {
		if name := LiftedJobName(LiftedJobNameParams{Config: params.Config, File: params.File, Service: service.Name}); name != "" {
			heirs = append(heirs, name)
		}
	}
	cfg := redirectJobRefs(redirectJobRefsParams{Config: params.Config, From: params.Job, To: heirs})
	cfg, effect := RemoveJob(cfg, params.Job)
	return SharedServicesOutcome{
		Config:    cfg,
		Withdrawn: []string{params.Job},
		Unlinked:  append(effect.EnvPorts, effect.EnvValues...),
	}
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
			owners[binding.Var] = LiftedJobName(LiftedJobNameParams{Config: params.Config, File: params.File, Service: binding.Service})
		}
	}
	if len(owners) == 0 {
		return params.Config
	}

	cfg := params.Config
	jobs := make([]domain.JobConfig, len(cfg.Jobs))
	copy(jobs, cfg.Jobs)

	for name, owner := range owners {
		host, hostFound := jobIndex(cfg, params.Host)
		target, targetFound := jobIndex(cfg, owner)
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

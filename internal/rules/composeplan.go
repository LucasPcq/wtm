package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ComposePortPlan is what a run of the detection decided, before anything is
// written: the declarations to add, the mappings to rewrite, and what it is
// leaving alone. Withheld is the reason the feature is honest — a port wtm
// cannot actually isolate is reported, never declared.
type ComposePortPlan struct {
	PortsByFile map[string]map[string]int
	// Declared keeps the bindings behind PortsByFile: a conflict only visible
	// once the files are grouped by job has to name them.
	Declared   map[string][]domain.ComposePortBinding
	Patches    map[string][]domain.ComposePortBinding
	Withheld   []domain.ComposePortBinding
	Unreadable []domain.ComposeScan
}

type PlanComposePortsParams struct {
	Scans map[string]domain.ComposeScan
	Files []string
	// Patch says the rewrite was authorized — by the wizard step or by the flag.
	// Without it a frozen mapping is withheld rather than declared: declaring a
	// port Docker will not honour would announce an isolation that does not exist.
	Patch bool
}

// PlanComposePorts writes nothing and reads no disk.
func PlanComposePorts(params PlanComposePortsParams) ComposePortPlan {
	plan := ComposePortPlan{
		PortsByFile: map[string]map[string]int{},
		Declared:    map[string][]domain.ComposePortBinding{},
		Patches:     map[string][]domain.ComposePortBinding{},
	}

	for _, file := range params.Files {
		scan, found := params.Scans[file]
		if !found {
			continue
		}
		if scan.Err != "" {
			plan.Unreadable = append(plan.Unreadable, scan)
			continue
		}

		shared := sharedVarBases(scan.Bindings, params.Patch)

		for _, binding := range scan.Bindings {
			switch {
			case !declarable(binding, params.Patch):
				plan.Withheld = append(plan.Withheld, binding)
			case shared[binding.Var]:
				binding.Reason = fmt.Sprintf(domain.ComposePortReasonSharedVar, binding.Var)
				plan.Withheld = append(plan.Withheld, binding)
			default:
				declareComposePort(declareComposePortParams{Ports: plan.PortsByFile, File: file, Binding: binding})
				plan.Declared[file] = append(plan.Declared[file], binding)
				if binding.Status == domain.ComposePortFrozen {
					plan.Patches[file] = append(plan.Patches[file], binding)
				}
			}
		}
	}

	return plan
}

func declarable(binding domain.ComposePortBinding, patch bool) bool {
	return binding.Status == domain.ComposePortTemplated ||
		(binding.Status == domain.ComposePortFrozen && patch)
}

// sharedVarBases names the variables a file declares twice with two different
// bases. wtm injects one value per variable, so declaring either would move the
// other service's binding onto it — the two cannot both be honoured and there
// is nothing to arbitrate on, so neither is declared.
func sharedVarBases(bindings []domain.ComposePortBinding, patch bool) map[string]bool {
	seen := map[string]int{}
	shared := map[string]bool{}
	for _, b := range bindings {
		if !declarable(b, patch) {
			continue
		}
		if base, found := seen[b.Var]; found && base != b.Base {
			shared[b.Var] = true
			continue
		}
		seen[b.Var] = b.Base
	}
	return shared
}

// declareComposePort keeps the first declaration of a variable; a second one
// with the same base is the same declaration, and a second one with a
// different base never reaches here.
type declareComposePortParams struct {
	Ports   map[string]map[string]int
	File    string
	Binding domain.ComposePortBinding
}

func declareComposePort(params declareComposePortParams) {
	if params.Ports[params.File] == nil {
		params.Ports[params.File] = map[string]int{}
	}
	if _, exists := params.Ports[params.File][params.Binding.Var]; !exists {
		params.Ports[params.File][params.Binding.Var] = params.Binding.Base
	}
}

type BackfillDockerPortsParams struct {
	Config      domain.RunConfig
	PortsByFile map[string]map[string]int
	// Declared keeps the bindings behind PortsByFile, which is what says
	// *which service* declares each variable. It is only needed once a service
	// has been lifted out of its file: the lifted job and the file's own job
	// both carry the same `-f <file>` flag, so without it each would receive
	// every port of the file and the two would collide on all of them.
	Declared map[string][]domain.ComposePortBinding
	// Shared are the services lifted out, so a port goes to the job that
	// actually binds it.
	Shared []domain.SharedComposeService
}

type BackfillDockerPortsResult struct {
	Config domain.RunConfig
	// Added is what this call actually wrote, job by job. A variable the job
	// already declared is never in it — and never overwritten.
	Added map[string]map[string]int
}

// BackfillDockerPorts gives each compose-backed job the ports detected in the
// file it runs, matched on the same "-f <file> " fragment BuildDockerJobs
// emits. It is additive by construction: a job that already declares a variable
// keeps the value it declares, which is what makes re-running `run init` safe
// on a project configured before ports existed.
func BackfillDockerPorts(params BackfillDockerPortsParams) BackfillDockerPortsResult {
	result := BackfillDockerPortsResult{
		Config: params.Config,
		Added:  map[string]map[string]int{},
	}
	result.Config.Jobs = make([]domain.JobConfig, len(params.Config.Jobs))
	copy(result.Config.Jobs, params.Config.Jobs)

	for _, file := range SortedComposeFiles(params.PortsByFile) {
		ports := params.PortsByFile[file]
		needle := DockerComposeFileFlag(file)

		owners := portOwners(portOwnersParams{Params: params, File: file})

		for i, job := range result.Config.Jobs {
			if !jobRunsComposeFile(job, needle) {
				continue
			}
			for _, name := range sortedPortNames(ports) {
				if !portBelongsTo(portBelongsParams{Owners: owners, Var: name, Job: job}) {
					continue
				}
				if _, declared := result.Config.Jobs[i].Ports[name]; declared {
					continue
				}
				// Cloned on first write: the slice copy above shares every job's
				// Ports map with the config the caller passed in.
				result.Config.Jobs[i].Ports = clonePorts(result.Config.Jobs[i].Ports)
				result.Config.Jobs[i].Ports[name] = ports[name]

				if result.Added[job.Name] == nil {
					result.Added[job.Name] = map[string]int{}
				}
				result.Added[job.Name][name] = ports[name]
			}
		}
	}

	return result
}

func jobRunsComposeFile(job domain.JobConfig, needle string) bool {
	return strings.Contains(job.Cmd, needle) || strings.Contains(job.Stop, needle)
}

type PruneCollidingPortsParams struct {
	Config domain.RunConfig
	// Detected names the declarations wtm just added, job by job. Only these are
	// ever dropped: a base the user wrote by hand outranks one wtm inferred.
	Detected map[string]map[string]int
}

// DroppedPort pairs a withdrawn declaration with the one it clashed with.
type DroppedPort struct {
	Port      PortDeclaration
	Against   PortDeclaration
	Worktrees int
}

type PruneCollidingPortsResult struct {
	Config  domain.RunConfig
	Dropped []DroppedPort
}

// PruneCollidingPorts withdraws the detected declarations that would make the
// config unloadable. Two bases whose gap is a multiple of the offset block bind
// the same port some worktrees apart, and ValidateRunPorts refuses the file for
// it — so the check runs here, before writing, where the port can still be
// named and explained instead of surfacing later as a rejected run.toml.
func PruneCollidingPorts(params PruneCollidingPortsParams) PruneCollidingPortsResult {
	result := PruneCollidingPortsResult{Config: params.Config}

	detected := func(d PortDeclaration) bool {
		_, ok := params.Detected[d.Job][d.Name]
		return ok
	}
	dropped := map[string]map[string]bool{}
	isDropped := func(d PortDeclaration) bool { return dropped[d.Job][d.Name] }
	drop := func(d, against PortDeclaration, worktrees int) {
		if dropped[d.Job] == nil {
			dropped[d.Job] = map[string]bool{}
		}
		dropped[d.Job][d.Name] = true
		result.Dropped = append(result.Dropped, DroppedPort{Port: d, Against: against, Worktrees: worktrees})
	}

	for _, c := range PortCollisions(params.Config) {
		if isDropped(c.A) || isDropped(c.B) {
			continue
		}
		switch {
		case detected(c.A) && detected(c.B):
			drop(c.A, c.B, c.Worktrees)
			drop(c.B, c.A, c.Worktrees)
		case detected(c.A):
			drop(c.A, c.B, c.Worktrees)
		case detected(c.B):
			drop(c.B, c.A, c.Worktrees)
		}
	}

	if len(result.Dropped) == 0 {
		return result
	}

	result.Config.Jobs = make([]domain.JobConfig, len(params.Config.Jobs))
	copy(result.Config.Jobs, params.Config.Jobs)
	for i, job := range result.Config.Jobs {
		if len(dropped[job.Name]) == 0 {
			continue
		}
		kept := map[string]int{}
		for name, base := range job.Ports {
			if !dropped[job.Name][name] {
				kept[name] = base
			}
		}
		if len(kept) == 0 {
			kept = nil
		}
		result.Config.Jobs[i].Ports = kept
	}

	return result
}

func clonePorts(ports map[string]int) map[string]int {
	clone := make(map[string]int, len(ports)+1)
	for name, base := range ports {
		clone[name] = base
	}
	return clone
}

type RemoveDroppedPortsParams struct {
	Added   map[string]map[string]int
	Dropped []DroppedPort
}

// RemoveDroppedPorts keeps the recap from announcing a port that is not in the
// file it just wrote.
func RemoveDroppedPorts(params RemoveDroppedPortsParams) map[string]map[string]int {
	if len(params.Dropped) == 0 {
		return params.Added
	}

	out := make(map[string]map[string]int, len(params.Added))
	for job, ports := range params.Added {
		kept := map[string]int{}
		for name, base := range ports {
			kept[name] = base
		}
		for _, d := range params.Dropped {
			if d.Port.Job == job {
				delete(kept, d.Port.Name)
			}
		}
		if len(kept) > 0 {
			out[job] = kept
		}
	}
	return out
}

type portOwnersParams struct {
	Params BackfillDockerPortsParams
	File   string
}

// portOwners names, for each variable of one file, the lifted job that owns it —
// empty meaning the file's own job keeps it. Nil when the file lifted nothing,
// which is the case every project had before sharing existed and which must
// keep behaving exactly as it did.
func portOwners(params portOwnersParams) map[string]string {
	lifted := map[string]bool{}
	for _, shared := range params.Params.Shared {
		if shared.File == params.File {
			lifted[shared.Service] = true
		}
	}
	if len(lifted) == 0 {
		return nil
	}

	owners := map[string]string{}
	for _, binding := range params.Params.Declared[params.File] {
		if !lifted[binding.Service] {
			continue
		}
		owners[binding.Var] = binding.Service
	}
	return owners
}

type portBelongsParams struct {
	Owners map[string]string
	Var    string
	Job    domain.JobConfig
}

// portBelongsTo keeps a lifted service's ports on its own job and the rest on
// the file's. Without it both jobs declared every port of the file at the same
// base, which reads as a collision on every one of them and has the whole file
// pruned of its ports.
func portBelongsTo(params portBelongsParams) bool {
	if params.Owners == nil {
		return true
	}
	owner, lifted := params.Owners[params.Var]
	if !lifted {
		return params.Job.Scope != domain.JobScopeShared
	}
	return params.Job.Name == owner
}

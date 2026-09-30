package rules

import (
	"fmt"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RunnerChildren are the jobs a runner starts, transitively, in declaration
// order and without the runner itself. A name matching no job is skipped: a
// stale reference must not invent a child, which would make its runner look
// like a service whose ports are held elsewhere. The `seen` set is what makes
// the walk terminate — a cycle reaches this before anything validates it.
func RunnerChildren(cfg domain.RunConfig, runner string) []string {
	byName := jobsByName(cfg)
	if _, declared := byName[runner]; !declared {
		return nil
	}

	seen := map[string]bool{runner: true}
	var children []string
	queue := append([]string{}, byName[runner].Runs...)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		seen[name] = true
		child, declared := byName[name]
		if !declared {
			continue
		}
		children = append(children, name)
		queue = append(queue, child.Runs...)
	}
	return children
}

// RunnersOf names the jobs that start this one, transitively. It is the other
// direction of the same relation, which is what a start has to check: the
// question is not what I run, but whether something already running holds me.
func RunnersOf(cfg domain.RunConfig, job string) []string {
	var runners []string
	for _, candidate := range cfg.Jobs {
		if candidate.Name == job || len(candidate.Runs) == 0 {
			continue
		}
		for _, child := range RunnerChildren(cfg, candidate.Name) {
			if child == job {
				runners = append(runners, candidate.Name)
				break
			}
		}
	}
	sort.Strings(runners)
	return runners
}

// EffectiveJobPorts are the ports a job's process should be given: its own,
// plus those of the jobs it runs. A name two children declare at two different
// bases has no answer, so it is left unresolved rather than guessed — the same
// rule the lifecycle hooks follow. Two children agreeing on a base is not an
// ambiguity: a port they both dial resolves to the one value they both name.
func EffectiveJobPorts(cfg domain.RunConfig, job domain.JobConfig) map[string]int {
	children := RunnerChildren(cfg, job.Name)
	if len(children) == 0 {
		return job.Ports
	}

	byName := jobsByName(cfg)
	bases := map[string]int{}
	ambiguous := map[string]bool{}
	for _, child := range children {
		for name, base := range byName[child].Ports {
			if seen, found := bases[name]; found && seen != base {
				ambiguous[name] = true
				continue
			}
			bases[name] = base
		}
	}

	ports := map[string]int{}
	for name, base := range bases {
		if ambiguous[name] {
			continue
		}
		ports[name] = base
	}
	// The runner's own declaration outranks anything it inherits: it is the one
	// its reader wrote about this job.
	for name, base := range job.Ports {
		ports[name] = base
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

// JobBindsNothing says a job listens on nothing, whether because its reader
// declared it or because it runs jobs that hold the ports instead.
func JobBindsNothing(cfg domain.RunConfig, job domain.JobConfig) bool {
	return job.BindsNoPort || (len(job.Ports) == 0 && len(RunnerChildren(cfg, job.Name)) > 0)
}

type StartConflictsParams struct {
	Config domain.RunConfig
	// Starting are the jobs about to run together, in one gesture or already up.
	Starting []string
}

// StartConflicts pairs each job with the runner that also starts it. Running a
// runner and one of its children at once is the same process twice on the same
// port — a collision wtm can see coming and nothing else can.
func StartConflicts(params StartConflictsParams) []domain.JobConflict {
	starting := make(map[string]bool, len(params.Starting))
	for _, name := range params.Starting {
		starting[name] = true
	}

	seen := map[domain.JobConflict]bool{}
	var conflicts []domain.JobConflict
	for _, name := range params.Starting {
		for _, runner := range RunnersOf(params.Config, name) {
			conflict := domain.JobConflict{Job: name, Runner: runner}
			if !starting[runner] || seen[conflict] {
				continue
			}
			seen[conflict] = true
			conflicts = append(conflicts, conflict)
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Runner != conflicts[j].Runner {
			return conflicts[i].Runner < conflicts[j].Runner
		}
		return conflicts[i].Job < conflicts[j].Job
	})
	return conflicts
}

func jobsByName(cfg domain.RunConfig) map[string]domain.JobConfig {
	byName := make(map[string]domain.JobConfig, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		byName[job.Name] = job
	}
	return byName
}

// JobsWithEffectivePorts is what a surface hands to the daemon: each job
// carrying the ports its process should see, its runner's children included.
// The config on disk is untouched — the inheritance is resolved at the moment
// of starting, never written back.
func JobsWithEffectivePorts(cfg domain.RunConfig, jobs []domain.JobConfig) []domain.JobConfig {
	out := make([]domain.JobConfig, len(jobs))
	copy(out, jobs)
	for i, job := range out {
		out[i].Ports = EffectiveJobPorts(cfg, job)
	}
	return out
}

func JobConflictLines(conflicts []domain.JobConflict) []string {
	if len(conflicts) == 0 {
		return nil
	}

	width := 0
	for _, conflict := range conflicts {
		width = max(width, len([]rune(conflict.Job)))
	}

	lines := make([]string, 0, len(conflicts)+2)
	for _, conflict := range conflicts {
		lines = append(lines, fmt.Sprintf(domain.JobConflictLineFmt, pad(conflict.Job, width), conflict.Runner))
	}
	return append(lines, "", domain.JobConflictHint)
}

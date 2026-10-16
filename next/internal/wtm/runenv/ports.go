package runenv

import (
	"sort"

	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

const (
	defaultOffsetBlock = 10
	collisionHorizon   = 20
)

func offsetBlock(cfg run.Config) int {
	if cfg.PortOffsetBlock <= 0 {
		return defaultOffsetBlock
	}
	return cfg.PortOffsetBlock
}

type portRef struct {
	Job  string
	Name string
}

func portBases(cfg run.Config) map[portRef]int {
	bases := map[portRef]int{}
	for _, job := range cfg.Jobs {
		for name, base := range job.Ports {
			bases[portRef{Job: job.Name, Name: name}] = base
		}
	}
	return bases
}

func portBaseFor(bases map[portRef]int, link run.EnvPortLink) (int, bool) {
	if link.Job == "" {
		return 0, false
	}
	base, found := bases[portRef{Job: link.Job, Name: link.Port}]
	return base, found
}

type resolvedPortParams struct {
	Base   int
	Offset int
	Shared bool
}

func resolvedPort(params resolvedPortParams) int {
	if params.Shared {
		return params.Base
	}
	return params.Base + params.Offset
}

type lifecyclePortsParams struct {
	Run    run.Config
	Offset int
}

func lifecyclePorts(params lifecyclePortsParams) map[string]int {
	declaring := map[string]int{}
	for _, job := range params.Run.Jobs {
		for name := range job.Ports {
			declaring[name]++
		}
	}
	ports := map[string]int{}
	for _, job := range params.Run.Jobs {
		for _, name := range sortedPortNames(job.Ports) {
			if declaring[name] > 1 {
				continue
			}
			ports[name] = resolvedPort(resolvedPortParams{Base: job.Ports[name], Offset: params.Offset, Shared: job.Scope == run.Shared})
		}
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

func sharedJobs(cfg run.Config) map[string]bool {
	shared := map[string]bool{}
	for _, job := range cfg.Jobs {
		if job.Scope == run.Shared {
			shared[job.Name] = true
		}
	}
	return shared
}

func sortedPortNames(ports map[string]int) []string {
	names := make([]string, 0, len(ports))
	for name := range ports {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

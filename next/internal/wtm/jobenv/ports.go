package jobenv

import "github.com/LucasPcq/wtm/next/internal/wtm/run"

const defaultOffsetBlock = 10

func OffsetBlock(cfg run.Config) int {
	if cfg.PortOffsetBlock <= 0 {
		return defaultOffsetBlock
	}
	return cfg.PortOffsetBlock
}

type ResolvedPortParams struct {
	Base   int
	Offset int
	Shared bool
}

func ResolvedPort(params ResolvedPortParams) int {
	if params.Shared {
		return params.Base
	}
	return params.Base + params.Offset
}

func SharedJobs(cfg run.Config) map[string]bool {
	shared := map[string]bool{}
	for _, job := range cfg.Jobs {
		if job.Scope == run.Shared {
			shared[job.Name] = true
		}
	}
	return shared
}

func (p Params) ports() map[string]int {
	declaring := map[string]int{}
	for _, job := range p.Run.Jobs {
		for name := range job.Ports {
			declaring[name]++
		}
	}
	ports := map[string]int{}
	for _, job := range p.Run.Jobs {
		for name, base := range job.Ports {
			if declaring[name] == 1 {
				ports[name] = ResolvedPort(ResolvedPortParams{Base: base, Offset: p.Offset(), Shared: job.Scope == run.Shared})
			}
		}
	}
	return ports
}

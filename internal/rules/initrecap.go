package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RecapJobLines is one line per job: its name, then the ports it will carry.
// The recap exists so the run can be judged before it is written, and a count
// ("3 ports") cannot be judged — the numbers can.
func RecapJobLines(cfg domain.RunConfig) []string {
	width := 0
	for _, job := range cfg.Jobs {
		width = max(width, len([]rune(job.Name)))
	}

	lines := make([]string, 0, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		line := fmt.Sprintf(domain.RecapJobLineFmt, pad(job.Name, width), recapPorts(cfg, job))
		if job.URL != nil {
			line += domain.RecapURLSuffix
		}
		lines = append(lines, line)
	}
	return lines
}

func recapPorts(cfg domain.RunConfig, job domain.JobConfig) string {
	if job.Kind != domain.JobKindService {
		return domain.RecapTask
	}
	if len(job.Ports) == 0 {
		return recapPortless(cfg, job)
	}

	parts := make([]string, 0, len(job.Ports))
	for _, name := range sortedPortNames(job.Ports) {
		parts = append(parts, fmt.Sprintf(domain.RecapPortFmt, name, job.Ports[name]))
	}
	return strings.Join(parts, domain.RecapPortSep)
}

// recapPortless says why a service declares no port. Only a service that has no
// reason is warned about — the others answered the question, and repeating the
// warning at them is how a recap ends up frightening its reader over a job that
// is exactly as it should be.
func recapPortless(cfg domain.RunConfig, job domain.JobConfig) string {
	if children := RunnerChildren(cfg, job.Name); len(children) > 0 {
		return fmt.Sprintf(domain.RecapRunsFmt, strings.Join(children, domain.RecapPortSep))
	}
	if job.BindsNoPort {
		return domain.RecapBindsNoPort
	}
	return domain.RecapNoPort
}

// RecapProfileLines is one line per profile: what `run up <name>` will start.
func RecapProfileLines(cfg domain.RunConfig) []string {
	width := 0
	for _, profile := range cfg.Profiles {
		width = max(width, len([]rune(profile.Name)))
	}

	lines := make([]string, 0, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		line := fmt.Sprintf(domain.RecapJobLineFmt, pad(profile.Name, width),
			strings.Join(profile.Jobs, domain.ProfileListJobSep))
		if profile.Default {
			line += domain.RecapDefaultSuffix
		}
		lines = append(lines, line)
	}
	return lines
}

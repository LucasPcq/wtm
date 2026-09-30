package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ComposeStopFix is a job whose stop tears down a compose project a shared
// service lives in, and the stop it should run instead.
type ComposeStopFix struct {
	Job     string
	Shared  string
	Current string
	Stop    string
}

// ComposeStopsOverShared finds the file jobs `run init` wrote before a lifted
// service changed their stop: `down` on the file a shared service was taken out
// of removes that service's container in the main checkout, where both share a
// project. The replacement stops the services the job starts, and only those.
func ComposeStopsOverShared(cfg domain.RunConfig) []ComposeStopFix {
	var fixes []ComposeStopFix
	for _, shared := range cfg.Jobs {
		if !IsShared(shared) {
			continue
		}
		prefix, ok := composePrefix(shared.Cmd, domain.ComposeUpVerb)
		if !ok {
			continue
		}
		for _, job := range cfg.Jobs {
			if IsShared(job) || !strings.HasPrefix(strings.TrimSpace(job.Stop), prefix+domain.ComposeDownVerb) {
				continue
			}
			services := composeUpServices(job.Cmd)
			if len(services) == 0 {
				continue
			}
			fixes = append(fixes, ComposeStopFix{
				Job:     job.Name,
				Shared:  shared.Name,
				Current: job.Stop,
				Stop:    fmt.Sprintf("%s%s %s", prefix, domain.ComposeRmStopVerb, strings.Join(services, " ")),
			})
		}
	}
	return fixes
}

// ComposeStopLines phrase the fixes as a warning a run surface prints.
func ComposeStopLines(fixes []ComposeStopFix) []string {
	lines := make([]string, 0, len(fixes))
	for _, fix := range fixes {
		lines = append(lines, fmt.Sprintf(domain.ComposeStopWarningFmt, fix.Job, fix.Shared, fix.Stop))
	}
	return lines
}

// composePrefix is everything before the compose verb — the command and its
// file flags — so two jobs are known to act on the same project.
func composePrefix(cmd, verb string) (string, bool) {
	cmd = strings.TrimSpace(cmd)
	if !strings.Contains(cmd, domain.ComposeWord) {
		return "", false
	}
	index := strings.Index(cmd, verb)
	if index < 0 {
		return "", false
	}
	return cmd[:index], true
}

// composeUpServices are the services an `up` names, its flags left out.
func composeUpServices(cmd string) []string {
	_, rest, found := strings.Cut(cmd, domain.ComposeUpVerb)
	if !found {
		return nil
	}
	var services []string
	for _, token := range strings.Fields(rest) {
		if strings.HasPrefix(token, "-") {
			continue
		}
		services = append(services, token)
	}
	return services
}

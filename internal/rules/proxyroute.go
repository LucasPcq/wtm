package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RouteHostParams struct {
	Job domain.JobConfig
	// Worktree and Project are the raw names; they are made DNS-safe here so a
	// caller never has to remember to.
	Worktree string
	Project  string
}

// RouteHost is the hostname a job is published under, empty for one that
// publishes nothing.
//
// The order is <job>.<worktree>.<project> and not the reverse: a cookie set on
// .<worktree>.<project>.localhost is then shared by that worktree's jobs and
// invisible to the others, which is the whole point. The reverse order would
// leak a cookie from one worktree to the next.
func RouteHost(params RouteHostParams) string {
	label := JobHostLabel(params.Job)
	if label == "" {
		return ""
	}
	// A shared job answers on one address for the whole repository, so its host
	// carries no worktree: keeping the segment would publish two names onto one
	// target, and the .env files of two worktrees would then disagree about
	// where a single service answers.
	if IsShared(params.Job) {
		return strings.Join([]string{label, HostLabel(params.Project), domain.ProxyTLD}, ".")
	}
	return strings.Join([]string{
		label,
		HostLabel(params.Worktree),
		HostLabel(params.Project),
		domain.ProxyTLD,
	}, ".")
}

// ProxyPort is the port the run proxy listens on, zero when it is switched off.
// Zero is what every caller reads to fall back to a job's own port, so the
// feature degrades to what it replaced instead of failing.
func ProxyPort(cfg domain.GlobalConfig) int {
	if cfg.Proxy.Enabled != nil && !*cfg.Proxy.Enabled {
		return 0
	}
	if cfg.Proxy.Port > 0 {
		return cfg.Proxy.Port
	}
	return domain.ProxyDefaultPort
}

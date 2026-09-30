package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type JobURLParams struct {
	Job domain.JobConfig
	// Ports is what the job actually bound in this worktree, base plus offset.
	Ports map[string]int
	// Host is the route the proxy serves this job under. Empty means no proxy —
	// the direct address is then the honest answer, not a degraded one.
	Host string
	// PublicPort is what the URL announces, not what the proxy binds.
	PublicPort int
}

type JobOriginParams struct {
	Host string
	// PublicPort of ProxyPrivilegedPort means the redirection is live and the
	// port vanishes from the origin.
	PublicPort int
	DirectPort int
}

func JobOrigin(params JobOriginParams) string {
	if params.Host != "" && params.PublicPort == domain.ProxyPrivilegedPort {
		return fmt.Sprintf(domain.ProxyOriginFmt, params.Host)
	}
	if params.Host != "" && params.PublicPort > 0 {
		return fmt.Sprintf(domain.ProxyURLFmt, params.Host, params.PublicPort)
	}
	if params.DirectPort > 0 {
		return fmt.Sprintf(domain.DirectURLFmt, params.DirectPort)
	}
	return ""
}

// JobURL is where a job is reachable, empty for one that publishes nothing. This
// is the single place a surface asks; the proxy changes what it answers, not who
// asks.
func JobURL(params JobURLParams) string {
	if params.Job.URL == nil {
		return ""
	}
	port, bound := params.Ports[params.Job.URL.Port]
	if !bound {
		return ""
	}
	return JobOrigin(JobOriginParams{Host: params.Host, PublicPort: params.PublicPort, DirectPort: port})
}

type JobURLFlagsParams struct {
	// Port and Host are the --url-port / --url-host pair as they were typed.
	Port string
	Host string
}

// JobURLFromFlags reads the pair as the one line ParseJobURL takes, so a flag
// and a form answer are validated by the same rule — and refuses a host with
// nothing to publish under it, which would otherwise be silently dropped.
func JobURLFromFlags(params JobURLFlagsParams) (*domain.JobURLConfig, error) {
	if params.Port == "" && params.Host != "" {
		return nil, fmt.Errorf(domain.RunJobURLHostOrphan, domain.FlagURLHost, domain.FlagURLPort)
	}
	return ParseJobURL(strings.TrimSpace(params.Port + " " + params.Host))
}

// ParseJobURL reads the `url` a wizard step or a flag pair expresses as one
// line: the port name to publish, optionally followed by the host to publish it
// under. Blank means the job publishes nothing.
func ParseJobURL(s string) (*domain.JobURLConfig, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil, nil
	}
	if len(fields) > 2 {
		return nil, fmt.Errorf("url takes a port name and an optional host, got %d values", len(fields))
	}
	if !IsEnvVarName(fields[0]) {
		return nil, fmt.Errorf("url port %q is not a valid environment variable name", fields[0])
	}
	cfg := &domain.JobURLConfig{Port: fields[0]}
	if len(fields) == 2 {
		if !IsHostLabels(fields[1]) {
			return nil, fmt.Errorf("url host %q is not a valid hostname — lowercase letters, digits and dashes, dot-separated", fields[1])
		}
		cfg.Host = fields[1]
	}
	return cfg, nil
}

// FormatJobURL is ParseJobURL's inverse, for a wizard step pre-filled with what
// the job already declares.
func FormatJobURL(cfg *domain.JobURLConfig) string {
	if cfg == nil {
		return ""
	}
	if cfg.Host == "" {
		return cfg.Port
	}
	return cfg.Port + " " + cfg.Host
}

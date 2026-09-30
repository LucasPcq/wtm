package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type LinkOriginParams struct {
	Job domain.JobConfig
	// PortName is the port the link follows, which is not always the one the
	// job publishes: a job may declare a metrics port alongside its HTTP one.
	PortName string
	Worktree string
	Project  string
	// PublicPort is what a named URL announces, zero when nothing serves names.
	PublicPort int
}

// LinkOrigin is the origin an [[env_port]] link writes under AddressingNames,
// empty when it must keep substituting a port instead.
//
// The three ways it comes back empty are the whole boundary of the feature: a
// job that publishes no name (Postgres never will — the proxy only speaks HTTP),
// a link following a port the published url does not carry, and a machine with
// no proxy to answer the name.
func LinkOrigin(params LinkOriginParams) string {
	if !PublishesPort(PublishesPortParams{Job: params.Job, PortName: params.PortName}) || params.PublicPort == 0 {
		return ""
	}
	host := RouteHost(RouteHostParams{
		Job: params.Job, Worktree: params.Worktree, Project: params.Project,
	})
	if host == "" {
		return ""
	}
	return JobOrigin(JobOriginParams{Host: host, PublicPort: params.PublicPort})
}

type PublishesPortParams struct {
	Job domain.JobConfig
	// PortName is the port the caller is asking about, which is not always the
	// one the job publishes: a job may declare a metrics port beside its HTTP
	// one, and only the published one has an address.
	PortName string
}

// PublishesPort reports whether the job publishes a name for this very port.
func PublishesPort(params PublishesPortParams) bool {
	return params.Job.URL != nil && params.Job.URL.Port == params.PortName
}

type RewriteOriginParams struct {
	Value string
	// Origin is what LinkOrigin resolved, already complete with its scheme.
	Origin   string
	JobLabel string
	Project  string
	// Base and Resolved are the ports that anchor a value still spelled by
	// port: the declaration, and what this worktree binds.
	Base     int
	Resolved int
}

// OriginRewrite is one value resolved against the origin its link publishes.
// Fallback is not a verdict but a redirection: the value has no URL in it, so
// the port substitution answers instead and this whole path never happened.
type OriginRewrite struct {
	Value    string
	Status   domain.EnvPortStatus
	Fallback bool
	// ForeignHost is the authority that made the value foreign, for the report.
	ForeignHost string
}

// RewriteOrigin swaps the authority of every element of a value that addresses
// this job, leaving the rest of the value — path, query, and the elements
// belonging to somebody else — exactly as written.
//
// Replacing an authority is structural, which is why the ambiguity that haunts
// the port substitution cannot arise here: a port sitting in a path or a query
// is never a candidate, so `http://localhost:4001/cb?port=4001` needs no
// disambiguation.
func RewriteOrigin(params RewriteOriginParams) OriginRewrite {
	elements := splitList(params.Value)

	var urls int
	var mine []int
	var foreign string
	for i, element := range elements {
		scheme, authority, ok := splitOrigin(element.value)
		if !ok {
			continue
		}
		urls++
		if !ownedAuthority(ownedAuthorityParams{
			Authority: authority, JobLabel: params.JobLabel, Project: params.Project,
			Base: params.Base, Resolved: params.Resolved,
		}) {
			// A loopback address on some other port is not foreign, only
			// unanchored: it names this machine, just not this job's port.
			if host, _ := splitHostPort(authority); !isLoopbackHost(host) && foreign == "" {
				foreign = authority
			}
			continue
		}
		if scheme == domain.OriginSchemeHTTPS {
			return OriginRewrite{Status: domain.EnvPortStatusSecureScheme}
		}
		mine = append(mine, i)
	}

	if urls == 0 {
		return OriginRewrite{Fallback: true}
	}
	if len(mine) == 0 {
		if foreign != "" {
			return OriginRewrite{Status: domain.EnvPortStatusForeignHost, ForeignHost: foreign}
		}
		return OriginRewrite{Status: domain.EnvPortStatusNotFound}
	}

	for _, i := range mine {
		elements[i].value = replaceAuthority(elements[i].value, params.Origin)
	}
	rewritten := joinList(elements)
	if rewritten == params.Value {
		return OriginRewrite{Status: domain.EnvPortStatusUnchanged}
	}
	return OriginRewrite{Value: rewritten, Status: domain.EnvPortStatusRewrite}
}

type ReduceOriginParams struct {
	Value    string
	JobLabel string
	Project  string
	Base     int
}

// ReduceOriginValue rewinds a value wtm wrote as an origin back to the port it
// stands for. It is the inverse of RewriteOrigin and serves two callers at once:
// the switch back to AddressingPorts, and the inter-worktree diff, which can
// only compare two worktrees' spellings of one setting once both reduce to the
// same string.
//
// Only this job's routes are touched. Another job's, or another project's, are
// left alone — rewinding them on a guess would hide a real difference.
func ReduceOriginValue(params ReduceOriginParams) string {
	elements := splitList(params.Value)

	changed := false
	for i, element := range elements {
		_, authority, ok := splitOrigin(element.value)
		if !ok || !isRouteAuthority(routeAuthorityParams{
			Authority: authority, JobLabel: params.JobLabel, Project: params.Project,
		}) {
			continue
		}
		elements[i].value = replaceAuthority(element.value,
			fmt.Sprintf(domain.DirectURLFmt, params.Base))
		changed = true
	}
	if !changed {
		return params.Value
	}
	return joinList(elements)
}

type routeAuthorityParams struct {
	Authority string
	JobLabel  string
	Project   string
}

type ownedAuthorityParams struct {
	Authority string
	JobLabel  string
	Project   string
	Base      int
	Resolved  int
}

func (p ownedAuthorityParams) route() routeAuthorityParams {
	return routeAuthorityParams{Authority: p.Authority, JobLabel: p.JobLabel, Project: p.Project}
}

// ownedAuthority reports whether an authority addresses this job — either as a
// route wtm already wrote, whatever worktree segment and port it carries, or as
// a loopback address on one of the two ports the link can legitimately hold.
func ownedAuthority(params ownedAuthorityParams) bool {
	if isRouteAuthority(params.route()) {
		return true
	}
	host, port := splitHostPort(params.Authority)
	if !isLoopbackHost(host) {
		return false
	}
	return port == params.Base || port == params.Resolved
}

// isRouteAuthority matches <job label>.<worktree>.<project label>.localhost
// without splitting on dots: a hand-written url.host may carry its own, and the
// worktree segment is the only part that varies between two copies of a value.
func isRouteAuthority(params routeAuthorityParams) bool {
	if params.JobLabel == "" || params.Project == "" {
		return false
	}
	host, _ := splitHostPort(params.Authority)

	prefix := params.JobLabel + "."
	suffix := "." + HostLabel(params.Project) + "." + domain.ProxyTLD
	if !strings.HasPrefix(host, prefix) || !strings.HasSuffix(host, suffix) {
		return false
	}
	worktree := host[len(prefix) : len(host)-len(suffix)]
	return worktree != "" && !strings.Contains(worktree, ".")
}

func isLoopbackHost(host string) bool {
	switch host {
	case domain.ProxyTLD, domain.LoopbackIPv4, domain.LoopbackIPv6:
		return true
	default:
		return false
	}
}

// splitOrigin cuts an element into its scheme and its host:port, credentials
// left out, reporting false for anything the proxy could not serve — a bare
// number, or a scheme it does not speak, both of which belong to the port
// substitution instead.
func splitOrigin(element string) (scheme, authority string, ok bool) {
	at := strings.Index(element, domain.OriginSchemeSeparator)
	if at < 0 {
		return "", "", false
	}
	scheme = element[:at]
	if scheme != domain.OriginSchemeHTTP && scheme != domain.OriginSchemeHTTPS {
		return "", "", false
	}
	_, hostPort, _ := splitAuthority(element[at+len(domain.OriginSchemeSeparator):])
	if hostPort == "" {
		return "", "", false
	}
	return scheme, hostPort, true
}

// splitAuthority cuts what follows the scheme into its credentials (with their
// `@`), its host:port, and the path, query and fragment after them.
func splitAuthority(rest string) (userinfo, hostPort, tail string) {
	if end := strings.IndexAny(rest, "/?#"); end >= 0 {
		rest, tail = rest[:end], rest[end:]
	}
	if at := strings.LastIndex(rest, domain.EnvCredentialsSeparator); at >= 0 {
		return rest[:at+1], rest[at+1:], tail
	}
	return "", rest, tail
}

// replaceAuthority swaps the host and port for origin's, keeping credentials,
// path, query and fragment byte for byte — a redirect URL carrying a
// percent-encoded address must survive untouched.
func replaceAuthority(element, origin string) string {
	at := strings.Index(element, domain.OriginSchemeSeparator)
	userinfo, _, tail := splitAuthority(element[at+len(domain.OriginSchemeSeparator):])
	if userinfo == "" {
		return origin + tail
	}
	cut := strings.Index(origin, domain.OriginSchemeSeparator) + len(domain.OriginSchemeSeparator)
	return origin[:cut] + userinfo + origin[cut:] + tail
}

func splitHostPort(authority string) (host string, port int) {
	at := strings.LastIndex(authority, ":")
	if at < 0 || strings.Contains(authority[at:], "]") {
		return authority, 0
	}
	n, err := strconv.Atoi(authority[at+1:])
	if err != nil {
		return authority, 0
	}
	return authority[:at], n
}

// listElement is one comma-separated value with the spacing that surrounded it,
// so a list is rebuilt exactly as the file spelled it.
type listElement struct {
	lead  string
	value string
	trail string
}

func splitList(value string) []listElement {
	parts := strings.Split(value, domain.OriginListSeparator)
	out := make([]listElement, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		lead := part[:strings.Index(part, trimmed)]
		if trimmed == "" {
			lead = part
		}
		out = append(out, listElement{
			lead:  lead,
			value: trimmed,
			trail: part[len(lead)+len(trimmed):],
		})
	}
	return out
}

func joinList(elements []listElement) string {
	parts := make([]string, 0, len(elements))
	for _, e := range elements {
		parts = append(parts, e.lead+e.value+e.trail)
	}
	return strings.Join(parts, domain.OriginListSeparator)
}

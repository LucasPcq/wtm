package runenv

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

const (
	proxyTLD        = "localhost"
	loopbackIPv4    = "127.0.0.1"
	loopbackIPv6    = "[::1]"
	schemeHTTP      = "http"
	schemeHTTPS     = "https"
	schemeSeparator = "://"
	credentialsEnd  = "@"
	listSeparator   = ","
	directURLFmt    = "http://localhost:%d"
	proxyOriginFmt  = "http://%s"
	proxyURLFmt     = "http://%s:%d"
)

type originContext struct {
	Addressing run.Addressing
	Jobs       map[string]run.Job
	Worktree   string
	Project    string
	PublicPort int
}

func (c originContext) names() bool {
	return c.Addressing == run.AddressNames && c.PublicPort > 0
}

func (c originContext) jobLabel(job string) string {
	return jobHostLabel(c.Jobs[job])
}

func effectiveAddressing(cfg run.Config) run.Addressing {
	if cfg.Addressing == run.AddressPorts {
		return run.AddressPorts
	}
	return run.AddressNames
}

func jobHostLabel(job run.Job) string {
	if job.URL == nil {
		return ""
	}
	if job.URL.Host != "" {
		return job.URL.Host
	}
	return branch.HostLabel(job.Name)
}

type routeHostParams struct {
	Job      run.Job
	Worktree string
	Project  string
}

func routeHost(params routeHostParams) string {
	label := jobHostLabel(params.Job)
	if label == "" {
		return ""
	}
	if params.Job.Scope == run.Shared {
		return strings.Join([]string{label, branch.HostLabel(params.Project), proxyTLD}, ".")
	}
	return strings.Join([]string{label, branch.HostLabel(params.Worktree), branch.HostLabel(params.Project), proxyTLD}, ".")
}

type jobOriginParams struct {
	Host       string
	PublicPort int
	DirectPort int
}

func jobOrigin(params jobOriginParams) string {
	switch {
	case params.Host != "" && params.PublicPort == PrivilegedProxyPort:
		return fmt.Sprintf(proxyOriginFmt, params.Host)
	case params.Host != "" && params.PublicPort > 0:
		return fmt.Sprintf(proxyURLFmt, params.Host, params.PublicPort)
	case params.DirectPort > 0:
		return fmt.Sprintf(directURLFmt, params.DirectPort)
	}
	return ""
}

type linkOriginParams struct {
	Job        run.Job
	PortName   string
	Worktree   string
	Project    string
	PublicPort int
}

func linkOrigin(params linkOriginParams) string {
	if !publishesPort(publishesPortParams{Job: params.Job, PortName: params.PortName}) || params.PublicPort == 0 {
		return ""
	}
	host := routeHost(routeHostParams{Job: params.Job, Worktree: params.Worktree, Project: params.Project})
	if host == "" {
		return ""
	}
	return jobOrigin(jobOriginParams{Host: host, PublicPort: params.PublicPort})
}

type publishesPortParams struct {
	Job      run.Job
	PortName string
}

func publishesPort(params publishesPortParams) bool {
	return params.Job.URL != nil && params.Job.URL.Port == params.PortName
}

type rewriteOriginParams struct {
	Value    string
	Origin   string
	JobLabel string
	Project  string
	Base     int
	Resolved int
}

type originRewrite struct {
	Value       string
	Status      Status
	Fallback    bool
	ForeignHost string
}

func rewriteOrigin(params rewriteOriginParams) originRewrite {
	elements := splitList(params.Value)
	urls := 0
	var mine []int
	foreign := ""
	for i, element := range elements {
		scheme, authority, ok := splitOrigin(element.value)
		if !ok {
			continue
		}
		urls++
		owned := ownedAuthority(ownedAuthorityParams{Authority: authority, JobLabel: params.JobLabel, Project: params.Project, Base: params.Base, Resolved: params.Resolved})
		if !owned {
			foreign = firstForeign(foreign, authority)
			continue
		}
		if scheme == schemeHTTPS {
			return originRewrite{Status: StatusSecureScheme}
		}
		mine = append(mine, i)
	}
	switch {
	case urls == 0:
		return originRewrite{Fallback: true}
	case len(mine) == 0 && foreign != "":
		return originRewrite{Status: StatusForeignHost, ForeignHost: foreign}
	case len(mine) == 0:
		return originRewrite{Status: StatusNotFound}
	}
	for _, i := range mine {
		elements[i].value = replaceAuthority(elements[i].value, params.Origin)
	}
	rewritten := joinList(elements)
	if rewritten == params.Value {
		return originRewrite{Status: StatusUnchanged}
	}
	return originRewrite{Value: rewritten, Status: StatusRewrite}
}

func firstForeign(found, authority string) string {
	host, _ := splitHostPort(authority)
	if found != "" || isLoopbackHost(host) {
		return found
	}
	return authority
}

type reduceOriginParams struct {
	Value    string
	JobLabel string
	Project  string
	Base     int
}

func reduceOriginValue(params reduceOriginParams) string {
	elements := splitList(params.Value)
	changed := false
	for i, element := range elements {
		_, authority, ok := splitOrigin(element.value)
		if !ok || !isRouteAuthority(routeAuthorityParams{Authority: authority, JobLabel: params.JobLabel, Project: params.Project}) {
			continue
		}
		elements[i].value = replaceAuthority(element.value, fmt.Sprintf(directURLFmt, params.Base))
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

func ownedAuthority(params ownedAuthorityParams) bool {
	if isRouteAuthority(routeAuthorityParams{Authority: params.Authority, JobLabel: params.JobLabel, Project: params.Project}) {
		return true
	}
	host, port := splitHostPort(params.Authority)
	return isLoopbackHost(host) && (port == params.Base || port == params.Resolved)
}

func isRouteAuthority(params routeAuthorityParams) bool {
	if params.JobLabel == "" || params.Project == "" {
		return false
	}
	host, _ := splitHostPort(params.Authority)
	prefix := params.JobLabel + "."
	suffix := "." + branch.HostLabel(params.Project) + "." + proxyTLD
	if !strings.HasPrefix(host, prefix) || !strings.HasSuffix(host, suffix) || len(host) <= len(prefix)+len(suffix) {
		return false
	}
	worktree := host[len(prefix) : len(host)-len(suffix)]
	return !strings.Contains(worktree, ".")
}

func isLoopbackHost(host string) bool {
	return host == proxyTLD || host == loopbackIPv4 || host == loopbackIPv6
}

func splitOrigin(element string) (string, string, bool) {
	scheme, rest, found := strings.Cut(element, schemeSeparator)
	if !found || (scheme != schemeHTTP && scheme != schemeHTTPS) {
		return "", "", false
	}
	_, hostPort, _ := splitAuthority(rest)
	if hostPort == "" {
		return "", "", false
	}
	return scheme, hostPort, true
}

func splitAuthority(rest string) (string, string, string) {
	tail := ""
	if end := strings.IndexAny(rest, "/?#"); end >= 0 {
		rest, tail = rest[:end], rest[end:]
	}
	if at := strings.LastIndex(rest, credentialsEnd); at >= 0 {
		return rest[:at+1], rest[at+1:], tail
	}
	return "", rest, tail
}

func replaceAuthority(element, origin string) string {
	at := strings.Index(element, schemeSeparator)
	userinfo, _, tail := splitAuthority(element[at+len(schemeSeparator):])
	if userinfo == "" {
		return origin + tail
	}
	cut := strings.Index(origin, schemeSeparator) + len(schemeSeparator)
	return origin[:cut] + userinfo + origin[cut:] + tail
}

func splitHostPort(authority string) (string, int) {
	at := strings.LastIndex(authority, ":")
	if at < 0 || strings.Contains(authority[at:], "]") {
		return authority, 0
	}
	port, err := strconv.Atoi(authority[at+1:])
	if err != nil {
		return authority, 0
	}
	return authority[:at], port
}

type listElement struct {
	lead  string
	value string
	trail string
}

func splitList(value string) []listElement {
	parts := strings.Split(value, listSeparator)
	elements := make([]listElement, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		lead := part[:strings.Index(part, trimmed)]
		if trimmed == "" {
			lead = part
		}
		elements = append(elements, listElement{lead: lead, value: trimmed, trail: part[len(lead)+len(trimmed):]})
	}
	return elements
}

func joinList(elements []listElement) string {
	parts := make([]string, 0, len(elements))
	for _, e := range elements {
		parts = append(parts, e.lead+e.value+e.trail)
	}
	return strings.Join(parts, listSeparator)
}

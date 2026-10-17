package env

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// The run proxy serves a worktree's jobs under <job>.<worktree>.<project>.localhost; on port 80 an origin leaves its port out.
const (
	privilegedProxyPort = 80
	proxyTLD            = "localhost"
)

// effectiveAddressing returns the addressing of cfg: by ports when it asks for them, by names otherwise.
func effectiveAddressing(cfg project.Run) project.Addressing {
	if cfg.Addressing == project.AddressPorts {
		return project.AddressPorts
	}
	return project.AddressNames
}

// jobHostLabel returns the first label of the job's host name, its url.host or else its name made DNS-safe, empty when the job publishes no url.
func jobHostLabel(job project.Job) string {
	if job.URL == nil {
		return ""
	}
	return cmp.Or(job.URL.Host, worktree.HostLabel(job.Name))
}

// routeHost returns the host name the proxy serves job under, one per worktree unless the job is shared, empty when the job publishes no url.
func (w settling) routeHost(job project.Job) string {
	label := jobHostLabel(job)
	if label == "" {
		return ""
	}
	if job.Scope == project.Shared {
		return strings.Join([]string{label, worktree.HostLabel(w.Project), proxyTLD}, ".")
	}
	return strings.Join([]string{label, worktree.HostLabel(w.Slug), worktree.HostLabel(w.Project), proxyTLD}, ".")
}

// linkOrigin returns the origin the proxy serves the link's port under, empty when the port is not the one its job publishes or the proxy serves no port.
func (w settling) linkOrigin(link project.EnvPortLink) string {
	job := w.Jobs[link.Job]
	if job.URL == nil || job.URL.Port != link.Port || w.PublicPort == 0 {
		return ""
	}

	host := w.routeHost(job)
	switch {
	case host == "":
		return ""
	case w.PublicPort == privilegedProxyPort:
		return "http://" + host
	}
	return fmt.Sprintf("http://%s:%d", host, w.PublicPort)
}

// originEntry returns entry planned as a rewrite to the link's named origin, and reports whether it was planned so; when it was not — no names addressing, no origin, or a value holding no http url — the link is planned by its port instead.
func (w settling) originEntry(entry Entry, link project.EnvPortLink) (Entry, bool) {
	origin := w.linkOrigin(link)
	if !w.names() || origin == "" {
		return entry, false
	}

	rewrite := rewriteOrigin(rewriteOriginParams{Value: entry.CurrentValue, Origin: origin, JobLabel: w.jobLabel(link.Job), Project: w.Project, Base: entry.Base, Resolved: entry.Resolved})
	if rewrite.Fallback {
		return entry, false
	}

	entry.Addressing = project.AddressNames
	entry.Status = rewrite.Status
	entry.NewValue = rewrite.Value
	entry.ForeignHost = rewrite.ForeignHost
	return entry, true
}

type rewriteOriginParams struct {
	Value    string
	Origin   string
	JobLabel string
	Project  string
	Base     int
	Resolved int
}

// An originRewrite is what rewriting a value's origins decided: the new value and its status, or Fallback when the value holds no http url and is planned by its port instead.
type originRewrite struct {
	Value       string
	Status      Status
	Fallback    bool
	ForeignHost string
}

// rewriteOrigin replaces with Origin every url of the comma-separated Value that points at the job — a loopback url on its base or resolved port, or a route of the job in any worktree. An https url of the job refuses the whole value rather than downgrade it to the proxy's http; a value whose urls all point elsewhere reports the first foreign host.
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
		if !params.owns(authority) {
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

// owns reports whether authority points at the job: a route of the job, or a loopback host on its base or resolved port.
func (p rewriteOriginParams) owns(authority string) bool {
	if isRouteAuthority(routeAuthorityParams{Authority: authority, JobLabel: p.JobLabel, Project: p.Project}) {
		return true
	}
	host, port := splitHostPort(authority)
	return isLoopbackHost(host) && (port == p.Base || port == p.Resolved)
}

// firstForeign returns found when one is already found, else authority unless its host is a loopback one.
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

// reduceOriginValue rewinds every route of the job in the comma-separated Value, whatever its worktree, to http://localhost:<Base>, so the value can be settled again; a value holding no route of the job is returned as it is.
func reduceOriginValue(params reduceOriginParams) string {
	elements := splitList(params.Value)
	changed := false
	for i, element := range elements {
		_, authority, ok := splitOrigin(element.value)
		if !ok || !isRouteAuthority(routeAuthorityParams{Authority: authority, JobLabel: params.JobLabel, Project: params.Project}) {
			continue
		}
		elements[i].value = replaceAuthority(element.value, fmt.Sprintf("http://localhost:%d", params.Base))
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

// isRouteAuthority reports whether Authority is a route of the job in the project, <job>.<worktree>.<project>.localhost for any one worktree.
func isRouteAuthority(params routeAuthorityParams) bool {
	if params.JobLabel == "" || params.Project == "" {
		return false
	}

	host, _ := splitHostPort(params.Authority)
	prefix := params.JobLabel + "."
	suffix := "." + worktree.HostLabel(params.Project) + "." + proxyTLD
	rest, isJob := strings.CutPrefix(host, prefix)
	label, inProject := strings.CutSuffix(rest, suffix)
	return isJob && inProject && label != "" && !strings.Contains(label, ".")
}

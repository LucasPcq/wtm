package settle

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

const (
	privilegedProxyPort = 80
	proxyTLD            = "localhost"
)

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
	return cmp.Or(job.URL.Host, branch.HostLabel(job.Name))
}

func (w worktree) routeHost(job run.Job) string {
	label := jobHostLabel(job)
	if label == "" {
		return ""
	}
	if job.Scope == run.Shared {
		return strings.Join([]string{label, branch.HostLabel(w.Project), proxyTLD}, ".")
	}
	return strings.Join([]string{label, branch.HostLabel(w.Slug), branch.HostLabel(w.Project), proxyTLD}, ".")
}

func (w worktree) linkOrigin(link run.EnvPortLink) string {
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

type originEntryParams struct {
	Entry Entry
	Link  run.EnvPortLink
}

func (w worktree) originEntry(params originEntryParams) (Entry, bool) {
	entry := params.Entry
	origin := w.linkOrigin(params.Link)
	if !w.names() || origin == "" {
		return entry, false
	}
	rewrite := rewriteOrigin(rewriteOriginParams{Value: entry.CurrentValue, Origin: origin, JobLabel: w.jobLabel(params.Link.Job), Project: w.Project, Base: entry.Base, Resolved: entry.Resolved})
	if rewrite.Fallback {
		return entry, false
	}
	entry.Addressing = run.AddressNames
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

func (p rewriteOriginParams) owns(authority string) bool {
	if isRouteAuthority(routeAuthorityParams{Authority: authority, JobLabel: p.JobLabel, Project: p.Project}) {
		return true
	}
	host, port := splitHostPort(authority)
	return isLoopbackHost(host) && (port == p.Base || port == p.Resolved)
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

func isRouteAuthority(params routeAuthorityParams) bool {
	if params.JobLabel == "" || params.Project == "" {
		return false
	}
	host, _ := splitHostPort(params.Authority)
	prefix := params.JobLabel + "."
	suffix := "." + branch.HostLabel(params.Project) + "." + proxyTLD
	rest, isJob := strings.CutPrefix(host, prefix)
	label, inProject := strings.CutSuffix(rest, suffix)
	return isJob && inProject && label != "" && !strings.Contains(label, ".")
}

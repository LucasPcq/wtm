package settle

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/jobenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

const (
	tokenNamespace  = "{namespace}"
	tokenOrigin     = "{origin}"
	tokenWorktree   = "{worktree}"
	tokenOrdinal    = "{ordinal}"
	tokenPortPrefix = "{port."
)

var (
	valueToken     = regexp.MustCompile(`\{[a-zA-Z_][a-zA-Z0-9_.]*\}`)
	namespaceToken = regexp.MustCompile(`\{[a-zA-Z_]+\}`)

	errNamespaceUnknownToken = errors.New("unknown placeholder in a [job.namespace] value")
)

func (w worktree) valueWrites(links []run.EnvValueLink) ([]OwnedEntry, error) {
	writes := make([]OwnedEntry, 0, len(links))
	for _, link := range links {
		value, err := w.expand(link)
		if err != nil {
			return nil, err
		}
		writes = append(writes, OwnedEntry{File: link.File, Key: link.Key, Value: value})
	}
	return writes, nil
}

func (w worktree) expand(link run.EnvValueLink) (string, error) {
	job, found := w.Jobs[link.Job]
	if !found {
		return "", fmt.Errorf("env %s in %s: no job named %q", link.Key, link.File, link.Job)
	}
	value, err := w.expandPorts(valueOf{Link: link, Job: job})
	if err != nil {
		return "", err
	}
	if strings.Contains(value, tokenNamespace) {
		namespace, err := w.linkNamespace(valueOf{Link: link, Job: job})
		if err != nil {
			return "", err
		}
		value = strings.ReplaceAll(value, tokenNamespace, namespace)
	}
	origin := w.valueOrigin(job)
	if strings.Contains(value, tokenOrigin) && origin == "" {
		return "", fmt.Errorf("env %s in %s: job %q publishes no address, so {origin} has no answer — publish a [job.url] for it, or write the host yourself", link.Key, link.File, job.Name)
	}
	value = strings.NewReplacer(tokenOrigin, origin, tokenWorktree, w.Slug, tokenOrdinal, strconv.Itoa(w.Ordinal)).Replace(value)
	if leftover := valueToken.FindString(value); leftover != "" {
		return "", fmt.Errorf("env %s in %s: unknown placeholder %s", link.Key, link.File, leftover)
	}
	return value, nil
}

type valueOf struct {
	Link run.EnvValueLink
	Job  run.Job
}

func (w worktree) expandPorts(params valueOf) (string, error) {
	value := params.Link.Value
	for {
		start := strings.Index(value, tokenPortPrefix)
		if start < 0 {
			return value, nil
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return "", fmt.Errorf("env %s in %s: a {port.…} placeholder is never closed", params.Link.Key, params.Link.File)
		}
		end += start
		name := value[start+len(tokenPortPrefix) : end]
		base, declared := params.Job.Ports[name]
		if !declared {
			return "", fmt.Errorf("env %s in %s: job %q declares no port named %q", params.Link.Key, params.Link.File, params.Job.Name, name)
		}
		port := jobenv.ResolvedPort(jobenv.ResolvedPortParams{Base: base, Offset: w.Offset, Shared: params.Job.Scope == run.Shared})
		value = value[:start] + strconv.Itoa(port) + value[end+1:]
	}
}

func (w worktree) linkNamespace(params valueOf) (string, error) {
	if params.Job.Namespace == nil {
		return "", fmt.Errorf("env %s in %s: job %q declares no [job.namespace], so {namespace} means nothing", params.Link.Key, params.Link.File, params.Job.Name)
	}
	name, err := w.namespace(*params.Job.Namespace)
	if err != nil {
		return "", fmt.Errorf("env %s in %s: %v", params.Link.Key, params.Link.File, err)
	}
	return name, nil
}

func (w worktree) namespace(namespace run.Namespace) (string, error) {
	replacer := strings.NewReplacer(tokenWorktree, w.Slug, tokenOrdinal, strconv.Itoa(w.Ordinal))
	name := replacer.Replace(namespace.Name)
	for _, value := range append([]string{namespace.Name}, slices.Collect(maps.Values(namespace.Env))...) {
		if leftover := namespaceToken.FindString(replacer.Replace(value)); leftover != "" {
			return "", fmt.Errorf("%w: %s", errNamespaceUnknownToken, leftover)
		}
	}
	return name, nil
}

func (w worktree) valueOrigin(job run.Job) string {
	if job.URL == nil || !w.names() {
		return ""
	}
	return w.linkOrigin(run.EnvPortLink{Job: job.Name, Port: job.URL.Port})
}

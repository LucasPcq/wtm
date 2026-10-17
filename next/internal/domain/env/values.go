package env

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// The placeholders a value of run.toml's [[env]] may hold; {port.<name>} names a port of the value's job.
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
	errNoProxy               = errors.New("served by the run proxy, which serves no port")
)

// A ValueError is a value of run.toml's [[env]] that cannot be expanded: a job, a port or a namespace it names is not declared, or it holds a placeholder with no answer.
type ValueError struct{ Err error }

// Error returns the message of the cause, which names the key and the file.
func (e *ValueError) Error() string { return e.Err.Error() }

// Unwrap returns the cause.
func (e *ValueError) Unwrap() error { return e.Err }

// valueWrites expands each value link into the value wtm owns in its file.
func (w settling) valueWrites(links []project.EnvValueLink) ([]OwnedEntry, error) {
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

// expand returns the link's value with every placeholder replaced for the worktree. It fails, wrapping errNoProxy, when {origin} needs the proxy and it serves no port, and on a placeholder it cannot answer.
func (w settling) expand(link project.EnvValueLink) (string, error) {
	job, found := w.Jobs[link.Job]
	if !found {
		return "", fmt.Errorf("env %s in %s: no job named %q", link.Key, link.File, link.Job)
	}

	value, err := w.expandPorts(link, job)
	if err != nil {
		return "", err
	}

	if strings.Contains(value, tokenNamespace) {
		namespace, err := w.linkNamespace(link, job)
		if err != nil {
			return "", err
		}
		value = strings.ReplaceAll(value, tokenNamespace, namespace)
	}

	origin := w.valueOrigin(job)
	if strings.Contains(value, tokenOrigin) && origin == "" && job.URL != nil && w.Addressing == project.AddressNames {
		return "", fmt.Errorf("env %s in %s: {origin} of job %q is %w: enable the proxy, or write the host yourself", link.Key, link.File, job.Name, errNoProxy)
	}
	if strings.Contains(value, tokenOrigin) && origin == "" {
		return "", fmt.Errorf("env %s in %s: job %q publishes no address, so {origin} has no answer — publish a [job.url] for it, or write the host yourself", link.Key, link.File, job.Name)
	}

	value = strings.NewReplacer(tokenOrigin, origin, tokenWorktree, w.Slug, tokenOrdinal, strconv.Itoa(w.Ordinal)).Replace(value)
	if leftover := valueToken.FindString(value); leftover != "" {
		return "", fmt.Errorf("env %s in %s: unknown placeholder %s", link.Key, link.File, leftover)
	}
	return value, nil
}

// expandPorts returns the link's value with each {port.<name>} replaced by the job's port resolved for the worktree.
func (w settling) expandPorts(link project.EnvValueLink, job project.Job) (string, error) {
	value := link.Value
	for {
		before, rest, found := strings.Cut(value, tokenPortPrefix)
		if !found {
			return value, nil
		}

		name, after, closed := strings.Cut(rest, "}")
		if !closed {
			return "", fmt.Errorf("env %s in %s: a {port.…} placeholder is never closed", link.Key, link.File)
		}

		base, declared := job.Ports[name]
		if !declared {
			return "", fmt.Errorf("env %s in %s: job %q declares no port named %q", link.Key, link.File, job.Name, name)
		}

		port := ResolvedPort(ResolvedPortParams{Base: base, Offset: w.Offset, Shared: job.Scope == project.Shared})
		value = before + strconv.Itoa(port) + after
	}
}

// linkNamespace returns the job's namespace for the worktree, failing when the job declares none.
func (w settling) linkNamespace(link project.EnvValueLink, job project.Job) (string, error) {
	if job.Namespace == nil {
		return "", fmt.Errorf("env %s in %s: job %q declares no [job.namespace], so {namespace} means nothing", link.Key, link.File, job.Name)
	}

	name, err := w.namespace(*job.Namespace)
	if err != nil {
		return "", fmt.Errorf("env %s in %s: %v", link.Key, link.File, err)
	}
	return name, nil
}

// namespace returns the namespace's name for the worktree. It fails with errNamespaceUnknownToken when its name or one of its env values holds a placeholder other than {worktree} and {ordinal}.
func (w settling) namespace(namespace project.Namespace) (string, error) {
	replacer := strings.NewReplacer(tokenWorktree, w.Slug, tokenOrdinal, strconv.Itoa(w.Ordinal))
	name := replacer.Replace(namespace.Name)
	for _, value := range slices.AppendSeq([]string{namespace.Name}, maps.Values(namespace.Env)) {
		if leftover := namespaceToken.FindString(replacer.Replace(value)); leftover != "" {
			return "", fmt.Errorf("%w: %s", errNamespaceUnknownToken, leftover)
		}
	}
	return name, nil
}

// valueOrigin returns the named origin of the url the job publishes, empty when jobs are not addressed by name or it publishes none.
func (w settling) valueOrigin(job project.Job) string {
	if job.URL == nil || !w.names() {
		return ""
	}
	return w.linkOrigin(project.EnvPortLink{Job: job.Name, Port: job.URL.Port})
}

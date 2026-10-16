package runenv

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

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

type expandValueParams struct {
	Link     run.EnvValueLink
	Job      run.Job
	Worktree string
	Ordinal  int
	Offset   int
	Shared   bool
	Origin   string
}

func expandValue(params expandValueParams) (string, error) {
	value, err := expandPortTokens(params)
	if err != nil {
		return "", err
	}
	if strings.Contains(value, tokenNamespace) {
		namespace, err := linkNamespace(params)
		if err != nil {
			return "", err
		}
		value = strings.ReplaceAll(value, tokenNamespace, namespace)
	}
	if strings.Contains(value, tokenOrigin) && params.Origin == "" {
		return "", fmt.Errorf("env %s in %s: job %q publishes no address, so {origin} has no answer — publish a [job.url] for it, or write the host yourself", params.Link.Key, params.Link.File, params.Job.Name)
	}
	value = strings.NewReplacer(tokenOrigin, params.Origin, tokenWorktree, params.Worktree, tokenOrdinal, strconv.Itoa(params.Ordinal)).Replace(value)
	if leftover := valueToken.FindString(value); leftover != "" {
		return "", fmt.Errorf("env %s in %s: unknown placeholder %s", params.Link.Key, params.Link.File, leftover)
	}
	return value, nil
}

func linkNamespace(params expandValueParams) (string, error) {
	if params.Job.Namespace == nil {
		return "", fmt.Errorf("env %s in %s: job %q declares no [job.namespace], so {namespace} means nothing", params.Link.Key, params.Link.File, params.Job.Name)
	}
	name, err := expandNamespace(namespaceParams{Namespace: *params.Job.Namespace, Worktree: params.Worktree, Ordinal: params.Ordinal})
	if err != nil {
		return "", fmt.Errorf("env %s in %s: %v", params.Link.Key, params.Link.File, err)
	}
	return name, nil
}

func expandPortTokens(params expandValueParams) (string, error) {
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
		port := resolvedPort(resolvedPortParams{Base: base, Offset: params.Offset, Shared: params.Shared})
		value = value[:start] + strconv.Itoa(port) + value[end+1:]
	}
}

type namespaceParams struct {
	Namespace run.Namespace
	Worktree  string
	Ordinal   int
}

func expandNamespace(params namespaceParams) (string, error) {
	replacer := strings.NewReplacer(tokenWorktree, params.Worktree, tokenOrdinal, strconv.Itoa(params.Ordinal))
	name := replacer.Replace(params.Namespace.Name)
	values := append([]string{name}, slices.Collect(maps.Values(params.Namespace.Env))...)
	for _, value := range values {
		if leftover := namespaceToken.FindString(replacer.Replace(value)); leftover != "" {
			return "", fmt.Errorf("%w: %s", errNamespaceUnknownToken, leftover)
		}
	}
	return name, nil
}

type valueWritesParams struct {
	Run      run.Config
	Worktree string
	Ordinal  int
	Offset   int
	Origins  originContext
}

func valueWrites(params valueWritesParams) ([]OwnedEntry, error) {
	if len(params.Run.EnvValues) == 0 {
		return nil, nil
	}
	jobs := jobsByName(params.Run)
	writes := make([]OwnedEntry, 0, len(params.Run.EnvValues))
	for _, link := range params.Run.EnvValues {
		job, found := jobs[link.Job]
		if !found {
			return nil, fmt.Errorf("env %s in %s: no job named %q", link.Key, link.File, link.Job)
		}
		value, err := expandValue(expandValueParams{
			Link:     link,
			Job:      job,
			Worktree: params.Worktree,
			Ordinal:  params.Ordinal,
			Offset:   params.Offset,
			Shared:   job.Scope == run.Shared,
			Origin:   valueOrigin(valueOriginParams{Job: job, Origins: params.Origins}),
		})
		if err != nil {
			return nil, err
		}
		writes = append(writes, OwnedEntry{File: link.File, Key: link.Key, Value: value})
	}
	return writes, nil
}

type valueOriginParams struct {
	Job     run.Job
	Origins originContext
}

func valueOrigin(params valueOriginParams) string {
	if params.Job.URL == nil || !params.Origins.names() {
		return ""
	}
	return linkOrigin(linkOriginParams{Job: params.Job, PortName: params.Job.URL.Port, Worktree: params.Origins.Worktree, Project: params.Origins.Project, PublicPort: params.Origins.PublicPort})
}

func jobsByName(cfg run.Config) map[string]run.Job {
	byName := make(map[string]run.Job, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		byName[job.Name] = job
	}
	return byName
}

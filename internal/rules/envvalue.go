package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// envValueToken matches every `{…}` the vocabulary might have left behind, so one
// wtm does not define is refused rather than written to a file as literal braces.
var envValueToken = regexp.MustCompile(`\{[a-zA-Z_][a-zA-Z0-9_.]*\}`)

type ExpandEnvValueParams struct {
	Link domain.EnvValueLink
	// Job is the one the link names, already resolved: this rule reads its
	// namespace, its declared ports and its scope, and asks nothing else.
	Job      domain.JobConfig
	Worktree string
	Ordinal  int
	Offset   int
	// Shared says the job runs once for the repository, which is what keeps its
	// ports unshifted.
	Shared bool
	// Origin is what the job publishes, empty when nothing serves it. Empty is
	// not an error in itself — only a value asking for {origin} makes it one.
	Origin string
}

// ExpandEnvValue resolves one [[env]] link's template for this worktree. Every
// placeholder it does not resolve is refused, so a value either lands complete
// or the run stops naming the line.
func ExpandEnvValue(params ExpandEnvValueParams) (string, error) {
	value, err := expandPortTokens(params)
	if err != nil {
		return "", err
	}

	if strings.Contains(value, domain.EnvValueTokenNamespace) {
		namespace, nsErr := linkNamespace(params)
		if nsErr != nil {
			return "", nsErr
		}
		value = strings.ReplaceAll(value, domain.EnvValueTokenNamespace, namespace)
	}

	if strings.Contains(value, domain.EnvValueTokenOrigin) {
		if params.Origin == "" {
			return "", fmt.Errorf(domain.EnvValueLinkNoOriginFmt, params.Link.Key, params.Link.File, params.Job.Name)
		}
		value = strings.ReplaceAll(value, domain.EnvValueTokenOrigin, params.Origin)
	}

	value = strings.NewReplacer(
		domain.EnvValueTokenWorktree, params.Worktree,
		domain.EnvValueTokenOrdinal, strconv.Itoa(params.Ordinal),
	).Replace(value)

	if leftover := envValueToken.FindString(value); leftover != "" {
		return "", fmt.Errorf(domain.EnvValueUnknownTokenFmt, params.Link.Key, params.Link.File, leftover)
	}
	return value, nil
}

// linkNamespace is the job's slice for this worktree, expanded by the one rule
// that answers it — the same value the attach command reads as $WTM_NAMESPACE.
func linkNamespace(params ExpandEnvValueParams) (string, error) {
	if params.Job.Namespace == nil {
		return "", fmt.Errorf(domain.EnvValueLinkNoNamespaceFmt, params.Link.Key, params.Link.File, params.Job.Name)
	}
	expanded, err := ExpandNamespace(ExpandNamespaceParams{
		Namespace: *params.Job.Namespace,
		Worktree:  params.Worktree,
		Ordinal:   params.Ordinal,
	})
	if err != nil {
		return "", fmt.Errorf(domain.EnvValueLinkBadNamespaceFmt, params.Link.Key, params.Link.File, err)
	}
	return expanded.Name, nil
}

func expandPortTokens(params ExpandEnvValueParams) (string, error) {
	value := params.Link.Value
	for {
		start := strings.Index(value, domain.EnvValueTokenPortPrefix)
		if start < 0 {
			return value, nil
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return "", fmt.Errorf(domain.EnvValueUnclosedTokenFmt, params.Link.Key, params.Link.File)
		}
		end += start

		name := value[start+len(domain.EnvValueTokenPortPrefix) : end]
		base, declared := params.Job.Ports[name]
		if !declared {
			return "", fmt.Errorf(domain.EnvValueLinkNoPortFmt, params.Link.Key, params.Link.File, params.Job.Name, name)
		}
		port := ResolvedPort(ResolvedPortParams{Base: base, Offset: params.Offset, Shared: params.Shared})
		value = value[:start] + strconv.Itoa(port) + value[end+1:]
	}
}

type EnvValueWritesParams struct {
	Config   domain.RunConfig
	Worktree string
	Ordinal  int
	Offset   int
	// Origins is what answers {origin}, and it is the same context [[env_port]]
	// resolves its addresses through — a zero value simply has no address to
	// give, which only a value asking for one turns into a refusal.
	Origins OriginContext
}

// EnvValueWrites resolves every [[env]] link into the key it owns in a
// worktree's .env. They join the identity keys wtm already writes in full, so
// the planning, the reporting and the exclusion from drift are the ones that
// already exist rather than a second pass beside them.
func EnvValueWrites(params EnvValueWritesParams) ([]domain.EnvOwnedEntry, error) {
	if len(params.Config.EnvValues) == 0 {
		return nil, nil
	}

	byName := make(map[string]domain.JobConfig, len(params.Config.Jobs))
	for _, job := range params.Config.Jobs {
		byName[job.Name] = job
	}

	writes := make([]domain.EnvOwnedEntry, 0, len(params.Config.EnvValues))
	for _, link := range params.Config.EnvValues {
		job, found := byName[link.Job]
		if !found {
			return nil, fmt.Errorf(domain.EnvValueLinkNoJobFmt, link.Key, link.File, link.Job)
		}
		value, err := ExpandEnvValue(ExpandEnvValueParams{
			Link:     link,
			Job:      job,
			Worktree: params.Worktree,
			Ordinal:  params.Ordinal,
			Offset:   params.Offset,
			Shared:   IsShared(job),
			Origin:   envValueOrigin(job, params.Origins),
		})
		if err != nil {
			return nil, err
		}
		writes = append(writes, domain.EnvOwnedEntry{File: link.File, Key: link.Key, Value: value})
	}
	return writes, nil
}

// envValueOrigin is where the job answers by name, empty when it publishes none
// or nothing serves names on this machine. A shared service publishes a host
// with no worktree segment, which is what makes one address right for every
// worktree.
func envValueOrigin(job domain.JobConfig, origins OriginContext) string {
	if job.URL == nil || !origins.Names() {
		return ""
	}
	return LinkOrigin(LinkOriginParams{
		Job:        job,
		PortName:   job.URL.Port,
		Worktree:   origins.Worktree,
		Project:    origins.Project,
		PublicPort: origins.PublicPort,
	})
}

// EnvValueOwnedKeys are the keys [[env]] links write in one file, so the
// reconciliation stops calling a value it wrote itself a drift.
func EnvValueOwnedKeys(links []domain.EnvValueLink, file string) map[string]bool {
	if len(links) == 0 {
		return nil
	}
	keys := map[string]bool{}
	for _, link := range links {
		if link.File == file {
			keys[link.Key] = true
		}
	}
	return keys
}

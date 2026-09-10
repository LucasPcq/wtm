package rules

import (
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type EnvValueFieldsParams struct {
	// Shared are the services the scope step marked, carrying the namespace the
	// step before this one settled. A service that carves nothing out is not
	// offered: there would be no slice for a key to name.
	Shared []domain.SharedComposeService
	// Lines are the parsed .env files wtm manages, keyed by target path.
	Lines map[string][]domain.EnvLine
	// Files bounds the candidates to the targets .wtm.toml configures, in the
	// order it declares them.
	Files []domain.EnvFile
	// Existing is run.toml as it stands, which outranks every proposal.
	Existing domain.RunConfig
	// Ports are the port variables each shared service declares, shown as part
	// of the vocabulary while a template is edited.
	Ports map[string][]string
	// Bases are the host ports each shared service declares. A key whose value
	// carries one is where the service answers, not which slice this worktree
	// holds — the [[env_port]] table already speaks for it — so it is offered
	// like every other key but never pre-checked.
	Bases map[string][]int
}

// EnvValueFields is the [[env]] step's rows: every managed .env key, once per
// shared service that carves a slice out. The list is complete rather than
// filtered — wtm has no way to recognize a realm name, so hiding a key would
// hide the only one the reader wanted.
func EnvValueFields(params EnvValueFieldsParams) []domain.EnvValueField {
	jobs := namespacedShared(params.Shared)
	if len(jobs) == 0 {
		return nil
	}

	linked := existingEnvValues(params.Existing)
	addressed := addressedKeys(params.Existing)
	var fields []domain.EnvValueField
	for _, job := range jobs {
		vars := envValueVars(params.Ports[job])
		for _, file := range params.Files {
			for _, key := range envKeysOf(params.Lines[file.Target]) {
				fields = append(fields, envValueField(envValueFieldParams{
					Job: job, File: file.Target, Key: key.Key, Current: key.Value,
					Linked: linked, Vars: vars,
					Addressed: addressed[domain.EnvKeyRef{File: file.Target, Key: key.Key}] ||
						carriesPort(key.Value, params.Bases[job]),
				}))
			}
		}
	}
	return fields
}

type envValueFieldParams struct {
	Job     string
	File    string
	Key     string
	Current string
	Linked  map[domain.EnvKeyRef]domain.EnvValueLink
	Vars    []domain.NamespaceVarGroup
	// Addressed says the key is where the service answers rather than which
	// slice this worktree holds, so the name affinity must not claim it.
	Addressed bool
}

// envValueField reads run.toml first and falls back to affinity: a key whose
// name starts with the service's is the one a project names after it, and that
// is a deduction from the job's own name rather than knowledge of what the
// service is.
func envValueField(params envValueFieldParams) domain.EnvValueField {
	field := domain.EnvValueField{
		Job: params.Job, File: params.File, Key: params.Key,
		Current: params.Current, Value: domain.EnvValueTokenNamespace,
		Vars: params.Vars,
	}
	if held, found := params.Linked[domain.EnvKeyRef{File: params.File, Key: params.Key}]; found {
		field.Linked = held.Job == params.Job
		if field.Linked {
			field.Value = held.Value
		}
		return field
	}
	field.Linked = !params.Addressed && namesAfter(params.Key, params.Job)
	return field
}

// addressedKeys are the keys an [[env_port]] link already writes. Pre-checking
// one would propose a config wtm refuses to read, since a key belongs to one
// table or the other.
func addressedKeys(cfg domain.RunConfig) map[domain.EnvKeyRef]bool {
	addressed := make(map[domain.EnvKeyRef]bool, len(cfg.EnvPorts))
	for _, link := range cfg.EnvPorts {
		addressed[domain.EnvKeyRef{File: link.File, Key: link.Key}] = true
	}
	return addressed
}

// carriesPort is the structural signal, and it works on a first init where no
// link exists yet: a value holding the very port the service binds is its
// address.
func carriesPort(value string, bases []int) bool {
	for _, base := range bases {
		if len(portOffsets(value, base)) > 0 {
			return true
		}
	}
	return false
}

// namesAfter is the whole of wtm's guess: KEYCLOAK_REALM beside a job called
// keycloak. It never reaches a key run.toml already speaks about.
func namesAfter(key, job string) bool {
	prefix := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_", ":", "_").Replace(job))
	return prefix != "" && strings.HasPrefix(strings.ToUpper(key), prefix+"_")
}

func namespacedShared(shared []domain.SharedComposeService) []string {
	var jobs []string
	for _, service := range shared {
		if service.Namespace == nil || service.Namespace.Name == "" {
			continue
		}
		jobs = append(jobs, service.Service)
	}
	sort.Strings(jobs)
	return jobs
}

func existingEnvValues(cfg domain.RunConfig) map[domain.EnvKeyRef]domain.EnvValueLink {
	byRef := make(map[domain.EnvKeyRef]domain.EnvValueLink, len(cfg.EnvValues))
	for _, link := range cfg.EnvValues {
		byRef[domain.EnvKeyRef{File: link.File, Key: link.Key}] = link
	}
	return byRef
}

func envKeysOf(lines []domain.EnvLine) []domain.EnvLine {
	var keys []domain.EnvLine
	for _, line := range lines {
		if line.Kind == domain.EnvLinePair && line.Key != "" {
			keys = append(keys, line)
		}
	}
	return keys
}

// envValueVars is the vocabulary a template may draw on, grouped the way the
// namespace step groups it: what wtm substitutes, then this job's own ports.
func envValueVars(ports []string) []domain.NamespaceVarGroup {
	groups := []domain.NamespaceVarGroup{{
		Label: domain.NamespaceVarSubstituted,
		Vars: []string{
			domain.EnvValueTokenNamespace, domain.EnvValueTokenOrigin,
			domain.NamespaceTokenWorktree, domain.NamespaceTokenOrdinal,
		},
	}}
	if len(ports) == 0 {
		return groups
	}
	named := make([]string, 0, len(ports))
	for _, port := range ports {
		named = append(named, domain.EnvValueTokenPortPrefix+port+"}")
	}
	return append(groups, domain.NamespaceVarGroup{Label: domain.NamespaceVarPorts, Vars: named})
}

// EnvValuesFromFields folds the step's rows back into the links run.toml holds.
// A row left unchecked contributes nothing, which is what withdraws a link the
// config carried.
func EnvValuesFromFields(fields []domain.EnvValueField) []domain.EnvValueLink {
	var links []domain.EnvValueLink
	for _, field := range fields {
		if !field.Linked || field.Value == "" {
			continue
		}
		links = append(links, domain.EnvValueLink{
			File: field.File, Key: field.Key, Job: field.Job, Value: field.Value,
		})
	}
	return links
}

// EnvValueKeyWidth aligns the keys into a column, so the templates beside them
// line up whatever the longest key is.
func EnvValueKeyWidth(fields []domain.EnvValueField) int {
	width := 0
	for _, field := range fields {
		if len(field.Key) > width {
			width = len(field.Key)
		}
	}
	return width
}

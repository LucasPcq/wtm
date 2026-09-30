package rules

import "github.com/LucasPcq/wtm/internal/domain"

type ApplyEnvValuesParams struct {
	Config domain.RunConfig
	// Values is what the [[env]] step answered. Asked says the question was put
	// at all: an empty list from a run that asked withdraws every link it
	// offered, where one that never asked leaves the config exactly as it stands.
	Values []domain.EnvValueLink
	Asked  bool
	// Offered are the keys the step actually showed. A link on a key it never
	// showed — a file .wtm.toml no longer configures, a service no longer shared
	// — survives untouched: a step may only remove what it proposed.
	Offered map[domain.EnvKeyRef]bool
}

// ApplyEnvValues carries the [[env]] step's answers into a configuration that
// already exists, and takes the [[env_port]] link off any key it now writes.
// The two tables are refused on one key at load, so a step that only added the
// new link would write a config wtm then refuses to read.
func ApplyEnvValues(params ApplyEnvValuesParams) domain.RunConfig {
	if !params.Asked {
		return params.Config
	}

	cfg := params.Config
	cfg.EnvValues = mergeEnvValues(params)
	return PruneEnvPortClashes(cfg)
}

// PruneEnvPortClashes drops the [[env_port]] links on keys an [[env]] link
// writes in full. It runs wherever both tables are complete — the init pipeline
// settles the values, then appends more port links, so the last word has to be
// taken after that append rather than in the middle of it.
func PruneEnvPortClashes(cfg domain.RunConfig) domain.RunConfig {
	cfg.EnvPorts = withoutEnvPortsOn(cfg.EnvPorts, cfg.EnvValues)
	return cfg
}

// mergeEnvValues keeps every link the step did not offer and replaces the ones
// it did, so a link written by hand on a file the step never showed survives a
// re-init.
func mergeEnvValues(params ApplyEnvValuesParams) []domain.EnvValueLink {
	var merged []domain.EnvValueLink
	for _, held := range params.Config.EnvValues {
		if params.Offered[domain.EnvKeyRef{File: held.File, Key: held.Key}] {
			continue
		}
		merged = append(merged, held)
	}
	return append(merged, params.Values...)
}

// withoutEnvPortsOn drops the port links on keys an [[env]] link now writes. It
// is a migration and not a preference: an [[env]] value writes its own port
// when it needs one, so the two never had anything to say about one key
// together.
func withoutEnvPortsOn(ports []domain.EnvPortLink, values []domain.EnvValueLink) []domain.EnvPortLink {
	if len(ports) == 0 || len(values) == 0 {
		return ports
	}

	written := map[domain.EnvKeyRef]bool{}
	for _, link := range values {
		written[domain.EnvKeyRef{File: link.File, Key: link.Key}] = true
	}

	kept := make([]domain.EnvPortLink, 0, len(ports))
	for _, link := range ports {
		if written[domain.EnvKeyRef{File: link.File, Key: link.Key}] {
			continue
		}
		kept = append(kept, link)
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

// EnvValuesOffered are the keys a step showed, which is exactly the set it is
// allowed to withdraw a link from.
func EnvValuesOffered(fields []domain.EnvValueField) map[domain.EnvKeyRef]bool {
	offered := make(map[domain.EnvKeyRef]bool, len(fields))
	for _, field := range fields {
		offered[domain.EnvKeyRef{File: field.File, Key: field.Key}] = true
	}
	return offered
}

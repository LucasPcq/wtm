package compose

import (
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type collectNamesParams struct {
	root    *yaml.Node
	lines   []string
	file    string
	project string
}

// collectNames finds every name the file resolves against the Docker daemon
// rather than against the compose project: a service's container_name, and the
// explicit name of a top-level volume or network. COMPOSE_PROJECT_NAME does
// not reach any of them, so two worktrees running the file meet on them.
func collectNames(params collectNamesParams) []domain.ComposeAbsoluteName {
	var names []domain.ComposeAbsoluteName

	for i := 0; i+1 < len(mappingContent(params.root, domain.ComposeServicesKey)); i += 2 {
		services := mappingContent(params.root, domain.ComposeServicesKey)
		owner := services[i].Value
		value := mappingValue(services[i+1], domain.ComposeContainerNameKey)
		if value == nil {
			continue
		}
		names = append(names, nameFor(nameForParams{
			collect: params, kind: domain.ComposeNameContainer, owner: owner, node: value,
		}))
	}

	for _, section := range []struct {
		key  string
		kind domain.ComposeNameKind
	}{
		{domain.ComposeVolumesKey, domain.ComposeNameVolume},
		{domain.ComposeNetworksKey, domain.ComposeNameNetwork},
	} {
		content := mappingContent(params.root, section.key)
		for i := 0; i+1 < len(content); i += 2 {
			owner, spec := content[i].Value, content[i+1]
			value := mappingValue(spec, domain.ComposeNameKey)
			// An external resource lives outside this project's lifecycle:
			// sharing it is the declaration's whole point.
			if value == nil || isExternal(spec) {
				continue
			}
			names = append(names, nameFor(nameForParams{
				collect: params, kind: section.kind, owner: owner, node: value,
			}))
		}
	}

	return names
}

type nameForParams struct {
	collect collectNamesParams
	kind    domain.ComposeNameKind
	owner   string
	node    *yaml.Node
}

func nameFor(params nameForParams) domain.ComposeAbsoluteName {
	name := domain.ComposeAbsoluteName{
		File:   params.collect.file,
		Kind:   params.kind,
		Owner:  params.owner,
		Name:   params.node.Value,
		Line:   params.node.Line,
		Column: params.node.Column,
	}

	switch {
	case params.node.Kind == yaml.AliasNode:
		return unsupportedName(name, domain.ComposeNameReasonAlias)
	case params.node.Anchor != "":
		return unsupportedName(name, domain.ComposeNameReasonAnchor)
	case params.node.Kind != yaml.ScalarNode:
		return unsupportedName(name, domain.ComposeNameReasonUnreadable)
	// A name already reading a variable moves on its own; wtm has nothing to add
	// and would only overwrite a choice the project made.
	case len(rules.EnvVarReferences(params.node.Value)) > 0 || strings.Contains(params.node.Value, "$"):
		name.Status = domain.ComposeNameTemplated
		return name
	case params.collect.project == "":
		return unsupportedName(name, domain.ComposeNameReasonNoProject)
	}

	token, ok := sourceToken(params.collect.lines, params.node)
	if !ok {
		return unsupportedName(name, domain.ComposeNameReasonUnreadable)
	}

	name.Status = domain.ComposeNameAbsolute
	name.Token = token
	name.Replacement = rules.ComposeIsolatedName(rules.ComposeIsolatedNameParams{
		Name:    params.node.Value,
		Project: params.collect.project,
	})
	return name
}

func unsupportedName(name domain.ComposeAbsoluteName, reason string) domain.ComposeAbsoluteName {
	name.Status = domain.ComposeNameUnsupported
	name.Reason = reason
	return name
}

// isExternal reads the two spellings compose accepts: `external: true`, and the
// mapping form that carries the shared name under it.
func isExternal(spec *yaml.Node) bool {
	external := mappingValue(spec, domain.ComposeExternalKey)
	if external == nil {
		return false
	}
	return external.Kind != yaml.ScalarNode || external.Value == "true"
}

func mappingContent(root *yaml.Node, key string) []*yaml.Node {
	section := mappingValue(root, key)
	if section == nil || section.Kind != yaml.MappingNode {
		return nil
	}
	return section.Content
}

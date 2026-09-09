// Package compose reads the `ports:` a docker-compose file declares, and
// rewrites them in place so each worktree binds its own. It is the content side
// of docker-compose support; finding which files exist stays in service/detect.
package compose

import (
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type ScanParams struct {
	ProjectDir string
	// File is the path run.toml uses, relative to ProjectDir.
	File string
	// Project names the repository, the same value COMPOSE_PROJECT_NAME is
	// built from. It becomes the default of every name wtm scopes, so a
	// checkout wtm is not driving keeps the name the file used to pin. Empty
	// leaves the absolute names reported but never rewritten.
	Project string
}

// Scan reads one docker-compose file and locates every port mapping precisely
// enough for Patch to rewrite it. A file it cannot read or parse comes back
// with Err set rather than aborting: the caller reports it and carries on.
func Scan(params ScanParams) domain.ComposeScan {
	scan := domain.ComposeScan{File: params.File}

	content, err := os.ReadFile(filepath.Join(params.ProjectDir, params.File))
	if err != nil {
		scan.Err = err.Error()
		return scan
	}

	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		scan.Err = err.Error()
		return scan
	}
	if len(root.Content) == 0 {
		return scan
	}

	lines := strings.Split(string(content), "\n")
	scan.Bindings = collectBindings(collectParams{
		services: mappingValue(root.Content[0], domain.ComposeServicesKey),
		lines:    lines,
		file:     params.File,
		taken:    referencedNames(&root),
	})
	scan.Services = collectServices(mappingValue(root.Content[0], domain.ComposeServicesKey))
	scan.Names = collectNames(collectNamesParams{
		root:    root.Content[0],
		lines:   lines,
		file:    params.File,
		project: params.Project,
	})
	return scan
}

// referencedNames collects every environment variable the file interpolates,
// anywhere — not just in its ports. A name wtm introduces for a literal port
// becomes a variable it injects into the job, so reusing one the file already
// reads elsewhere (a DATABASE_URL, an environment: entry) would silently
// rewrite a value the project depends on.
func referencedNames(root *yaml.Node) map[string]bool {
	taken := map[string]bool{}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode {
			for _, name := range rules.EnvVarReferences(n.Value) {
				taken[name] = true
			}
		}
		for _, child := range n.Content {
			walk(child)
		}
	}
	walk(root)
	return taken
}

type ScanAllParams struct {
	ProjectDir string
	Files      []string
	Project    string
}

func ScanAll(params ScanAllParams) map[string]domain.ComposeScan {
	if len(params.Files) == 0 {
		return nil
	}
	scans := make(map[string]domain.ComposeScan, len(params.Files))
	for _, f := range params.Files {
		scans[f] = Scan(ScanParams{ProjectDir: params.ProjectDir, File: f, Project: params.Project})
	}
	return scans
}

type collectParams struct {
	services *yaml.Node
	lines    []string
	file     string
	taken    map[string]bool
}

func collectBindings(params collectParams) []domain.ComposePortBinding {
	if params.services == nil || params.services.Kind != yaml.MappingNode {
		return nil
	}

	var bindings []domain.ComposePortBinding
	taken := params.taken
	if taken == nil {
		taken = map[string]bool{}
	}

	for i := 0; i+1 < len(params.services.Content); i += 2 {
		name := params.services.Content[i].Value
		ports := mappingValue(params.services.Content[i+1], domain.ComposePortsKey)
		if ports == nil {
			continue
		}

		if ports.Kind == yaml.AliasNode {
			bindings = append(bindings, unsupported(unsupportedParams{File: params.file, Service: name, Reason: domain.ComposePortReasonAlias}))
			continue
		}
		// An anchored list is read by every service aliasing it, so rewriting it
		// here would move their bindings too.
		if ports.Anchor != "" {
			bindings = append(bindings, unsupported(unsupportedParams{File: params.file, Service: name, Reason: domain.ComposePortReasonAnchor}))
			continue
		}
		if ports.Kind != yaml.SequenceNode {
			continue
		}

		for _, entry := range ports.Content {
			b, ok := bindingFor(bindingForParams{
				file:    params.file,
				service: name,
				entry:   entry,
				lines:   params.lines,
				taken:   taken,
			})
			if !ok {
				continue
			}
			if b.Var != "" {
				taken[b.Var] = true
			}
			bindings = append(bindings, b)
		}
	}

	return bindings
}

type bindingForParams struct {
	file    string
	service string
	entry   *yaml.Node
	lines   []string
	taken   map[string]bool
}

func bindingFor(params bindingForParams) (domain.ComposePortBinding, bool) {
	if params.entry.Anchor != "" {
		return unsupported(unsupportedParams{File: params.file, Service: params.service, Reason: domain.ComposePortReasonAnchor}), true
	}

	switch params.entry.Kind {
	case yaml.AliasNode:
		return unsupported(unsupportedParams{File: params.file, Service: params.service, Reason: domain.ComposePortReasonAlias}), true
	case yaml.ScalarNode:
		return shortBinding(params)
	case yaml.MappingNode:
		return longBinding(params)
	default:
		return domain.ComposePortBinding{}, false
	}
}

func shortBinding(params bindingForParams) (domain.ComposePortBinding, bool) {
	binding := rules.ComposeShortPort(rules.ComposeShortPortParams{
		Service: params.service,
		Mapping: params.entry.Value,
		Taken:   params.taken,
	})
	return locate(locateParams{Binding: binding, File: params.file, Node: params.entry, Lines: params.lines}), true
}

func longBinding(params bindingForParams) (domain.ComposePortBinding, bool) {
	published := mappingValue(params.entry, domain.ComposePublishedKey)
	target := mappingValue(params.entry, domain.ComposeTargetKey)
	if published == nil || published.Kind != yaml.ScalarNode {
		return unsupported(unsupportedParams{File: params.file, Service: params.service, Reason: domain.ComposePortReasonNoHost}), true
	}

	targetValue := ""
	if target != nil {
		targetValue = target.Value
	}

	binding := rules.ComposeLongPort(rules.ComposeLongPortParams{
		Service:   params.service,
		Published: published.Value,
		Target:    targetValue,
		Taken:     params.taken,
	})
	return locate(locateParams{Binding: binding, File: params.file, Node: published, Lines: params.lines}), true
}

// locate pins a binding to the scalar it came from. A token wtm cannot read
// back byte for byte from the source line is downgraded rather than patched
// blind — the position is the only thing the rewrite trusts.
type locateParams struct {
	Binding domain.ComposePortBinding
	File    string
	Node    *yaml.Node
	Lines   []string
}

func locate(params locateParams) domain.ComposePortBinding {
	binding := params.Binding
	binding.File = params.File
	binding.Line = params.Node.Line
	binding.Column = params.Node.Column

	token, ok := sourceToken(params.Lines, params.Node)
	if !ok {
		if binding.Reason != "" {
			return binding
		}
		return unsupported(unsupportedParams{File: params.File, Service: binding.Service, Reason: domain.ComposePortReasonUnreadable})
	}
	binding.Token = token
	return binding
}

func sourceToken(lines []string, node *yaml.Node) (string, bool) {
	if node.Line < 1 || node.Line > len(lines) {
		return "", false
	}
	line := lines[node.Line-1]
	start := node.Column - 1
	if start < 0 || start >= len(line) {
		return "", false
	}

	if quote := line[start]; quote == '"' || quote == '\'' {
		end := start + len(node.Value) + 2
		if end > len(line) || line[end-1] != quote {
			return "", false
		}
		return line[start:end], true
	}

	end := start + len(node.Value)
	if end > len(line) || line[start:end] != node.Value {
		return "", false
	}
	return line[start:end], true
}

type unsupportedParams struct {
	File    string
	Service string
	Reason  string
}

func unsupported(params unsupportedParams) domain.ComposePortBinding {
	return domain.ComposePortBinding{
		File:    params.File,
		Service: params.Service,
		Status:  domain.ComposePortUnsupported,
		Reason:  params.Reason,
	}
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// collectServices reads what each service is, not what it binds: whether it
// comes from a registry image or is built here is the one structural fact that
// separates a service which could be shared from one which never can.
func collectServices(services *yaml.Node) []domain.ComposeService {
	if services == nil || services.Kind != yaml.MappingNode {
		return nil
	}

	var found []domain.ComposeService
	for i := 0; i+1 < len(services.Content); i += 2 {
		body := services.Content[i+1]
		service := domain.ComposeService{Name: services.Content[i].Value}
		if image := mappingValue(body, domain.ComposeImageKey); image != nil {
			service.Image = image.Value
		}
		service.HasBuild = mappingValue(body, domain.ComposeBuildKey) != nil
		found = append(found, service)
	}
	return found
}

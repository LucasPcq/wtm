// Package compose reads a repository's docker compose file, its services, the ports they publish and the names it pins, each with where it is written, and rewrites those values in place.
package compose

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

var variable = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z0-9_]*)`)

// Repository is the working tree of a repository at Dir.
type Repository struct {
	Dir string
}

// Compose reads the docker compose file at the repository's root, the first docker compose itself would pick; none is a Compose with no File. A file that cannot be read is an error; one that does not parse is a Compose saying why (Err).
func (r Repository) Compose() (project.Compose, error) {
	for _, name := range project.ComposeFileNames() {
		data, err := os.ReadFile(filepath.Join(r.Dir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return project.Compose{}, fmt.Errorf("read %s: %w", name, err)
		}
		return parse(name, data), nil
	}

	return project.Compose{}, nil
}

// SaveCompose applies rewrite's edits to the docker compose file in place, the rest of the file byte for byte as it was, and returns what it wrote. A file whose bytes are no longer those rewrite replaces is refused with project.ErrComposeChanged; a symlink is written through, never replaced.
func (r Repository) SaveCompose(ctx context.Context, rewrite project.ComposeRewrite) (domain.Written, error) {
	path := filepath.Join(r.Dir, rewrite.File)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	perm := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	return statefile.Update(ctx, statefile.UpdateParams{
		Path:     path,
		LockPath: statefile.ScratchLockPath(path),
		Perm:     perm,
		Mutate: func(current []byte) (statefile.Change, error) {
			if digest(current) != rewrite.Replaces {
				return nil, project.ErrComposeChanged
			}
			edited, err := Apply(string(current), rewrite.Edits)
			if err != nil {
				return nil, err
			}
			return statefile.Write{Data: []byte(edited)}, nil
		},
	})
}

// Apply returns content with each edit made at its line and column, refusing one whose text is no longer written there.
func Apply(content string, edits []project.ComposeEdit) (string, error) {
	lines := strings.Split(content, "\n")
	sorted := slices.Clone(edits)
	// Right to left within a line, so an edit never moves the column of one still to make.
	slices.SortFunc(sorted, func(a, b project.ComposeEdit) int {
		if a.At.Line != b.At.Line {
			return a.At.Line - b.At.Line
		}
		return b.At.Column - a.At.Column
	})
	for _, edit := range sorted {
		at := edit.At
		start := at.Column - 1
		if at.Line < 1 || at.Line > len(lines) || start < 0 || start+len(at.Text) > len(lines[at.Line-1]) || lines[at.Line-1][start:start+len(at.Text)] != at.Text {
			return "", fmt.Errorf("line %d, column %d no longer reads %s: %w", at.Line, at.Column, at.Text, project.ErrComposeChanged)
		}
		line := lines[at.Line-1]
		lines[at.Line-1] = line[:start] + edit.New + line[start+len(at.Text):]
	}
	return strings.Join(lines, "\n"), nil
}

func parse(file string, data []byte) project.Compose {
	compose := project.Compose{File: file, Digest: digest(data)}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		compose.Err = err.Error()
		return compose
	}
	if len(root.Content) == 0 {
		return compose
	}

	lines := strings.Split(string(data), "\n")
	top := root.Content[0]
	compose.Reads = reads(top)
	for name, body := range pairs(value(top, "services")) {
		service := project.ComposeService{Name: name, Build: value(body, "build") != nil}
		if image := value(body, "image"); image != nil {
			service.Image = image.Value
		}
		service.Ports = ports(value(body, "ports"), lines)
		compose.Services = append(compose.Services, service)
		if pinned := value(body, "container_name"); pinned != nil {
			compose.Names = append(compose.Names, named(project.ComposeContainer, name, pinned, lines))
		}
	}
	for _, section := range []struct {
		key  string
		kind project.ComposeNameKind
	}{{"volumes", project.ComposeVolume}, {"networks", project.ComposeNetwork}} {
		for owner, spec := range pairs(value(top, section.key)) {
			// An external resource is shared on purpose: it lives outside the project.
			if pinned := value(spec, "name"); pinned != nil && !external(spec) {
				compose.Names = append(compose.Names, named(section.kind, owner, pinned, lines))
			}
		}
	}
	return compose
}

func ports(list *yaml.Node, lines []string) []project.ComposePort {
	if list == nil {
		return nil
	}
	if list.Kind != yaml.SequenceNode || list.Anchor != "" {
		return []project.ComposePort{{Unreadable: "anchor or alias"}}
	}

	var found []project.ComposePort
	for _, entry := range list.Content {
		switch {
		case entry.Anchor != "" || entry.Kind == yaml.AliasNode:
			found = append(found, project.ComposePort{Unreadable: "anchor or alias"})
		case entry.Kind == yaml.ScalarNode:
			found = append(found, short(entry, lines))
		case entry.Kind == yaml.MappingNode:
			found = append(found, long(entry, lines))
		}
	}
	return found
}

// short is a port of the short syntax, "[ip:]host:container[/protocol]", its whole scalar the span a rewrite replaces.
func short(entry *yaml.Node, lines []string) project.ComposePort {
	port := project.ComposePort{At: span(entry, lines)}
	if port.At.Text == "" {
		port.Unreadable = "not found in its line"
	}
	fields := splitMapping(entry.Value)
	if len(fields) < 2 {
		port.Container = entry.Value
		return port
	}
	port.IP = strings.Join(fields[:len(fields)-2], ":")
	port.Host, port.Container = fields[len(fields)-2], fields[len(fields)-1]
	return port
}

// long is a port of the long syntax, its published value the span a rewrite replaces.
func long(entry *yaml.Node, lines []string) project.ComposePort {
	port := project.ComposePort{}
	if target := value(entry, "target"); target != nil {
		port.Container = target.Value
	}
	if ip := value(entry, "host_ip"); ip != nil {
		port.IP = ip.Value
	}
	published := value(entry, "published")
	if published == nil || published.Kind != yaml.ScalarNode {
		return port
	}
	port.Host, port.At = published.Value, span(published, lines)
	if port.At.Text == "" {
		port.Unreadable = "not found in its line"
	}
	return port
}

func named(kind project.ComposeNameKind, owner string, node *yaml.Node, lines []string) project.ComposeName {
	name := project.ComposeName{Kind: kind, Owner: owner, Value: node.Value}
	switch {
	case node.Kind != yaml.ScalarNode || node.Anchor != "":
		name.Unreadable = "anchor or alias"
	default:
		if name.At = span(node, lines); name.At.Text == "" {
			name.Unreadable = "not found in its line"
		}
	}
	return name
}

// span is where node's scalar is written, its quotes included; none when its line does not read it back byte for byte, since a rewrite trusts nothing else.
func span(node *yaml.Node, lines []string) project.ComposeSpan {
	if node.Line < 1 || node.Line > len(lines) {
		return project.ComposeSpan{}
	}
	line, start := lines[node.Line-1], node.Column-1
	if start < 0 || start >= len(line) {
		return project.ComposeSpan{}
	}
	end := start + len(node.Value)
	if quote := line[start]; quote == '"' || quote == '\'' {
		end += 2
		if end > len(line) || line[end-1] != quote {
			return project.ComposeSpan{}
		}
	} else if end > len(line) || line[start:end] != node.Value {
		return project.ComposeSpan{}
	}
	return project.ComposeSpan{Line: node.Line, Column: node.Column, Text: line[start:end]}
}

// splitMapping splits a short port at its colons, a variable's own "${A:-1}" and an IPv6 address's brackets kept whole.
func splitMapping(mapping string) []string {
	var fields []string
	depth, from := 0, 0
	for i, r := range mapping {
		switch r {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ':':
			if depth == 0 {
				fields = append(fields, mapping[from:i])
				from = i + 1
			}
		}
	}
	return append(fields, mapping[from:])
}

// reads are the variables node interpolates anywhere, sorted, each once.
func reads(node *yaml.Node) []string {
	var found []string
	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode {
			for _, match := range variable.FindAllStringSubmatch(n.Value, -1) {
				found = append(found, match[1])
			}
		}
		for _, child := range n.Content {
			walk(child)
		}
	}
	walk(node)
	slices.Sort(found)
	return slices.Compact(found)
}

func external(spec *yaml.Node) bool {
	flag := value(spec, "external")
	return flag != nil && (flag.Kind != yaml.ScalarNode || flag.Value == "true")
}

func value(node *yaml.Node, key string) *yaml.Node {
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

// pairs are the keys of a mapping node and their values, in the file's order.
func pairs(node *yaml.Node) func(yield func(string, *yaml.Node) bool) {
	return func(yield func(string, *yaml.Node) bool) {
		if node == nil || node.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			if !yield(node.Content[i].Value, node.Content[i+1]) {
				return
			}
		}
	}
}

// digest is the digest of a file's bytes, "" for no file.
func digest(data []byte) string {
	if data == nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

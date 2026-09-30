package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ComposePortVarNameParams struct {
	Service   string
	Container int
	Taken     map[string]bool
}

// ComposePortVarName is the variable wtm introduces for a frozen mapping:
// <SERVICE>_PORT, then the container port for a service exposing several.
func ComposePortVarName(params ComposePortVarNameParams) string {
	base := normalizeComposeVarSegment(params.Service) + domain.ComposePortVarSuffix
	if !params.Taken[base] {
		return base
	}

	withContainer := fmt.Sprintf("%s_%d", base, params.Container)
	if !params.Taken[withContainer] {
		return withContainer
	}

	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s_%d", withContainer, n)
		if !params.Taken[candidate] {
			return candidate
		}
	}
}

func normalizeComposeVarSegment(service string) string {
	return EnvVarNameFor(service)
}

func collapseUnderscores(s string) string {
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return s
}

type ComposeShortPortParams struct {
	Service string
	// Mapping is the scalar as the file spells it, quotes already stripped:
	// "5432:5432", "127.0.0.1:8025:8025", "${DB_PORT:-5432}:5432".
	Mapping string
	Taken   map[string]bool
}

// ComposeShortPort decides what wtm may do with one short-syntax entry.
// Locating it in the file is the scanner's job.
func ComposeShortPort(params ComposeShortPortParams) domain.ComposePortBinding {
	binding := domain.ComposePortBinding{Service: params.Service}

	fields := splitComposeMapping(params.Mapping)
	if len(fields) < 2 {
		return unsupportedComposePort(binding, domain.ComposePortReasonNoHost)
	}

	// The host port and the container port are always the last two fields;
	// anything before them is the listen address, which an IPv6 literal spreads
	// over several colon-separated pieces ("[::1]", "", "1]").
	ip := strings.Join(fields[:len(fields)-2], ":")
	host, container := fields[len(fields)-2], fields[len(fields)-1]

	containerPort, ok := parseComposeContainerPort(container)
	if !ok {
		return unsupportedComposePort(binding, composeContainerReason(container))
	}
	binding.Container = containerPort

	resolved := resolveComposeHostPort(resolveComposeHostPortParams{
		Service:   params.Service,
		Host:      host,
		Container: containerPort,
		Taken:     params.Taken,
	})
	binding.Var = resolved.varName
	binding.Base = resolved.base
	if resolved.reason != "" {
		return unsupportedComposePort(binding, resolved.reason)
	}

	binding.Status = resolved.status
	if resolved.status == domain.ComposePortFrozen {
		binding.Replacement = renderComposeShortMapping(renderComposeShortMappingParams{
			IP:        ip,
			Var:       resolved.varName,
			Base:      resolved.base,
			Container: container,
		})
	}
	return binding
}

type ComposeLongPortParams struct {
	Service string
	// Published and Target are the two scalars of a long-syntax entry, as the
	// file spells them. Published carries the host port and is the only one wtm
	// ever rewrites.
	Published string
	Target    string
	Taken     map[string]bool
}

// ComposeLongPort is ComposeShortPort for the long syntax, where `published:`
// carries the host port on its own.
func ComposeLongPort(params ComposeLongPortParams) domain.ComposePortBinding {
	binding := domain.ComposePortBinding{Service: params.Service}

	if strings.TrimSpace(params.Published) == "" {
		return unsupportedComposePort(binding, domain.ComposePortReasonNoHost)
	}

	containerPort, ok := parseComposeContainerPort(params.Target)
	if !ok {
		return unsupportedComposePort(binding, composeContainerReason(params.Target))
	}
	binding.Container = containerPort

	resolved := resolveComposeHostPort(resolveComposeHostPortParams{
		Service:   params.Service,
		Host:      params.Published,
		Container: containerPort,
		Taken:     params.Taken,
	})
	binding.Var = resolved.varName
	binding.Base = resolved.base
	if resolved.reason != "" {
		return unsupportedComposePort(binding, resolved.reason)
	}

	binding.Status = resolved.status
	if resolved.status == domain.ComposePortFrozen {
		binding.Replacement = strconv.Quote(fmt.Sprintf(domain.ComposeTemplatedPortFmt, resolved.varName, resolved.base))
	}
	return binding
}

type resolveComposeHostPortParams struct {
	Service   string
	Host      string
	Container int
	Taken     map[string]bool
}

type resolvedComposeHostPort struct {
	status  domain.ComposePortStatus
	varName string
	base    int
	reason  string
}

// resolveComposeHostPort is the one place that decides what a host port field
// is: a variable the project already templated, a literal wtm can template, or
// something it refuses to touch.
func resolveComposeHostPort(params resolveComposeHostPortParams) resolvedComposeHostPort {
	host := strings.TrimSpace(params.Host)

	if strings.Contains(host, "$") {
		name, fallback, ok := parseComposeVarReference(host)
		if !ok {
			return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonNotAPort, host)}
		}
		if !IsEnvVarName(name) {
			return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonBadVarName, name)}
		}

		// No default means the file does not say which port the variable stands
		// for. Inferring one from the container port would let a declaration
		// override whatever the project's .env already sets, silently moving a
		// binding that worked.
		if fallback == "" {
			return resolvedComposeHostPort{
				varName: name,
				base:    params.Container,
				reason:  fmt.Sprintf(domain.ComposePortReasonNoDefault, host),
			}
		}
		base, err := strconv.Atoi(fallback)
		if err != nil {
			return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonNotAPort, fallback)}
		}
		if base < domain.PortMin || base > domain.PortMax {
			return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonOutOfRange, base, domain.PortMin, domain.PortMax)}
		}
		return resolvedComposeHostPort{status: domain.ComposePortTemplated, varName: name, base: base}
	}

	if isComposePortRange(host) {
		return resolvedComposeHostPort{reason: domain.ComposePortReasonRange}
	}

	base, err := strconv.Atoi(host)
	if err != nil {
		return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonNotAPort, host)}
	}
	if base < domain.PortMin || base > domain.PortMax {
		return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonOutOfRange, base, domain.PortMin, domain.PortMax)}
	}

	return resolvedComposeHostPort{
		status: domain.ComposePortFrozen,
		base:   base,
		varName: ComposePortVarName(ComposePortVarNameParams{
			Service:   params.Service,
			Container: params.Container,
			Taken:     params.Taken,
		}),
	}
}

func unsupportedComposePort(binding domain.ComposePortBinding, reason string) domain.ComposePortBinding {
	binding.Status = domain.ComposePortUnsupported
	binding.Reason = reason
	binding.Replacement = ""
	return binding
}

// splitComposeMapping splits on ":" while keeping "${VAR:-default}" whole.
func splitComposeMapping(mapping string) []string {
	var fields []string
	depth, start := 0, 0
	for i := 0; i < len(mapping); i++ {
		switch mapping[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 {
				fields = append(fields, mapping[start:i])
				start = i + 1
			}
		}
	}
	return append(fields, mapping[start:])
}

// parseComposeVarReference reads "${NAME:-default}", "${NAME-default}",
// "${NAME}" or "$NAME", returning the name and the default when there is one.
func parseComposeVarReference(host string) (name, fallback string, ok bool) {
	if !strings.HasPrefix(host, "$") {
		return "", "", false
	}
	body := host[1:]

	if !strings.HasPrefix(body, "{") {
		return body, "", body != ""
	}
	if !strings.HasSuffix(body, "}") {
		return "", "", false
	}
	body = body[1 : len(body)-1]

	if n, f, found := strings.Cut(body, ":-"); found {
		return n, f, true
	}
	if n, f, found := strings.Cut(body, "-"); found {
		return n, f, true
	}
	return body, "", body != ""
}

// parseComposeContainerPort reads the container side, tolerating the "/udp"
// suffix compose allows there.
func parseComposeContainerPort(field string) (int, bool) {
	field = strings.TrimSpace(field)
	if proto := strings.IndexByte(field, '/'); proto >= 0 {
		field = field[:proto]
	}
	port, err := strconv.Atoi(field)
	if err != nil || port < domain.PortMin || port > domain.PortMax {
		return 0, false
	}
	return port, true
}

func composeContainerReason(field string) string {
	if isComposePortRange(field) {
		return domain.ComposePortReasonRange
	}
	return fmt.Sprintf(domain.ComposePortReasonNotAPort, field)
}

func isComposePortRange(field string) bool {
	low, high, found := strings.Cut(strings.TrimSpace(field), "-")
	if !found {
		return false
	}
	if proto := strings.IndexByte(high, '/'); proto >= 0 {
		high = high[:proto]
	}
	_, lowErr := strconv.Atoi(low)
	_, highErr := strconv.Atoi(high)
	return lowErr == nil && highErr == nil
}

type renderComposeShortMappingParams struct {
	IP        string
	Var       string
	Base      int
	Container string
}

// renderComposeShortMapping rewrites a whole mapping rather than splicing
// inside it: the host IP and the container side are carried over verbatim, and
// the result is always quoted, which is valid whatever the source style was.
func renderComposeShortMapping(params renderComposeShortMappingParams) string {
	host := fmt.Sprintf(domain.ComposeTemplatedPortFmt, params.Var, params.Base)
	mapping := host + ":" + params.Container
	if params.IP != "" {
		mapping = params.IP + ":" + mapping
	}
	return strconv.Quote(mapping)
}

// ComposePortEdits reduces the bindings to the splice the patcher performs.
func ComposePortEdits(bindings []domain.ComposePortBinding) []domain.ComposeEdit {
	edits := make([]domain.ComposeEdit, 0, len(bindings))
	for _, b := range bindings {
		if b.Replacement == "" {
			continue
		}
		edits = append(edits, domain.ComposeEdit{
			File: b.File, Line: b.Line, Column: b.Column,
			Token: b.Token, Replacement: b.Replacement,
		})
	}
	return edits
}

type ApplyComposeEditsParams struct {
	Content string
	Edits   []domain.ComposeEdit
}

// ApplyComposeEdits splices each edit in place, addressing it by the line and
// column the scan recorded. The file is never re-serialized, so comments,
// indentation, anchors and quoting style survive untouched. A token that is no
// longer where it was scanned aborts the whole patch rather than overwriting
// whatever took its place.
func ApplyComposeEdits(params ApplyComposeEditsParams) (string, error) {
	if len(params.Edits) == 0 {
		return params.Content, nil
	}

	targets := make([]domain.ComposeEdit, len(params.Edits))
	copy(targets, params.Edits)

	// Descending, so splicing one token never shifts the next one's column.
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Line != targets[j].Line {
			return targets[i].Line > targets[j].Line
		}
		return targets[i].Column > targets[j].Column
	})

	lines := strings.Split(params.Content, "\n")
	for _, e := range targets {
		if e.Line < 1 || e.Line > len(lines) {
			return "", fmt.Errorf(domain.ComposePatchOutOfRangeFmt, e.File, e.Line)
		}

		line := lines[e.Line-1]
		start := e.Column - 1
		end := start + len(e.Token)
		if start < 0 || end > len(line) || line[start:end] != e.Token {
			return "", fmt.Errorf(domain.ComposePatchMovedFmt, e.File, e.Line, e.Token, composeFoundAt(line, start, len(e.Token)))
		}

		lines[e.Line-1] = line[:start] + e.Replacement + line[end:]
	}

	return strings.Join(lines, "\n"), nil
}

func composeFoundAt(line string, start, length int) string {
	if start < 0 || start >= len(line) {
		return "end of line"
	}
	end := start + length
	if end > len(line) {
		end = len(line)
	}
	return strconv.Quote(line[start:end])
}

// SortedComposeFiles keeps a run's output and its rewrites in the same order
// twice running.
func SortedComposeFiles[T any](byFile map[string]T) []string { return sortedKeys(byFile) }

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var envVarReferencePattern = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z0-9_]*)`)

// EnvVarReferences lists the environment variables a string interpolates. The
// scanner seeds a compose file's taken names with every reference in it, so a
// name wtm introduces for a literal port cannot land on one the file already
// uses somewhere else — which would make wtm's own injection rewrite a value
// the project relies on.
func EnvVarReferences(s string) []string {
	matches := envVarReferencePattern.FindAllStringSubmatch(s, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m[1])
	}
	return names
}

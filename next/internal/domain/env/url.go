package env

import (
	"slices"
	"strconv"
	"strings"
)

// The schemes an origin of a job may use.
const (
	schemeHTTP      = "http"
	schemeHTTPS     = "https"
	schemeSeparator = "://"
)

// splitOrigin returns the scheme and the host:port of an http or https url, and reports whether element is one with a host.
func splitOrigin(element string) (string, string, bool) {
	scheme, rest, found := strings.Cut(element, schemeSeparator)
	if !found || (scheme != schemeHTTP && scheme != schemeHTTPS) {
		return "", "", false
	}

	_, hostPort, _ := splitAuthority(rest)
	if hostPort == "" {
		return "", "", false
	}
	return scheme, hostPort, true
}

// splitAuthority splits what follows a url's scheme into its userinfo with its trailing @, its host:port, and the path, query and fragment after them.
func splitAuthority(rest string) (string, string, string) {
	tail := ""
	if end := strings.IndexAny(rest, "/?#"); end >= 0 {
		rest, tail = rest[:end], rest[end:]
	}
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		return rest[:at+1], rest[at+1:], tail
	}
	return "", rest, tail
}

// replaceAuthority returns the url element with its scheme and host:port replaced by those of origin, its userinfo, path, query and fragment kept.
func replaceAuthority(element, origin string) string {
	_, rest, _ := strings.Cut(element, schemeSeparator)
	userinfo, _, tail := splitAuthority(rest)
	if userinfo == "" {
		return origin + tail
	}

	scheme, host, _ := strings.Cut(origin, schemeSeparator)
	return scheme + schemeSeparator + userinfo + host + tail
}

// splitHostPort returns the host and the port of authority, the port zero when it has none or none that reads as a number; an IPv6 host in brackets keeps its colons.
func splitHostPort(authority string) (string, int) {
	at := strings.LastIndex(authority, ":")
	if at < 0 || strings.Contains(authority[at:], "]") {
		return authority, 0
	}

	port, err := strconv.Atoi(authority[at+1:])
	if err != nil {
		return authority, 0
	}
	return authority[:at], port
}

// isLoopbackHost reports whether host names this machine: localhost, 127.0.0.1 or [::1].
func isLoopbackHost(host string) bool {
	return host == proxyTLD || host == "127.0.0.1" || host == "[::1]"
}

// A listElement is one element of a comma-separated value, trimmed, with the spacing around it kept so the list can be rebuilt byte for byte.
type listElement struct {
	lead  string
	value string
	trail string
}

// splitList splits a comma-separated value into its elements.
func splitList(value string) []listElement {
	parts := strings.Split(value, ",")
	elements := make([]listElement, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		lead := part[:strings.Index(part, trimmed)]
		if trimmed == "" {
			lead = part
		}
		elements = append(elements, listElement{lead: lead, value: trimmed, trail: part[len(lead)+len(trimmed):]})
	}
	return elements
}

// joinList rebuilds the comma-separated value of elements, their spacing included.
func joinList(elements []listElement) string {
	parts := make([]string, 0, len(elements))
	for _, e := range elements {
		parts = append(parts, e.lead+e.value+e.trail)
	}
	return strings.Join(parts, ",")
}

// hasLoopbackOrigin reports whether value holds an http url to this machine.
func hasLoopbackOrigin(value string) bool {
	for _, element := range splitList(value) {
		if _, authority, ok := splitOrigin(element.value); ok {
			if host, _ := splitHostPort(authority); isLoopbackHost(host) {
				return true
			}
		}
	}
	return false
}

// secretSuffixes end the names of the parameters that carry a secret, whose values WithoutCredentials masks.
var secretSuffixes = []string{"password", "passwd", "pwd", "secret", "token", "key"}

// paramStarts are what a parameter's name follows, beside the start of a value: a query string, a fragment, a connection string's ; and a libpq DSN's spaces; paramEnds end its value.
const (
	paramStarts = "?&#; \t"
	paramEnds   = "&#;, \t"
)

// WithoutCredentials returns value with the user and password of each url in it masked, and the value of each parameter whose name says it holds a secret (a query string's, a fragment's, a connection string's, a DSN's): what wtm writes is the port or the address, never the credentials beside them.
func WithoutCredentials(value string) string {
	masked, rest := "", value
	for {
		at := strings.Index(rest, schemeSeparator)
		if at < 0 {
			return maskedParams(masked + rest)
		}
		masked, rest = masked+rest[:at+len(schemeSeparator)], rest[at+len(schemeSeparator):]
		// a password may hold / ? # or a comma: its url's userinfo ends at the last @ before the next url or a space
		end := len(rest)
		if next := strings.Index(rest, schemeSeparator); next >= 0 {
			end = next
		}
		if space := strings.IndexAny(rest[:end], " \t"); space >= 0 {
			end = space
		}
		if user := strings.LastIndex(rest[:end], "@"); user >= 0 {
			masked, rest = masked+"•••", rest[user:]
		}
	}
}

// maskedParams returns value with the value of each parameter whose name ends as a secret's masked, wherever its name starts a parameter.
func maskedParams(value string) string {
	var masked strings.Builder
	last := 0
	for i := 0; i < len(value); i++ {
		if value[i] != '=' {
			continue
		}
		start := i
		for start > 0 && isNameByte(value[start-1]) {
			start--
		}
		if start == i || start > 0 && !strings.ContainsRune(paramStarts, rune(value[start-1])) || !isSecretName(value[start:i]) {
			continue
		}
		end := i + 1 + strings.IndexAny(value[i+1:]+"&", paramEnds)
		masked.WriteString(value[last:i+1] + "•••")
		last, i = end, end-1
	}
	return masked.String() + value[last:]
}

// isNameByte reports whether b may be part of a parameter's name: ASCII alone, so lowering a name keeps its length.
func isNameByte(b byte) bool {
	return 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' || '0' <= b && b <= '9' || b == '_' || b == '-' || b == '.'
}

// isSecretName reports whether name ends as the name of a secret, whatever its case.
func isSecretName(name string) bool {
	lower := strings.ToLower(name)
	return slices.ContainsFunc(secretSuffixes, func(suffix string) bool { return strings.HasSuffix(lower, suffix) })
}

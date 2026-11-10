package env

import (
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

// passwordParams are the query parameters that carry a password, whose values WithoutCredentials masks.
var passwordParams = []string{"password", "passwd", "pwd", "secret", "token"}

// WithoutCredentials returns value with the user and password of each url in it masked, and the value of each query parameter that carries a password: what wtm writes is the port or the address, never the credentials beside them.
func WithoutCredentials(value string) string {
	masked, rest := "", value
	for {
		at := strings.Index(rest, schemeSeparator)
		if at < 0 {
			return maskedParams(masked + rest)
		}
		masked, rest = masked+rest[:at+len(schemeSeparator)], rest[at+len(schemeSeparator):]
		end := strings.IndexAny(rest, "/?#, ")
		if end < 0 {
			end = len(rest)
		}
		if user := strings.LastIndex(rest[:end], "@"); user >= 0 {
			masked, rest = masked+"•••", rest[user:]
		}
	}
}

// maskedParams returns value with the value of each password parameter of its query strings masked.
func maskedParams(value string) string {
	for _, name := range passwordParams {
		for _, sep := range []string{"?", "&"} {
			key := sep + name + "="
			for from := 0; ; {
				at := strings.Index(strings.ToLower(value[from:]), key)
				if at < 0 {
					break
				}
				start := from + at + len(key)
				end := start + strings.IndexAny(value[start:]+"&", "&#, ")
				value, from = value[:start]+"•••"+value[end:], start+len("•••")
			}
		}
	}
	return value
}

package env

import (
	"strconv"
	"strings"
)

const (
	schemeHTTP      = "http"
	schemeHTTPS     = "https"
	schemeSeparator = "://"
)

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

func replaceAuthority(element, origin string) string {
	_, rest, _ := strings.Cut(element, schemeSeparator)
	userinfo, _, tail := splitAuthority(rest)
	if userinfo == "" {
		return origin + tail
	}
	scheme, host, _ := strings.Cut(origin, schemeSeparator)
	return scheme + schemeSeparator + userinfo + host + tail
}

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

func isLoopbackHost(host string) bool {
	return host == proxyTLD || host == "127.0.0.1" || host == "[::1]"
}

type listElement struct {
	lead  string
	value string
	trail string
}

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

func joinList(elements []listElement) string {
	parts := make([]string, 0, len(elements))
	for _, e := range elements {
		parts = append(parts, e.lead+e.value+e.trail)
	}
	return strings.Join(parts, ",")
}

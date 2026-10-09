package rules

import (
	"net"
	"regexp"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type EnvOriginMovesParams struct {
	From string
	To   string
}

// EnvOriginMoves reads, from two spellings of one value, the host:port pairs
// that differ — the only part of a value wtm writes. It is an allow-list: a
// report prints what it returns and nothing else of the value, so the rest
// (scheme, user, password, path, query) never needs a parser to find it.
//
// The two values are cut into the same tokens and compared position by
// position. Every token that differs has to be a port, or the host right before
// one, or the reading fails and nothing is shown: a password that changed is a
// difference wtm did not make. A host is shown only when it names this machine.
func EnvOriginMoves(params EnvOriginMovesParams) ([]domain.EnvOriginMove, bool) {
	from, to := originTokens(params.From), originTokens(params.To)
	if len(from) != len(to) {
		return nil, false
	}

	var ports []int
	for i := range from {
		if from[i] == to[i] {
			continue
		}
		port, ok := movedPortAt(from, to, i)
		if !ok {
			return nil, false
		}
		if len(ports) == 0 || ports[len(ports)-1] != port {
			ports = append(ports, port)
		}
	}

	if len(ports) == 0 {
		return nil, false
	}
	moves := make([]domain.EnvOriginMove, 0, len(ports))
	for _, port := range ports {
		moves = append(moves, domain.EnvOriginMove{From: originAt(from, port), To: originAt(to, port)})
	}
	return moves, true
}

// movedPortAt is the port a differing token belongs to: the token itself, or
// the one after the ":" when it is the host.
func movedPortAt(from, to []string, i int) (int, bool) {
	if isPortToken(from, i) && isPortToken(to, i) {
		return i, true
	}
	port := i + 2
	if port >= len(from) || from[i+1] != ":" || to[i+1] != ":" {
		return 0, false
	}
	return port, isPortToken(from, port) && isPortToken(to, port)
}

// isPortToken says a number stands where a port does: the whole value, after
// a host's ":" (not before an "@", where it is a password), or after "port=".
func isPortToken(tokens []string, i int) bool {
	if !isDigits(tokens[i]) {
		return false
	}
	if len(tokens) == 1 {
		return true
	}
	if i > 0 && tokens[i-1] == ":" {
		return i+1 >= len(tokens) || tokens[i+1] != domain.EnvCredentialsSeparator
	}
	return followsPortKey(tokens, i)
}

func followsPortKey(tokens []string, i int) bool {
	j := skipSpacesBack(tokens, i-1)
	if j < 0 || tokens[j] != "=" {
		return false
	}
	j = skipSpacesBack(tokens, j-1)
	return j >= 0 && strings.EqualFold(tokens[j], domain.EnvPortKeyword)
}

func skipSpacesBack(tokens []string, i int) int {
	for i >= 0 && strings.TrimSpace(tokens[i]) == "" {
		i--
	}
	return i
}

func originAt(tokens []string, port int) string {
	if len(tokens) == 1 {
		return tokens[port]
	}
	if port >= 2 && tokens[port-1] == ":" && isLocalHost(tokens[port-2]) {
		return tokens[port-2] + ":" + tokens[port]
	}
	return ":" + tokens[port]
}

// isLocalHost is a host a report may print: one naming this machine, a route
// the proxy serves included.
func isLocalHost(host string) bool {
	return isLoopbackHost(host) || strings.HasSuffix(host, "."+domain.ProxyTLD)
}

// originTokens cuts a value into runs of host characters, bracketed IPv6
// addresses, and single bytes for everything else.
func originTokens(value string) []string {
	var tokens []string
	for i := 0; i < len(value); {
		end := i + 1
		switch {
		case isHostByte(value[i]):
			for end < len(value) && isHostByte(value[end]) {
				end++
			}
		case value[i] == '[':
			if closing := strings.IndexByte(value[i:], ']'); closing > 0 {
				end = i + closing + 1
			}
		}
		tokens = append(tokens, value[i:end])
		i = end
	}
	return tokens
}

func isHostByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == '_'
}

func isDigits(token string) bool {
	if token == "" {
		return false
	}
	for i := 0; i < len(token); i++ {
		if token[i] < '0' || token[i] > '9' {
			return false
		}
	}
	return true
}

func originSide(moves []domain.EnvOriginMove, from bool) []string {
	side := make([]string, 0, len(moves))
	for _, move := range moves {
		if from {
			side = append(side, move.From)
			continue
		}
		side = append(side, move.To)
	}
	return side
}

var dnsName = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*$`)

type printableForeignHostParams struct {
	// Value is the whole value the authority was read from.
	Value     string
	Authority string
}

// printableForeignHost is the authority a refusal may name, empty when it
// could be anything but a host: an "@" anywhere in the value means two parsers
// can disagree on where the credentials end.
func printableForeignHost(params printableForeignHostParams) string {
	if strings.Contains(params.Value, domain.EnvCredentialsSeparator) {
		return ""
	}
	host, port := splitHostPort(params.Authority)
	if port == 0 && strings.Contains(params.Authority, ":") && !strings.HasPrefix(params.Authority, "[") {
		return ""
	}
	if dnsName.MatchString(host) || net.ParseIP(strings.Trim(host, "[]")) != nil {
		return params.Authority
	}
	return ""
}

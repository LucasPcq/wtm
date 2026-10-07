package rules

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/internal/domain"
)

type EnvManagedKeysParams struct {
	Plan   domain.EnvPortPlan
	Target string
}

// envPortLinkedKeys are the keys of one file a [[env_port]] link rewrites.
func envPortLinkedKeys(params EnvManagedKeysParams) map[string]bool {
	linked := map[string]bool{}
	for _, entry := range params.Plan.Entries {
		if entry.File == params.Target {
			linked[entry.Key] = true
		}
	}
	return linked
}

// EnvManagedKeys are the keys of one file whose value wtm writes itself — a
// port link, an owned value — and is therefore free to print. Every other
// value is the user's, secrets included.
func EnvManagedKeys(params EnvManagedKeysParams) map[string]bool {
	managed := map[string]bool{}
	for _, key := range domain.WtmOwnedEnvKeys {
		managed[key] = true
	}
	for key := range envPortLinkedKeys(params) {
		managed[key] = true
	}
	for _, entry := range params.Plan.Owned {
		if entry.File == params.Target {
			managed[entry.Key] = true
		}
	}
	return managed
}

// RedactEnvResult withholds the values of the keys wtm does not write, and
// masks the passwords in those it does, so a report piped into a log or an agent's
// context never carries a secret. It returns a copy: the classification of a
// report runs on the values.
func RedactEnvResult(result domain.EnvSyncResult) domain.EnvSyncResult {
	files := slices.Clone(result.Files)
	for i, file := range files {
		managed := EnvManagedKeys(EnvManagedKeysParams{Plan: result.Ports, Target: file.Target})
		entries := slices.Clone(file.Diff.Entries)
		for j, entry := range entries {
			entries[j] = redactEnvKey(redactEnvKeyParams{Entry: entry, Managed: managed[entry.Key]})
		}
		files[i].Diff.Entries = entries
	}
	result.Files = files
	result.Ports = RedactEnvPortPlan(result.Ports)
	result.Restored = redactEnvRestored(result.Restored)
	return result
}

type redactEnvKeyParams struct {
	Entry   domain.EnvKeyDiff
	Managed bool
}

// redactEnvKey masks a managed key's values rather than trusting them: the
// current one is read before wtm writes its own, so it may be the user's.
func redactEnvKey(params redactEnvKeyParams) domain.EnvKeyDiff {
	entry := params.Entry
	if params.Managed {
		entry.CurrentValue = MaskURLPassword(entry.CurrentValue)
		entry.ResolvedValue = MaskURLPassword(entry.ResolvedValue)
		return entry
	}
	if entry.CurrentValue == "" && entry.ResolvedValue == "" {
		return entry
	}
	entry.CurrentValue = ""
	entry.ResolvedValue = ""
	entry.Redacted = true
	return entry
}

func redactEnvRestored(entries []domain.EnvRestoredEntry) []domain.EnvRestoredEntry {
	masked := slices.Clone(entries)
	for i, entry := range masked {
		masked[i].From = MaskURLPassword(entry.From)
		masked[i].To = MaskURLPassword(entry.To)
	}
	return masked
}

// RedactEnvPortPlan masks the password of every port-linked URL a plan
// reports, the one secret a value wtm rewrites can hold.
func RedactEnvPortPlan(plan domain.EnvPortPlan) domain.EnvPortPlan {
	entries := slices.Clone(plan.Entries)
	for i, entry := range entries {
		entries[i].CurrentValue = MaskURLPassword(entry.CurrentValue)
		entries[i].NewValue = MaskURLPassword(entry.NewValue)
	}
	plan.Entries = entries
	return plan
}

// MaskURLPassword masks every password a value carries — a URL's, one per
// element of a comma-separated list, a password= pair — and returns the rest
// byte for byte. Where parsers disagree on where a password ends, it masks up
// to the farthest of their readings: a report masks too much rather than print
// a secret.
func MaskURLPassword(value string) string {
	parts := urlListParts(value)
	for i, part := range parts {
		parts[i] = maskURLUserinfo(part)
	}
	return maskPasswordPairs(strings.Join(parts, domain.OriginListSeparator))
}

// urlScheme starts a URL, a jdbc one ("jdbc:mysql://") included.
var (
	urlScheme         = regexp.MustCompile(`^\s*[A-Za-z][A-Za-z0-9+.:-]*://`)
	embeddedURLScheme = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.:-]*://`)
)

// urlListParts splits a list only before an element that starts a URL of its
// own, so a comma inside a password stays in its URL.
func urlListParts(value string) []string {
	var parts []string
	for i, part := range strings.Split(value, domain.OriginListSeparator) {
		if i > 0 && !urlScheme.MatchString(part) {
			parts[len(parts)-1] += domain.OriginListSeparator + part
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

// maskURLUserinfo reads a userinfo net/url finds up to the path rather than to
// a "?" or "#": the host a client connects to follows the last "@" before it.
// Past the authority, an "@" cannot be told from a password net/url read as a
// path ("app:12/x@h"), so a URL without userinfo is masked up to its last one.
// A value that does not start with a scheme is a credential up to its last "@"
// ("app:pw@tcp(h)/db"), whatever URL it carries further on.
func maskURLUserinfo(value string) string {
	scheme := urlScheme.FindStringIndex(value)
	if scheme == nil {
		return maskUserinfo(maskUserinfoParams{Value: value, End: len(value)})
	}
	start := scheme[1]
	u, err := url.Parse(value)
	if err != nil || u.Opaque != "" || u.User == nil {
		return maskUserinfo(maskUserinfoParams{Value: value, Start: start, End: len(value), MaskUser: true})
	}
	end := len(value)
	if slash := strings.Index(value[start:], "/"); slash >= 0 {
		end = start + slash
	}
	return maskUserinfo(maskUserinfoParams{Value: value[:end], Start: start, End: end}) + maskEmbeddedURL(value[end:])
}

// maskEmbeddedURL masks a URL a path or a query carries ("?next=http://u:pw@h").
func maskEmbeddedURL(value string) string {
	at := embeddedURLScheme.FindStringIndex(value)
	if at == nil {
		return value
	}
	return value[:at[0]] + maskURLUserinfo(value[at[0]:])
}

type maskUserinfoParams struct {
	Value      string
	Start, End int
	// MaskUser masks a userinfo with no ":" whole.
	MaskUser bool
}

func maskUserinfo(params maskUserinfoParams) string {
	authority := params.Value[params.Start:params.End]
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return params.Value
	}
	colon := strings.Index(authority[:at], ":")
	if colon < 0 && !params.MaskUser {
		return params.Value
	}
	return params.Value[:params.Start+colon+1] + domain.MaskedURLPassword + params.Value[params.Start+at:]
}

// passwordKey is a password given as a key=value pair: a libpq DSN, a query
// string, an ADO.NET, ODBC or jdbc property list.
var passwordKey = regexp.MustCompile(`(?i)(?:password|pwd)\s*=\s*`)

func maskPasswordPairs(value string) string {
	var out strings.Builder
	done := 0
	for _, loc := range passwordKey.FindAllStringIndex(value, -1) {
		if loc[0] < done {
			continue
		}
		out.WriteString(value[done:loc[1]])
		out.WriteString(domain.MaskedURLPassword)
		done = loc[1] + passwordValueLength(passwordValueParams{Before: value[:loc[0]], Value: value[loc[1]:]})
	}
	out.WriteString(value[done:])
	return out.String()
}

type passwordValueParams struct {
	// Before is the text ahead of the key: its separator names the format.
	Before string
	Value  string
}

func passwordValueLength(params passwordValueParams) int {
	key := strings.TrimRightFunc(params.Before, isKeyRune)
	spaced := strings.TrimRightFunc(key, unicode.IsSpace)
	switch {
	case strings.HasSuffix(key, "?"), strings.HasSuffix(key, "&"):
		return indexOrLength(params.Value, "&#")
	case strings.HasSuffix(key, ";"):
		return propertyValueLength(params.Value)
	case strings.HasSuffix(spaced, ";"):
		// "a; password=" reads as ADO.NET and as libpq: mask the longer.
		return max(propertyValueLength(params.Value), libpqValueLength(params.Value))
	default:
		return libpqValueLength(params.Value)
	}
}

// propertyQuotes are the openings an ADO.NET or jdbc value may be quoted
// with, and their closings, doubled to escape one.
var propertyQuotes = map[byte]byte{'"': '"', '\'': '\'', '{': '}'}

// propertyValueLength reads an ADO.NET or jdbc value: up to the next ";", or
// to its closing quote.
func propertyValueLength(value string) int {
	if value == "" {
		return 0
	}
	quote, quoted := propertyQuotes[value[0]]
	if !quoted {
		return indexOrLength(value, ";")
	}
	for i := 1; i < len(value); i++ {
		if value[i] != quote {
			continue
		}
		if i+1 < len(value) && value[i+1] == quote {
			i++
			continue
		}
		return i + 1
	}
	return len(value)
}

// libpqValueLength reads a libpq value: up to whitespace, or to its closing
// single quote, a backslash escaping the next character in both.
func libpqValueLength(value string) int {
	quoted := strings.HasPrefix(value, "'")
	for i := 0; i < len(value); i++ {
		switch {
		case value[i] == '\\':
			i++
		case quoted && i > 0 && value[i] == '\'':
			return i + 1
		case !quoted && unicode.IsSpace(rune(value[i])):
			return i
		}
	}
	return len(value)
}

// isKeyRune is a rune of a prefix the password key carries, "ssl" in sslpassword.
func isKeyRune(r rune) bool {
	return r == '_' || r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r))
}

func indexOrLength(value, chars string) int {
	if i := strings.IndexAny(value, chars); i >= 0 {
		return i
	}
	return len(value)
}

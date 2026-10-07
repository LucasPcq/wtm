package rules

import (
	"net/url"
	"regexp"
	"slices"
	"strings"

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

// passwordParam is a password given as a key=value pair: a libpq DSN, a query
// string, a jdbc property list.
var passwordParam = regexp.MustCompile(`(?i)(password=)(?:'[^']*'?|[^\s&;']*)`)

// MaskURLPassword masks every password a value carries — a URL's, one per
// element of a comma-separated list, a password= pair — and returns the rest
// byte for byte. A URL net/url cannot read is masked up to its last "@": a
// report masks too much rather than print a secret.
func MaskURLPassword(value string) string {
	parts := strings.Split(value, domain.OriginListSeparator)
	for i, part := range parts {
		parts[i] = passwordParam.ReplaceAllString(maskURLUserinfo(part), "${1}"+domain.MaskedURLPassword)
	}
	return strings.Join(parts, domain.OriginListSeparator)
}

func maskURLUserinfo(value string) string {
	scheme := strings.Index(value, domain.OriginSchemeSeparator)
	if scheme < 0 {
		return value
	}
	start := scheme + len(domain.OriginSchemeSeparator)
	authority := value[start:]
	if u, err := url.Parse(value); err == nil && u.Opaque == "" {
		if u.User == nil {
			return value
		}
		if _, has := u.User.Password(); !has {
			return value
		}
		if end := strings.IndexAny(authority, "/?#"); end >= 0 {
			authority = authority[:end]
		}
	}
	at := strings.LastIndex(authority, "@")
	colon := strings.Index(authority, ":")
	if at < 0 || colon < 0 || colon > at {
		return value
	}
	return value[:start+colon+1] + domain.MaskedURLPassword + value[start+at:]
}

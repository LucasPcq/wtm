package rules

import (
	"net/url"
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

// RedactEnvResult withholds the values of the keys wtm does not write, and the
// password of a port-linked URL, so a report piped into a log or an agent's
// context never carries a secret. It returns a copy: the classification of a
// report runs on the values.
func RedactEnvResult(result domain.EnvSyncResult) domain.EnvSyncResult {
	files := slices.Clone(result.Files)
	for i, file := range files {
		keys := EnvManagedKeysParams{Plan: result.Ports, Target: file.Target}
		managed, linked := EnvManagedKeys(keys), envPortLinkedKeys(keys)
		entries := slices.Clone(file.Diff.Entries)
		for j, entry := range entries {
			entries[j] = redactEnvKey(redactEnvKeyParams{Entry: entry, Managed: managed[entry.Key], Linked: linked[entry.Key]})
		}
		files[i].Diff.Entries = entries
	}
	result.Files = files
	result.Ports = RedactEnvPortPlan(result.Ports)
	return result
}

type redactEnvKeyParams struct {
	Entry   domain.EnvKeyDiff
	Managed bool
	Linked  bool
}

func redactEnvKey(params redactEnvKeyParams) domain.EnvKeyDiff {
	entry := params.Entry
	if params.Linked {
		entry.CurrentValue = MaskURLPassword(entry.CurrentValue)
		entry.ResolvedValue = MaskURLPassword(entry.ResolvedValue)
		return entry
	}
	if params.Managed || (entry.CurrentValue == "" && entry.ResolvedValue == "") {
		return entry
	}
	entry.CurrentValue = ""
	entry.ResolvedValue = ""
	entry.Redacted = true
	return entry
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

// MaskURLPassword replaces the password of a URL with a mask and returns every
// other value byte for byte: a plain port, a URL without one, a string
// net/url cannot parse. The value is spliced rather than re-serialised, so
// nothing but the password can change.
func MaskURLPassword(value string) string {
	u, err := url.Parse(value)
	if err != nil || u.User == nil {
		return value
	}
	if _, has := u.User.Password(); !has {
		return value
	}
	scheme := strings.Index(value, domain.OriginSchemeSeparator)
	if scheme < 0 {
		return value
	}
	start := scheme + len(domain.OriginSchemeSeparator)
	authority := value[start:]
	if end := strings.IndexAny(authority, "/?#"); end >= 0 {
		authority = authority[:end]
	}
	at := strings.LastIndex(authority, "@")
	colon := strings.Index(authority, ":")
	if at < 0 || colon < 0 || colon > at {
		return value
	}
	return value[:start+colon+1] + domain.MaskedURLPassword + value[start+at:]
}

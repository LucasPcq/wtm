package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

var scriptServiceKeywords = []string{
	domain.ScriptKeyDev,
	domain.ScriptKeyStart,
	domain.ScriptKeyServe,
	domain.ScriptKeyWatch,
}

// StripScope removes an npm scope prefix from a package name.
// "@acme/web" → "web", "my-app" → "my-app".
func StripScope(name string) string {
	if !strings.HasPrefix(name, "@") {
		return name
	}
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return name
}

type PreselectScriptParams struct {
	Script domain.PackageScript
	// All is every script detected in the repo. A root script is only judged
	// against its neighbours: the same name in a workspace package is what tells
	// an orchestrator apart from a job.
	All []domain.PackageScript
}

// PreselectScript says whether `run init` checks a script by default. It is not
// ClassifyScriptKind: that one decides whether a job blocks its profile, this
// one decides whether the job is created at all.
//
// A root script whose name a workspace package also declares is an orchestrator
// — `turbo run dev`, `pnpm -r dev` — and checking it alongside the packages it
// fans out to would start every one of them twice, fighting over the same
// ports. It stays proposed, unchecked: nothing here knows what turbo is, only
// that the repo declares the same script at two levels.
func PreselectScript(params PreselectScriptParams) bool {
	if !strings.Contains(strings.ToLower(params.Script.Name), domain.ScriptPreselectKey) {
		return false
	}
	return !isOrchestrator(params)
}

func isOrchestrator(params PreselectScriptParams) bool {
	if params.Script.Workspace != "" {
		return false
	}
	for _, other := range params.All {
		if other.Workspace != "" && other.Name == params.Script.Name {
			return true
		}
	}
	return false
}

// ClassifyScriptKind returns JobKindService for long-running dev scripts and
// JobKindTask for everything else.
//
// A script name maps to JobKindService when, for any keyword kw in
// {dev, start, serve, watch}, the name satisfies at least one condition:
//   - exact match: name == kw
//   - prefix:      name starts with kw+":"   (e.g. "dev:api")
//   - suffix:      name ends with   ":"+kw   (e.g. "api:dev")
func ClassifyScriptKind(scriptName string) domain.JobKind {
	for _, kw := range scriptServiceKeywords {
		if scriptName == kw {
			return domain.JobKindService
		}
		if strings.HasPrefix(scriptName, kw+":") {
			return domain.JobKindService
		}
		if strings.HasSuffix(scriptName, ":"+kw) {
			return domain.JobKindService
		}
	}
	return domain.JobKindTask
}

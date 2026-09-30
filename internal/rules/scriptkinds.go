package rules

import "github.com/LucasPcq/wtm/internal/domain"

type ApplyScriptKindsParams struct {
	Scripts []domain.PackageScript
	Choices []domain.JobKindChoice
}

// ApplyScriptKinds settles the kind of every script the wizard asked about. A
// script it never asked about is left untouched, keeping the kind its name
// implies.
func ApplyScriptKinds(params ApplyScriptKindsParams) []domain.PackageScript {
	type identity struct{ name, workspace string }

	settled := make(map[identity]domain.JobKind, len(params.Choices))
	for _, choice := range params.Choices {
		settled[identity{choice.Name, choice.Workspace}] = choice.Kind
	}

	scripts := make([]domain.PackageScript, len(params.Scripts))
	copy(scripts, params.Scripts)
	for i, script := range scripts {
		if kind, asked := settled[identity{script.Name, script.Workspace}]; asked {
			scripts[i].Kind = kind
		}
	}
	return scripts
}

// ScriptKindsSkipReason explains a kind question that was never put: either
// nothing was checked, or every checked script is one whose name settles it.
func ScriptKindsSkipReason(checked int) string {
	if checked == 0 {
		return domain.SkipReasonNoScriptChecked
	}
	return domain.SkipReasonKindsSettled
}

// PortsSkipReason explains an unasked ports review: either there is no service
// to shift, or nothing declared a port for the ones there are.
func PortsSkipReason(cfg domain.RunConfig) string {
	for _, job := range cfg.Jobs {
		if job.Kind == domain.JobKindService {
			return domain.SkipReasonNoPortDetected
		}
	}
	return domain.SkipReasonNoService
}

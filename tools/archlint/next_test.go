package main

import "testing"

func TestANextPackageImportsOnlyWhatItsRowAllows(t *testing.T) {
	runTxtar(t, newNextAnalyzer(loadedTests), "next",
		nextModule,
		nextPrefix+"internal/domain/branch",
		nextPrefix+"internal/service/provision",
		nextPrefix+"internal/command/create",
		nextPrefix+"internal/adapter/git",
		nextPrefix+"internal/adapter/gittest",
		nextPrefix+"internal/adapter/store",
		nextPrefix+"internal/ui/output",
		nextPrefix+"internal/ui/cli",
		nextPrefix+"internal/command/create/cli",
		nextPrefix+"internal/command/clean",
		nextPrefix+"cmd/wtm-next",
		nextPrefix+"internal/stray",
	)
}

func TestTheRulesWrittenForInternalLeaveNextAlone(t *testing.T) {
	for _, a := range analyzers {
		if a.Name == typeassertAnalyzer.Name {
			continue
		}
		t.Run(a.Name, func(t *testing.T) {
			runTxtar(t, a, "nextlegacy",
				nextPrefix+"internal/adapter/git",
				nextPrefix+"internal/adapter/shell",
				nextPrefix+"internal/ui/output",
			)
		})
	}
}

func TestALanguageRuleStillActsOnNext(t *testing.T) {
	runTxtar(t, typeassertAnalyzer, "nextlegacy", nextPrefix+"internal/domain/assert")
}

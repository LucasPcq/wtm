package envcopy

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/stretchr/testify/assert"
)

func TestTheTemplateIsThePinnedOneElseTheFirstConventionalSpellingPresent(t *testing.T) {
	guessed := Request{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: ".env"}}, TargetDir: "/wt", MainPath: "/main"}
	pinned := Request{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: ".env", Template: ".env.example"}}, TargetDir: "/wt", MainPath: "/main"}
	present := map[string]bool{"/main/.env.sample": true, "/main/.env.tmpl": true}

	found, _ := sources(t, SourcesParams{Request: guessed, Present: present})
	gone, warnings := sources(t, SourcesParams{Request: pinned, Present: present})

	assert.Equal(t, []Source{{From: "/main/.env.sample", To: "/wt/.env"}}, found)
	assert.Empty(t, gone, "a pinned template that is gone is not replaced by a guess")
	assert.Equal(t, []Warning{NoTemplate{Target: ".env"}}, warnings)
}

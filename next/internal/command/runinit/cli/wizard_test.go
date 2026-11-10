package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheWizardAsksEachPartThenTheRecap(t *testing.T) {
	d := open(t, newWorld(t))
	d.looksLike("jobs")

	for _, step := range []string{"tasks", "runner", "shared", "spaces", "touches", "ports", "commands", "addressing", "urls", "env_copy", "links", "profiles", "default_profile", "patch_compose"} {
		d.Keys("enter")
		d.looksLike(step)
	}
	d.Keys("enter")
	d.looksLike("recap")
}

func TestAJobUncheckedInTheWizardIsNotWritten(t *testing.T) {
	d := open(t, newWorld(t))

	d.Keys("space", "enter")

	assert.NotContains(t, d.s.Request().Jobs, "postgres")
	assert.Contains(t, d.s.Request().Jobs, "redis")
}

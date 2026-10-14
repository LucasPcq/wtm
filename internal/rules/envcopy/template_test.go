package envcopy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules/envcopy"
)

func TestTheTemplateIsThePinnedOneElseTheFirstCandidatePresent(t *testing.T) {
	present := map[string]bool{"/dir/.env.sample": true}

	assert.Equal(t, "/dir/.env.sample", envcopy.Template(envcopy.TemplateCandidates("/dir", domain.EnvFile{Target: ".env"}), present))
	assert.Empty(t, envcopy.Template(envcopy.TemplateCandidates("/dir", domain.EnvFile{Target: ".env", Template: ".env.example"}), present), "a pinned template that is gone is not replaced by a guess")
}

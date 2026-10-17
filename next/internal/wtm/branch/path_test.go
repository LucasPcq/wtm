package branch

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPathSegmentTurnsEachSlashIntoADash(t *testing.T) {
	cases := map[string]string{"feature/login": "feature-login", "fix/auth/step-2": "fix-auth-step-2", "simple": "simple", "a/b/c/d": "a-b-c-d"}
	for name, want := range cases {
		assert.Equal(t, want, PathSegment(name), name)
	}
}

func TestASlugIsWhatDockerAcceptsAsAProjectName(t *testing.T) {
	cases := map[string]string{
		"feat/x":     "feat-x",
		"Feat/Login": "feat-login",
		"feat/a_b":   "feat-a_b",
		"_-feat":     "feat",
		"feat.x":     "feat-x",
		"///":        "wtm",
	}
	for name, want := range cases {
		assert.Equal(t, want, Slug(name), name)
	}
}

func TestAHostLabelIsOneDNSLabel(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"already a label", "web", "web"},
		{"uppercase lowered", "Web-API", "web-api"},
		{"npm scope and slash", "@acme/web", "acme-web"},
		{"no underscore in a host", "app_web", "app-web"},
		{"runs collapsed", "app///web", "app-web"},
		{"edge dashes trimmed", "-web-", "web"},
		{"cut at 63", "a" + strings.Repeat("b", 79), "a" + strings.Repeat("b", 62)},
		{"empty falls back", "///", "wtm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, HostLabel(tc.in))
		})
	}
}

func TestAStateSegmentEscapesTheSlashesAPathSegmentTurnsIntoDashes(t *testing.T) {
	assert.Equal(t, "feat%2Fx", StateSegment("feat/x"))
	assert.Equal(t, "feat-x", StateSegment("feat-x"))
	assert.NotEqual(t, StateSegment("feat/x"), StateSegment("feat-x"))
}

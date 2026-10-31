package run

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
)

func TestTheURLOfAJobIsItsNameOnTheProxysPublicPortElseItsPort(t *testing.T) {
	spec := Spec{Job: project.Job{Name: "web", URL: &project.URL{Port: "PORT"}, Ports: map[string]int{"PORT": 3000}}, Env: map[string]string{"PORT": "3100"}, Routes: []Route{{Job: "web", Host: "web.feat.acme.localhost", Port: "PORT"}}}
	cases := map[string]struct {
		public int
		want   string
	}{
		"off":        {public: 0, want: "http://localhost:3100"},
		"proxy":      {public: 11080, want: "http://web.feat.acme.localhost:11080"},
		"redirected": {public: PrivilegedPort, want: "http://web.feat.acme.localhost"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, spec.URL(c.public))
		})
	}
	spec.Routes = []Route{{Job: "admin", Host: "admin.feat.acme.localhost", Port: "PORT"}}
	assert.Equal(t, "http://localhost:3100", spec.URL(11080), "no route of its own, its port")
}

func TestTheAddressesOfAJobAreItsRoutesWhosePortsItsEnvironmentSpells(t *testing.T) {
	spec := Spec{
		Env:    map[string]string{"PORT": "3100", "ADMIN_PORT": "nope", "WTM_WORKTREE": "feat", "WTM_PROJECT": "acme"},
		Routes: []Route{{Job: "web", Host: "web.feat.acme.localhost", Port: "PORT"}, {Job: "admin", Host: "admin.feat.acme.localhost", Port: "ADMIN_PORT"}, {Job: "docs", Host: "docs.feat.acme.localhost", Port: "DOCS_PORT"}},
	}

	assert.Equal(t, []Address{{Host: "web.feat.acme.localhost", Port: 3100, Job: "web", Worktree: "feat", Project: "acme"}}, spec.Addresses())
}

func TestTheProxysPublicPortIs80OnlyWhenTheRedirectionReachesIt(t *testing.T) {
	assert.Equal(t, 0, PublicPort(0, 0), "off")
	assert.Equal(t, PrivilegedPort, PublicPort(11080, 11080))
	assert.Equal(t, 11080, PublicPort(11080, 0), "no redirection")
	assert.Equal(t, 11080, PublicPort(11080, 11081), "a redirection to another proxy")
}

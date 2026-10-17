package settle

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
)

func TestALinkWritesTheOriginItsJobPublishes(t *testing.T) {
	job := publishedOriginJob("api-dev", "PORT")
	cases := []struct {
		name     string
		job      run.Job
		port     string
		worktree worktree
		want     string
	}{
		{"published job on the ported proxy", job, "PORT", worktree{Slug: "feat-x", Project: "monorepo", PublicPort: 10080}, "http://api-dev.feat-x.monorepo.localhost:10080"},
		{"the privileged port vanishes from the origin", job, "PORT", worktree{Slug: "feat-x", Project: "monorepo", PublicPort: privilegedProxyPort}, "http://api-dev.feat-x.monorepo.localhost"},
		{"an explicit url.host wins over the job name", run.Job{Name: "api-dev", Ports: map[string]int{"PORT": 4001}, URL: &run.URL{Port: "PORT", Host: "api"}}, "PORT", worktree{Slug: "feat-x", Project: "monorepo", PublicPort: 10080}, "http://api.feat-x.monorepo.localhost:10080"},
		{"names and project are made DNS-safe", job, "PORT", worktree{Slug: "feat/Big_Thing", Project: "My Repo", PublicPort: 10080}, "http://api-dev.feat-big-thing.my-repo.localhost:10080"},
		{"a job publishing nothing keeps its port", run.Job{Name: "postgres", Ports: map[string]int{"POSTGRES_PORT": 5432}}, "POSTGRES_PORT", worktree{Slug: "feat-x", Project: "monorepo", PublicPort: 10080}, ""},
		{"a link following a port the url does not publish keeps its port", run.Job{Name: "api-dev", Ports: map[string]int{"PORT": 4001, "METRICS_PORT": 9001}, URL: &run.URL{Port: "PORT"}}, "METRICS_PORT", worktree{Slug: "feat-x", Project: "monorepo", PublicPort: 10080}, ""},
		{"no public port means nothing serves the name", job, "PORT", worktree{Slug: "feat-x", Project: "monorepo"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.worktree.Jobs = map[string]run.Job{tc.job.Name: tc.job}

			assert.Equal(t, tc.want, tc.worktree.linkOrigin(run.EnvPortLink{Job: tc.job.Name, Port: tc.port}))
		})
	}
}

func TestAnOriginReplacesOnlyTheAuthorityOfThisJobsElements(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		want     string
		status   Status
		fallback bool
	}{
		{name: "a local origin becomes the named one", value: "http://localhost:4001", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "the worktree's own port anchors too", value: "http://localhost:4011", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "the loopback address anchors like the name", value: "http://127.0.0.1:4001", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "path and query survive the authority swap", value: "http://localhost:4001/v1/auth?redirect=http%3A%2F%2Fx&p=4001", want: "http://api-dev.feat-x.monorepo.localhost:10080/v1/auth?redirect=http%3A%2F%2Fx&p=4001", status: StatusRewrite},
		{name: "a port sitting in the path is never a candidate", value: "http://localhost:4001/cb?port=4001", want: "http://api-dev.feat-x.monorepo.localhost:10080/cb?port=4001", status: StatusRewrite},
		{name: "the same origin twice is idempotent", value: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusUnchanged},
		{name: "another worktree's route is corrected to this one", value: "http://api-dev.feat-y.monorepo.localhost:10080", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "a route written before the redirection was installed is recalled", value: "http://api-dev.feat-x.monorepo.localhost", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "every local element of a list is translated, the rest left", value: "http://localhost:4001,https://app.example.com", want: "http://api-dev.feat-x.monorepo.localhost:10080,https://app.example.com", status: StatusRewrite},
		{name: "spacing inside a list is preserved", value: "http://localhost:4001, http://localhost:9999", want: "http://api-dev.feat-x.monorepo.localhost:10080, http://localhost:9999", status: StatusRewrite},
		{name: "credentials in the url survive the authority swap", value: "http://u:pw@localhost:4001/api", want: "http://u:pw@api-dev.feat-x.monorepo.localhost:10080/api", status: StatusRewrite},
		{name: "the IPv6 loopback anchors too", value: "http://[::1]:4001", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "https on our own port is refused, never downgraded", value: "https://localhost:4001", status: StatusSecureScheme},
		{name: "https anywhere in a list refuses the whole value", value: "http://localhost:4001,https://localhost:4001", status: StatusSecureScheme},
		{name: "a host no job here serves is reported", value: "https://api.staging.example.com", status: StatusForeignHost},
		{name: "a local url on some other port anchors nothing", value: "http://localhost:9999", status: StatusNotFound},
		{name: "a bare number falls back to the port substitution", value: "4001", fallback: true},
		{name: "a scheme the proxy does not speak falls back too", value: "postgres://user:pw@localhost:4001/db", fallback: true},
		{name: "an empty value falls back", value: "", fallback: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteOrigin(rewriteOriginOf(tc.value))

			assert.Equal(t, tc.fallback, got.Fallback)
			if tc.fallback {
				return
			}
			assert.Equal(t, tc.status, got.Status)
			if got.Status == StatusRewrite {
				assert.Equal(t, tc.want, got.Value)
			}
		})
	}
}

func TestARouteRewindsToItsBasePort(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"a route of this worktree rewinds to the base port", "http://api-dev.feat-x.monorepo.localhost:10080", "http://localhost:4001"},
		{"another worktree's route rewinds to the same thing", "http://api-dev.feat-y.monorepo.localhost:10080", "http://localhost:4001"},
		{"a portless route rewinds too", "http://api-dev.feat-x.monorepo.localhost/v1", "http://localhost:4001/v1"},
		{"a value already on a port is left for the port reduction", "http://localhost:4011", "http://localhost:4011"},
		{"another job's route is not ours to rewind", "http://web-dev.feat-x.monorepo.localhost:10080", "http://web-dev.feat-x.monorepo.localhost:10080"},
		{"another project's route is not ours either", "http://api-dev.feat-x.other.localhost:10080", "http://api-dev.feat-x.other.localhost:10080"},
		{"each element of a list is rewound", "http://api-dev.feat-x.monorepo.localhost:10080,https://app.example.com", "http://localhost:4001,https://app.example.com"},
		{"credentials survive the rewind", "http://u:pw@api-dev.feat-x.monorepo.localhost:10080/api", "http://u:pw@localhost:4001/api"},
		{"a bare number is untouched", "4011", "4011"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, reduceOriginValue(reduceOriginParams{Value: tc.value, JobLabel: "api-dev", Project: "monorepo", Base: 4001}))
		})
	}
}

func TestThreeSpellingsOfOneSettingReduceEqual(t *testing.T) {
	reduce := func(value string) string {
		return reducePortValue(reducePortParams{Value: value, Base: 4001, Block: 10, JobLabel: "api-dev", Project: "monorepo"})
	}

	main := reduce("http://localhost:4001")

	assert.Equal(t, main, reduce("http://api-dev.feat-x.monorepo.localhost:10080"))
	assert.Equal(t, main, reduce("http://localhost:4011"))
}

func TestARewriteNamesTheForeignHostItFound(t *testing.T) {
	got := rewriteOrigin(rewriteOriginOf("https://api.staging.example.com/v1"))

	assert.Equal(t, StatusForeignHost, got.Status)
	assert.Equal(t, "api.staging.example.com", got.ForeignHost)
}

func TestNoReportNamesAForeignHost(t *testing.T) {
	for _, value := range []string{
		"https://u:pw@api.staging.example.com/v1",
		"http://user:hunter2#x@api.staging.example.com/v1",
		"http://user:hunter2?x@api.staging.example.com/v1",
	} {
		t.Run(value, func(t *testing.T) {
			got := rewriteOrigin(rewriteOriginOf(value))
			plan := Plan{Entries: []Entry{{File: ".env", Key: "API", Port: "P", Status: got.Status, ForeignHost: got.ForeignHost}}}

			redacted := Redact(plan)

			assert.Equal(t, StatusForeignHost, got.Status)
			assert.Empty(t, redacted.Entries[0].ForeignHost)
		})
	}
}

func TestAJobIsPublishedUnderItsJobWorktreeAndProject(t *testing.T) {
	published := func(name, host string) run.Job {
		return run.Job{Name: name, Ports: map[string]int{"PORT": 3000}, URL: &run.URL{Port: "PORT", Host: host}}
	}
	cases := []struct {
		name     string
		job      run.Job
		worktree string
		project  string
		want     string
	}{
		{"the job name by default", published("app1-web", ""), "feat-auth", "myapp", "app1-web.feat-auth.myapp.localhost"},
		{"a declared host", published("app1-api", "api.app-1"), "feat-auth", "myapp", "api.app-1.feat-auth.myapp.localhost"},
		{"segments made DNS-safe", published("App_Web", ""), "feat/auth", "My.App", "app-web.feat-auth.my-app.localhost"},
		{"a job without a url", run.Job{Name: "db"}, "feat-auth", "myapp", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, worktree{Slug: tc.worktree, Project: tc.project}.routeHost(tc.job))
		})
	}
}

func TestTwoWorktreesNeverShareACookieParent(t *testing.T) {
	job := run.Job{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &run.URL{Port: "PORT"}}

	a := worktree{Slug: "feat-auth", Project: "myapp"}.routeHost(job)
	b := worktree{Slug: "main", Project: "myapp"}.routeHost(job)

	assert.NotEqual(t, a, b)
	assert.NotEqual(t, cookieParent(a), cookieParent(b))
}

func TestAddressingIsNamesUnlessPortsAreAskedFor(t *testing.T) {
	cases := []struct {
		name   string
		config run.Config
		want   run.Addressing
	}{
		{"unset defaults to names", run.Config{}, run.AddressNames},
		{"names", run.Config{Addressing: run.AddressNames}, run.AddressNames},
		{"ports opts out", run.Config{Addressing: run.AddressPorts}, run.AddressPorts},
		{"unknown falls back to the default", run.Config{Addressing: "nope"}, run.AddressNames},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, effectiveAddressing(tc.config))
		})
	}
}

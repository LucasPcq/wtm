package env

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
)

func TestALinkWritesTheOriginItsJobPublishes(t *testing.T) {
	job := publishedOriginJob("api-dev", "PORT")
	cases := []struct {
		name     string
		job      project.Job
		port     string
		worktree settling
		want     string
	}{
		{"published_job_ported_proxy", job, "PORT", settling{Slug: "feat-x", Project: "monorepo", PublicPort: 10080}, "http://api-dev.feat-x.monorepo.localhost:10080"},
		{"privileged_port_vanishes", job, "PORT", settling{Slug: "feat-x", Project: "monorepo", PublicPort: privilegedProxyPort}, "http://api-dev.feat-x.monorepo.localhost"},
		{"explicit_host_wins", project.Job{Name: "api-dev", Ports: map[string]int{"PORT": 4001}, URL: &project.URL{Port: "PORT", Host: "api"}}, "PORT", settling{Slug: "feat-x", Project: "monorepo", PublicPort: 10080}, "http://api.feat-x.monorepo.localhost:10080"},
		{"names_project_dns_safe", job, "PORT", settling{Slug: "feat/Big_Thing", Project: "My Repo", PublicPort: 10080}, "http://api-dev.feat-big-thing.my-repo.localhost:10080"},
		{"unpublished_job_keeps_port", project.Job{Name: "postgres", Ports: map[string]int{"POSTGRES_PORT": 5432}}, "POSTGRES_PORT", settling{Slug: "feat-x", Project: "monorepo", PublicPort: 10080}, ""},
		{"unpublished_port_link_keeps_port", project.Job{Name: "api-dev", Ports: map[string]int{"PORT": 4001, "METRICS_PORT": 9001}, URL: &project.URL{Port: "PORT"}}, "METRICS_PORT", settling{Slug: "feat-x", Project: "monorepo", PublicPort: 10080}, ""},
		{"no_public_port_no_name", job, "PORT", settling{Slug: "feat-x", Project: "monorepo"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.worktree.Jobs = map[string]project.Job{tc.job.Name: tc.job}

			assert.Equal(t, tc.want, tc.worktree.linkOrigin(project.EnvPortLink{Job: tc.job.Name, Port: tc.port}))
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
		{name: "local_origin_named", value: "http://localhost:4001", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "own_port_anchors", value: "http://localhost:4011", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "loopback_anchors", value: "http://127.0.0.1:4001", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "path_query_survive", value: "http://localhost:4001/v1/auth?redirect=http%3A%2F%2Fx&p=4001", want: "http://api-dev.feat-x.monorepo.localhost:10080/v1/auth?redirect=http%3A%2F%2Fx&p=4001", status: StatusRewrite},
		{name: "port_in_path_ignored", value: "http://localhost:4001/cb?port=4001", want: "http://api-dev.feat-x.monorepo.localhost:10080/cb?port=4001", status: StatusRewrite},
		{name: "same_origin_idempotent", value: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusUnchanged},
		{name: "foreign_worktree_route_corrected", value: "http://api-dev.feat-y.monorepo.localhost:10080", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "pre_redirection_route_recalled", value: "http://api-dev.feat-x.monorepo.localhost", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "list_local_elements_translated", value: "http://localhost:4001,https://app.example.com", want: "http://api-dev.feat-x.monorepo.localhost:10080,https://app.example.com", status: StatusRewrite},
		{name: "list_spacing_preserved", value: "http://localhost:4001, http://localhost:9999", want: "http://api-dev.feat-x.monorepo.localhost:10080, http://localhost:9999", status: StatusRewrite},
		{name: "credentials_survive", value: "http://u:pw@localhost:4001/api", want: "http://u:pw@api-dev.feat-x.monorepo.localhost:10080/api", status: StatusRewrite},
		{name: "ipv6_loopback_anchors", value: "http://[::1]:4001", want: "http://api-dev.feat-x.monorepo.localhost:10080", status: StatusRewrite},
		{name: "https_own_port_refused", value: "https://localhost:4001", status: StatusSecureScheme},
		{name: "https_in_list_refused", value: "http://localhost:4001,https://localhost:4001", status: StatusSecureScheme},
		{name: "foreign_host_reported", value: "https://api.staging.example.com", status: StatusForeignHost},
		{name: "other_local_port_ignored", value: "http://localhost:9999", status: StatusNotFound},
		{name: "bare_number_falls_back", value: "4001", fallback: true},
		{name: "unknown_scheme_falls_back", value: "postgres://user:pw@localhost:4001/db", fallback: true},
		{name: "empty_falls_back", value: "", fallback: true},
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
		{"own_route_rewinds", "http://api-dev.feat-x.monorepo.localhost:10080", "http://localhost:4001"},
		{"foreign_worktree_route_rewinds", "http://api-dev.feat-y.monorepo.localhost:10080", "http://localhost:4001"},
		{"portless_route_rewinds", "http://api-dev.feat-x.monorepo.localhost/v1", "http://localhost:4001/v1"},
		{"ported_value_left", "http://localhost:4011", "http://localhost:4011"},
		{"other_job_route_left", "http://web-dev.feat-x.monorepo.localhost:10080", "http://web-dev.feat-x.monorepo.localhost:10080"},
		{"other_project_route_left", "http://api-dev.feat-x.other.localhost:10080", "http://api-dev.feat-x.other.localhost:10080"},
		{"list_elements_rewound", "http://api-dev.feat-x.monorepo.localhost:10080,https://app.example.com", "http://localhost:4001,https://app.example.com"},
		{"credentials_survive", "http://u:pw@api-dev.feat-x.monorepo.localhost:10080/api", "http://u:pw@localhost:4001/api"},
		{"bare_number_untouched", "4011", "4011"},
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
	cases := []struct{ name, value string }{
		{"credentials", "https://u:pw@api.staging.example.com/v1"},
		{"hash_in_password", "http://user:hunter2#x@api.staging.example.com/v1"},
		{"question_mark_in_password", "http://user:hunter2?x@api.staging.example.com/v1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteOrigin(rewriteOriginOf(tc.value))
			plan := Plan{Entries: []Entry{{File: ".env", Key: "API", Port: "P", Status: got.Status, ForeignHost: got.ForeignHost}}}

			redacted := Redact(plan)

			assert.Equal(t, StatusForeignHost, got.Status)
			assert.Empty(t, redacted.Entries[0].ForeignHost)
		})
	}
}

func TestAJobIsPublishedUnderItsJobWorktreeAndProject(t *testing.T) {
	published := func(name, host string) project.Job {
		return project.Job{Name: name, Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT", Host: host}}
	}
	cases := []struct {
		name     string
		job      project.Job
		worktree string
		project  string
		want     string
	}{
		{"job_name_default", published("app1-web", ""), "feat-auth", "myapp", "app1-web.feat-auth.myapp.localhost"},
		{"declared_host", published("app1-api", "api.app-1"), "feat-auth", "myapp", "api.app-1.feat-auth.myapp.localhost"},
		{"segments_dns_safe", published("App_Web", ""), "feat/auth", "My.App", "app-web.feat-auth.my-app.localhost"},
		{"job_without_url", project.Job{Name: "db"}, "feat-auth", "myapp", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, settling{Slug: tc.worktree, Project: tc.project}.routeHost(tc.job))
		})
	}
}

func TestTwoWorktreesNeverShareACookieParent(t *testing.T) {
	job := project.Job{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}}

	a := settling{Slug: "feat-auth", Project: "myapp"}.routeHost(job)
	b := settling{Slug: "main", Project: "myapp"}.routeHost(job)

	assert.NotEqual(t, a, b)
	assert.NotEqual(t, cookieParent(a), cookieParent(b))
}

func TestAddressingIsNamesUnlessPortsAreAskedFor(t *testing.T) {
	cases := []struct {
		name   string
		config project.Run
		want   project.Addressing
	}{
		{"unset_names", project.Run{}, project.AddressNames},
		{"names", project.Run{Addressing: project.AddressNames}, project.AddressNames},
		{"ports_opt_out", project.Run{Addressing: project.AddressPorts}, project.AddressPorts},
		{"unknown_default", project.Run{Addressing: "nope"}, project.AddressNames},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, EffectiveAddressing(tc.config))
		})
	}
}

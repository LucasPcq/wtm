package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func publishedJob(name, port string) domain.JobConfig {
	return domain.JobConfig{
		Name:  name,
		Ports: map[string]int{port: 4001},
		URL:   &domain.JobURLConfig{Port: port},
	}
}

func TestLinkOrigin(t *testing.T) {
	job := publishedJob("api-dev", "PORT")

	cases := []struct {
		name   string
		params rules.LinkOriginParams
		want   string
	}{
		{
			name: "published job on the ported proxy",
			params: rules.LinkOriginParams{
				Job: job, PortName: "PORT", Worktree: "feat-x", Project: "monorepo", PublicPort: 10080,
			},
			want: "http://api-dev.feat-x.monorepo.localhost:10080",
		},
		{
			name: "the privileged port vanishes from the origin",
			params: rules.LinkOriginParams{
				Job: job, PortName: "PORT", Worktree: "feat-x", Project: "monorepo",
				PublicPort: domain.ProxyPrivilegedPort,
			},
			want: "http://api-dev.feat-x.monorepo.localhost",
		},
		{
			name: "an explicit url.host wins over the job name",
			params: rules.LinkOriginParams{
				Job: domain.JobConfig{
					Name: "api-dev", Ports: map[string]int{"PORT": 4001},
					URL: &domain.JobURLConfig{Port: "PORT", Host: "api"},
				},
				PortName: "PORT", Worktree: "feat-x", Project: "monorepo", PublicPort: 10080,
			},
			want: "http://api.feat-x.monorepo.localhost:10080",
		},
		{
			name: "names and project are made DNS-safe",
			params: rules.LinkOriginParams{
				Job: job, PortName: "PORT", Worktree: "feat/Big_Thing", Project: "My Repo", PublicPort: 10080,
			},
			want: "http://api-dev.feat-big-thing.my-repo.localhost:10080",
		},
		{
			name: "a job publishing nothing keeps its port",
			params: rules.LinkOriginParams{
				Job:      domain.JobConfig{Name: "postgres", Ports: map[string]int{"POSTGRES_PORT": 5432}},
				PortName: "POSTGRES_PORT", Worktree: "feat-x", Project: "monorepo", PublicPort: 10080,
			},
			want: "",
		},
		{
			name: "a link following a port the url does not publish keeps its port",
			params: rules.LinkOriginParams{
				Job: domain.JobConfig{
					Name: "api-dev", Ports: map[string]int{"PORT": 4001, "METRICS_PORT": 9001},
					URL: &domain.JobURLConfig{Port: "PORT"},
				},
				PortName: "METRICS_PORT", Worktree: "feat-x", Project: "monorepo", PublicPort: 10080,
			},
			want: "",
		},
		{
			name: "no public port means nothing serves the name",
			params: rules.LinkOriginParams{
				Job: job, PortName: "PORT", Worktree: "feat-x", Project: "monorepo", PublicPort: 0,
			},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.LinkOrigin(tc.params); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func rewriteParams(value string) rules.RewriteOriginParams {
	return rules.RewriteOriginParams{
		Value:    value,
		Origin:   "http://api-dev.feat-x.monorepo.localhost:10080",
		JobLabel: "api-dev",
		Project:  "monorepo",
		Base:     4001,
		Resolved: 4011,
	}
}

func TestRewriteOrigin(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		want     string
		status   domain.EnvPortStatus
		fallback bool
	}{
		{
			name:   "a local origin becomes the named one",
			value:  "http://localhost:4001",
			want:   "http://api-dev.feat-x.monorepo.localhost:10080",
			status: domain.EnvPortStatusRewrite,
		},
		{
			name:   "the worktree's own port anchors too",
			value:  "http://localhost:4011",
			want:   "http://api-dev.feat-x.monorepo.localhost:10080",
			status: domain.EnvPortStatusRewrite,
		},
		{
			name:   "the loopback address anchors like the name",
			value:  "http://127.0.0.1:4001",
			want:   "http://api-dev.feat-x.monorepo.localhost:10080",
			status: domain.EnvPortStatusRewrite,
		},
		{
			name:   "path and query survive the authority swap",
			value:  "http://localhost:4001/v1/auth?redirect=http%3A%2F%2Fx&p=4001",
			want:   "http://api-dev.feat-x.monorepo.localhost:10080/v1/auth?redirect=http%3A%2F%2Fx&p=4001",
			status: domain.EnvPortStatusRewrite,
		},
		{
			name:   "a port sitting in the path is never a candidate",
			value:  "http://localhost:4001/cb?port=4001",
			want:   "http://api-dev.feat-x.monorepo.localhost:10080/cb?port=4001",
			status: domain.EnvPortStatusRewrite,
		},
		{
			name:   "the same origin twice is idempotent",
			value:  "http://api-dev.feat-x.monorepo.localhost:10080",
			status: domain.EnvPortStatusUnchanged,
		},
		{
			name:   "another worktree's route is corrected to this one",
			value:  "http://api-dev.feat-y.monorepo.localhost:10080",
			want:   "http://api-dev.feat-x.monorepo.localhost:10080",
			status: domain.EnvPortStatusRewrite,
		},
		{
			name:   "a route written before the redirection was installed is recalled",
			value:  "http://api-dev.feat-x.monorepo.localhost",
			want:   "http://api-dev.feat-x.monorepo.localhost:10080",
			status: domain.EnvPortStatusRewrite,
		},
		{
			name:   "every local element of a list is translated, the rest left",
			value:  "http://localhost:4001,https://app.example.com",
			want:   "http://api-dev.feat-x.monorepo.localhost:10080,https://app.example.com",
			status: domain.EnvPortStatusRewrite,
		},
		{
			name:   "spacing inside a list is preserved",
			value:  "http://localhost:4001, http://localhost:9999",
			want:   "http://api-dev.feat-x.monorepo.localhost:10080, http://localhost:9999",
			status: domain.EnvPortStatusRewrite,
		},
		{
			name:   "credentials in the url survive the authority swap",
			value:  "http://u:pw@localhost:4001/api",
			want:   "http://u:pw@api-dev.feat-x.monorepo.localhost:10080/api",
			status: domain.EnvPortStatusRewrite,
		},
		{
			name:   "the IPv6 loopback anchors too",
			value:  "http://[::1]:4001",
			want:   "http://api-dev.feat-x.monorepo.localhost:10080",
			status: domain.EnvPortStatusRewrite,
		},
		{
			name:   "https on our own port is refused, never downgraded",
			value:  "https://localhost:4001",
			status: domain.EnvPortStatusSecureScheme,
		},
		{
			name:   "https anywhere in a list refuses the whole value",
			value:  "http://localhost:4001,https://localhost:4001",
			status: domain.EnvPortStatusSecureScheme,
		},
		{
			name:   "a host no job here serves is reported",
			value:  "https://api.staging.example.com",
			status: domain.EnvPortStatusForeignHost,
		},
		{
			name:   "a local url on some other port anchors nothing",
			value:  "http://localhost:9999",
			status: domain.EnvPortStatusNotFound,
		},
		{
			name:     "a bare number falls back to the port substitution",
			value:    "4001",
			fallback: true,
		},
		{
			name:     "a scheme the proxy does not speak falls back too",
			value:    "postgres://user:pw@localhost:4001/db",
			fallback: true,
		},
		{
			name:     "an empty value falls back",
			value:    "",
			fallback: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rules.RewriteOrigin(rewriteParams(tc.value))
			if got.Fallback != tc.fallback {
				t.Fatalf("fallback: got %v, want %v", got.Fallback, tc.fallback)
			}
			if tc.fallback {
				return
			}
			if got.Status != tc.status {
				t.Fatalf("status: got %q, want %q", got.Status, tc.status)
			}
			if got.Status == domain.EnvPortStatusRewrite && got.Value != tc.want {
				t.Fatalf("value:\n got %q\nwant %q", got.Value, tc.want)
			}
		})
	}
}

func TestReduceOriginValue(t *testing.T) {
	params := func(value string) rules.ReduceOriginParams {
		return rules.ReduceOriginParams{
			Value: value, JobLabel: "api-dev", Project: "monorepo", Base: 4001,
		}
	}

	cases := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "a route of this worktree rewinds to the base port",
			value: "http://api-dev.feat-x.monorepo.localhost:10080",
			want:  "http://localhost:4001",
		},
		{
			name:  "another worktree's route rewinds to the same thing",
			value: "http://api-dev.feat-y.monorepo.localhost:10080",
			want:  "http://localhost:4001",
		},
		{
			name:  "a portless route rewinds too",
			value: "http://api-dev.feat-x.monorepo.localhost/v1",
			want:  "http://localhost:4001/v1",
		},
		{
			name:  "a value already on a port is left for the port reduction",
			value: "http://localhost:4011",
			want:  "http://localhost:4011",
		},
		{
			name:  "another job's route is not ours to rewind",
			value: "http://web-dev.feat-x.monorepo.localhost:10080",
			want:  "http://web-dev.feat-x.monorepo.localhost:10080",
		},
		{
			name:  "another project's route is not ours either",
			value: "http://api-dev.feat-x.other.localhost:10080",
			want:  "http://api-dev.feat-x.other.localhost:10080",
		},
		{
			name:  "each element of a list is rewound",
			value: "http://api-dev.feat-x.monorepo.localhost:10080,https://app.example.com",
			want:  "http://localhost:4001,https://app.example.com",
		},
		{
			name:  "credentials survive the rewind",
			value: "http://u:pw@api-dev.feat-x.monorepo.localhost:10080/api",
			want:  "http://u:pw@localhost:4001/api",
		},
		{
			name:  "a bare number is untouched",
			value: "4011",
			want:  "4011",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.ReduceOriginValue(params(tc.value)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestReduceOriginValueCanonicalizes is the property the inter-worktree diff
// rests on: main spelling a value by port and a worktree spelling it by name
// must reduce to the same string, or every URL key reads as permanent drift.
func TestReduceOriginValueCanonicalizes(t *testing.T) {
	reduce := func(value string) string {
		return rules.ReduceEnvPortValue(rules.ReduceEnvPortParams{
			Value: value, Base: 4001, Block: 10, JobLabel: "api-dev", Project: "monorepo",
		})
	}

	main := reduce("http://localhost:4001")
	named := reduce("http://api-dev.feat-x.monorepo.localhost:10080")
	ported := reduce("http://localhost:4011")

	if main != named || main != ported {
		t.Fatalf("three spellings of one setting must reduce equal: main=%q named=%q ported=%q", main, named, ported)
	}
}

func TestRewriteOriginNamesAForeignHost(t *testing.T) {
	got := rules.RewriteOrigin(rewriteParams("https://api.staging.example.com/v1"))
	if got.Status != domain.EnvPortStatusForeignHost || got.ForeignHost != "api.staging.example.com" {
		t.Fatalf("got %+v, want foreign host api.staging.example.com", got)
	}
}

// LUC-279: the host was read up to a "#" before the "@", so a password came
// out as the foreign host; a value carrying credentials names no host at all.
func TestRewriteOriginNamesNoForeignHostBesideCredentials(t *testing.T) {
	for _, value := range []string{
		"https://u:pw@api.staging.example.com/v1",
		"http://user:hunter2#x@api.staging.example.com/v1",
		"http://user:hunter2?x@api.staging.example.com/v1",
	} {
		got := rules.RewriteOrigin(rewriteParams(value))
		if got.Status != domain.EnvPortStatusForeignHost || got.ForeignHost != "" {
			t.Errorf("RewriteOrigin(%q) = %+v, want foreign_host with no host named", value, got)
		}
	}
}

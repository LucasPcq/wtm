package env

import (
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

const shopProject = "shop"

func shopRun() project.Run {
	return project.Run{Jobs: []project.Job{
		{Name: "web", Kind: project.Service, Cmd: "npm start", Ports: map[string]int{"http": 3000}},
		{Name: "infra", Kind: project.Service, Cmd: "docker compose up -d", Cwd: "infra", Scope: project.Shared},
	}}
}

func shopParams(branch string, ordinal int, isolation project.Isolation) JobParams {
	return JobParams{Branch: branch, ProjectDir: "/repo/" + shopProject, Ordinal: ordinal, Isolation: isolation, Run: shopRun()}
}

const redactSecret = "s3cr3t-value"

func sharedKeycloakJob() project.Job {
	return project.Job{
		Name:      "keycloak",
		Kind:      project.Service,
		Cmd:       "docker compose up -d keycloak",
		Scope:     project.Shared,
		Ports:     map[string]int{"KEYCLOAK_PORT": 8080},
		URL:       &project.URL{Port: "KEYCLOAK_PORT"},
		Namespace: &project.Namespace{Name: "realm_{worktree}", Create: "true"},
	}
}

func linkedResolve() ResolveParams {
	config := shopRun()
	config.EnvPorts = []project.EnvPortLink{{File: ".env", Key: "PORT", Job: "web", Port: "http"}}
	return ResolveParams{
		JobParams: JobParams{Branch: "feat/a", ProjectDir: "/repo/shop", Ordinal: 2, Run: config},
		Files:     []project.EnvFile{{Target: ".env"}, {Target: "infra/.env"}},
	}
}

func publishedOriginJob(name, port string) project.Job {
	return project.Job{Name: name, Ports: map[string]int{port: 4001}, URL: &project.URL{Port: port}}
}

func rewriteOriginOf(value string) rewriteOriginParams {
	return rewriteOriginParams{
		Value:    value,
		Origin:   "http://api-dev.feat-x.monorepo.localhost:10080",
		JobLabel: "api-dev",
		Project:  "monorepo",
		Base:     4001,
		Resolved: 4011,
	}
}

func redactedPlanFixture() Plan {
	return Plan{
		Offset: 10,
		Entries: []Entry{
			{File: ".env", Key: "WEB_PORT", Status: StatusUnchanged, CurrentValue: "3010"},
			{File: ".env", Key: "DATABASE_URL", Base: 5432, Resolved: 5442, Status: StatusRewrite,
				CurrentValue: "postgres://app:" + redactSecret + "@localhost:5432/db", NewValue: "postgres://app:" + redactSecret + "@localhost:5442/db"},
			{File: ".env", Key: "API_URL", Status: StatusForeignHost, CurrentValue: "http://" + redactSecret + "@staging.example.com", ForeignHost: "staging.example.com"},
		},
		Owned: []OwnedEntry{{File: ".env", Key: "REALM", Value: "app-feat-a"}},
	}
}

func keycloakWorktree(job project.Job) settling {
	return settling{Addressing: project.AddressNames, Jobs: map[string]project.Job{job.Name: job}, Slug: "feat-a", Project: "demo", PublicPort: privilegedProxyPort, Ordinal: 2, Offset: 20}
}

func keycloakValue(value string) project.EnvValueLink {
	return project.EnvValueLink{File: ".env", Key: "K", Job: "keycloak", Value: value}
}

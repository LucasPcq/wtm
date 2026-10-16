package runenv

import (
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

const (
	shopProject    = "shop"
	shopProjectDir = "/repo/shop"
)

const redactSecret = "s3cr3t-value"

func sharedKeycloakJob() run.Job {
	return run.Job{
		Name:      "keycloak",
		Kind:      run.Service,
		Cmd:       "docker compose up -d keycloak",
		Scope:     run.Shared,
		Ports:     map[string]int{"KEYCLOAK_PORT": 8080},
		URL:       &run.URL{Port: "KEYCLOAK_PORT"},
		Namespace: &run.Namespace{Name: "realm_{worktree}", Create: "true"},
	}
}

func shopRun() run.Config {
	return run.Config{Jobs: []run.Job{
		{Name: "web", Kind: run.Service, Cmd: "npm start", Ports: map[string]int{"http": 3000}},
		{Name: "infra", Kind: run.Service, Cmd: "docker compose up -d", Cwd: "infra", Scope: run.Shared},
	}}
}

func shopParams(branch string, ordinal int, isolation run.Isolation) Params {
	return Params{Branch: branch, ProjectDir: shopProjectDir, Ordinal: ordinal, Isolation: isolation, Run: shopRun()}
}

func linkedResolve() ResolveParams {
	config := shopRun()
	config.EnvPorts = []run.EnvPortLink{{File: ".env", Key: "PORT", Job: "web", Port: "http"}}
	return ResolveParams{
		Params: Params{Branch: "feat/a", ProjectDir: "/repo/shop", Ordinal: 2, Run: config},
		Files:  []project.EnvFile{{Target: ".env"}, {Target: "infra/.env"}},
	}
}

func publishedOriginJob(name, port string) run.Job {
	return run.Job{Name: name, Ports: map[string]int{port: 4001}, URL: &run.URL{Port: port}}
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

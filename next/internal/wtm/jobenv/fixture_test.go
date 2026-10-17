package jobenv

import "github.com/LucasPcq/wtm/next/internal/wtm/run"

const shopProject = "shop"

func shopRun() run.Config {
	return run.Config{Jobs: []run.Job{
		{Name: "web", Kind: run.Service, Cmd: "npm start", Ports: map[string]int{"http": 3000}},
		{Name: "infra", Kind: run.Service, Cmd: "docker compose up -d", Cwd: "infra", Scope: run.Shared},
	}}
}

func shopParams(branch string, ordinal int, isolation run.Isolation) Params {
	return Params{Branch: branch, ProjectDir: "/repo/" + shopProject, Ordinal: ordinal, Isolation: isolation, Run: shopRun()}
}

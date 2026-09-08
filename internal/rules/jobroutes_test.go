package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func turborepo() domain.RunConfig {
	published := func(name, port string) domain.JobConfig {
		return domain.JobConfig{
			Name: name, Kind: domain.JobKindService, Cwd: "apps/" + name,
			Ports: map[string]int{port: 3000},
			URL:   &domain.JobURLConfig{Port: port},
		}
	}
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "dev", Kind: domain.JobKindService, Runs: []string{"dev-shop", "dev-crm"}},
		{Name: "dev-shop", Kind: domain.JobKindService, Runs: []string{"shop-web", "shop-api"}},
		{Name: "dev-crm", Kind: domain.JobKindService, Runs: []string{"crm-web"}},
		published("shop-web", "PORT"),
		published("shop-api", "SHOP_API_PORT"),
		published("crm-web", "CRM_WEB_PORT"),
	}}
}

func routeHosts(routes []domain.JobRoute) []string {
	hosts := make([]string, 0, len(routes))
	for _, route := range routes {
		hosts = append(hosts, route.Host)
	}
	return hosts
}

func TestJobRoutesPublishesTheChildrenARunnerStarts(t *testing.T) {
	cfg := turborepo()

	routes := rules.JobRoutes(rules.JobRoutesParams{
		Config: cfg, Job: cfg.Jobs[0], Worktree: "feat-x", Project: "shop",
	})

	// The root runs no process of its own that answers, so it publishes nothing
	// for itself — and the three apps it holds, transitively through the two
	// intermediate runners, are the names that must reach the proxy.
	want := []string{
		"shop-web.feat-x.shop.localhost",
		"shop-api.feat-x.shop.localhost",
		"crm-web.feat-x.shop.localhost",
	}
	got := routeHosts(routes)
	if len(got) != len(want) {
		t.Fatalf("hosts = %v, want %v", got, want)
	}
	for i, host := range want {
		if got[i] != host {
			t.Errorf("host %d = %q, want %q", i, got[i], host)
		}
	}
}

func TestJobRoutesNamesThePortVariableNotItsValue(t *testing.T) {
	cfg := turborepo()

	routes := rules.JobRoutes(rules.JobRoutesParams{
		Config: cfg, Job: cfg.Jobs[1], Worktree: "feat-x", Project: "shop",
	})

	if len(routes) != 2 {
		t.Fatalf("routes = %+v, want the two apps dev-shop starts", routes)
	}
	if routes[0].Job != "shop-web" || routes[0].Port != "PORT" {
		t.Errorf("route = %+v, want shop-web's own port variable", routes[0])
	}
	if routes[1].Port != "SHOP_API_PORT" {
		t.Errorf("route = %+v, want shop-api's own port variable", routes[1])
	}
}

func TestJobRoutesOfAPlainJobIsItsOwnName(t *testing.T) {
	cfg := turborepo()

	routes := rules.JobRoutes(rules.JobRoutesParams{
		Config: cfg, Job: cfg.Jobs[3], Worktree: "feat-x", Project: "shop",
	})

	if len(routes) != 1 || routes[0].Host != "shop-web.feat-x.shop.localhost" {
		t.Fatalf("routes = %+v, want the job's own name alone", routes)
	}
	if host := rules.JobOwnRoute(routes, "shop-web"); host != routes[0].Host {
		t.Errorf("JobOwnRoute = %q, want %q", host, routes[0].Host)
	}
}

func TestJobRoutesSkipsWhatPublishesNothing(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "stack", Kind: domain.JobKindService, Runs: []string{"db"}},
		{Name: "db", Kind: domain.JobKindService, Ports: map[string]int{"PG_PORT": 5432}},
	}}

	routes := rules.JobRoutes(rules.JobRoutesParams{
		Config: cfg, Job: cfg.Jobs[0], Worktree: "feat-x", Project: "shop",
	})

	if len(routes) != 0 {
		t.Errorf("routes = %+v, want none: neither job publishes a url", routes)
	}
	if host := rules.JobOwnRoute(routes, "stack"); host != "" {
		t.Errorf("JobOwnRoute = %q, want empty", host)
	}
}

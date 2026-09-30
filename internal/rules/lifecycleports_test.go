package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func lifecycleConfig(jobs ...domain.JobConfig) domain.RunConfig {
	return domain.RunConfig{Jobs: jobs}
}

func TestLifecyclePortsAppliesOffsetToUnambiguousNames(t *testing.T) {
	ports := LifecyclePorts(LifecyclePortsParams{
		Config: lifecycleConfig(
			domain.JobConfig{Name: "db", Ports: map[string]int{"DB_PORT": 5432}},
			domain.JobConfig{Name: "web", Ports: map[string]int{"PORT": 3000}},
		),
		PortOffset: 10,
	})

	want := map[string]int{"DB_PORT": 5442, "PORT": 3010}
	for name, port := range want {
		if ports[name] != port {
			t.Errorf("%s = %d, want %d", name, ports[name], port)
		}
	}
	if len(ports) != len(want) {
		t.Errorf("got %d ports, want %d: %v", len(ports), len(want), ports)
	}
}

// The main checkout runs at offset 0, so its hooks see the declared bases.
func TestLifecyclePortsMainCheckoutKeepsBases(t *testing.T) {
	ports := LifecyclePorts(LifecyclePortsParams{
		Config: lifecycleConfig(domain.JobConfig{Name: "db", Ports: map[string]int{"DB_PORT": 5432}}),
	})

	if ports["DB_PORT"] != 5432 {
		t.Errorf("DB_PORT = %d, want 5432", ports["DB_PORT"])
	}
}

// Two jobs naming the same variable give it no answer: leaving it unresolved is
// the only alternative to handing a hook the wrong port.
func TestLifecyclePortsSkipsAmbiguousName(t *testing.T) {
	ports := LifecyclePorts(LifecyclePortsParams{
		Config: lifecycleConfig(
			domain.JobConfig{Name: "web", Ports: map[string]int{"PORT": 3000, "DB_PORT": 5432}},
			domain.JobConfig{Name: "api", Ports: map[string]int{"PORT": 8080}},
		),
		PortOffset: 10,
	})

	if _, declared := ports["PORT"]; declared {
		t.Errorf("PORT resolved to %d, want it left out", ports["PORT"])
	}
	if ports["DB_PORT"] != 5442 {
		t.Errorf("DB_PORT = %d, want 5442", ports["DB_PORT"])
	}
}

func TestLifecyclePortsWithoutDeclarationsIsNil(t *testing.T) {
	if ports := LifecyclePorts(LifecyclePortsParams{Config: lifecycleConfig(domain.JobConfig{Name: "web"})}); ports != nil {
		t.Errorf("got %v, want nil", ports)
	}
}

func TestWithPortEnvOverridesInheritedValue(t *testing.T) {
	env := map[string]string{"DB_PORT": "5432", domain.EnvBranch: "feat/x"}

	merged := WithPortEnv(env, map[string]int{"DB_PORT": 5442})

	if merged["DB_PORT"] != "5442" {
		t.Errorf("DB_PORT = %q, want %q", merged["DB_PORT"], "5442")
	}
	if merged[domain.EnvBranch] != "feat/x" {
		t.Errorf("%s = %q, want %q", domain.EnvBranch, merged[domain.EnvBranch], "feat/x")
	}
	if env["DB_PORT"] != "5432" {
		t.Errorf("source env was mutated: DB_PORT = %q", env["DB_PORT"])
	}
}

func TestWithPortEnvWithoutPortsReturnsEnv(t *testing.T) {
	env := map[string]string{domain.EnvBranch: "main"}

	if merged := WithPortEnv(env, nil); len(merged) != 1 || merged[domain.EnvBranch] != "main" {
		t.Errorf("got %v, want %v", merged, env)
	}
}

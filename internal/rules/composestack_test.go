package rules_test

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestComposeProbeCarriesTheFilesTheLauncherUsed(t *testing.T) {
	probe := rules.ComposeProbeFor(domain.JobConfig{
		Cmd: "docker compose -f docker-compose.yml up -d --no-deps redis mailpit",
	})

	if !probe.Recognized {
		t.Fatal("a compose launcher is the one thing wtm can verify")
	}
	want := []string{"compose", "-f", "docker-compose.yml", "ps", "-q"}
	if !slices.Equal(probe.Args, want) {
		t.Fatalf("args = %v, want %v: without the file, ps answers about whatever compose file sits in the directory", probe.Args, want)
	}
}

func TestComposeProbeReadsEverySpellingOfTheFileFlag(t *testing.T) {
	cases := []string{
		"docker compose --file a.yml -f b.yml up -d",
		"docker compose --file=a.yml -f b.yml up -d",
	}
	want := []string{"compose", "-f", "a.yml", "-f", "b.yml", "ps", "-q"}

	for _, cmd := range cases {
		probe := rules.ComposeProbeFor(domain.JobConfig{Cmd: cmd})
		if !slices.Equal(probe.Args, want) {
			t.Errorf("%q → args = %v, want %v", cmd, probe.Args, want)
		}
	}
}

func TestComposeProbeAcceptsTheLegacyBinary(t *testing.T) {
	probe := rules.ComposeProbeFor(domain.JobConfig{Cmd: "docker-compose up -d"})

	if !probe.Recognized {
		t.Fatal("docker-compose v1 is still what a lot of run.toml files say")
	}
	if !slices.Equal(probe.Args, []string{"compose", "ps", "-q"}) {
		t.Fatalf("args = %v", probe.Args)
	}
}

func TestComposeProbeRecognizesNothingElse(t *testing.T) {
	for _, cmd := range []string{
		"",
		"pnpm dev",
		"kubectl apply -f k8s.yml",
		"podman compose up -d",
		"my-docker compose up",
		"docker run -d nginx",
	} {
		if probe := rules.ComposeProbeFor(domain.JobConfig{Cmd: cmd}); probe.Recognized {
			t.Errorf("%q was recognized: an unverifiable launcher must keep saying what wtm actually knows", cmd)
		}
	}
}

func TestComposeProbeSurvivesATrailingFileFlag(t *testing.T) {
	probe := rules.ComposeProbeFor(domain.JobConfig{Cmd: "docker compose -f"})

	if !probe.Recognized {
		t.Fatal("the launcher is still compose, however malformed the rest is")
	}
	if !slices.Equal(probe.Args, []string{"compose", "ps", "-q"}) {
		t.Fatalf("args = %v, want the flag with no value dropped rather than passed on", probe.Args)
	}
}

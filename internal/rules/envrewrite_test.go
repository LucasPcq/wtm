package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func onlyPair(t *testing.T, content string) domain.EnvLine {
	t.Helper()
	for _, line := range rules.ParseEnv(content) {
		if line.Kind == domain.EnvLinePair {
			return line
		}
	}
	t.Fatalf("no pair in %q", content)
	return domain.EnvLine{}
}

func TestWithEnvValueKeepsTheLineAsWritten(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		value string
		want  string
	}{
		{"bare", "PORT=4002", "4012", "PORT=4012"},
		{"single quotes holding a dollar stay single", "REDIS_URL='redis://u:pa$w0rd@localhost:6379/0'", "redis://u:pa$w0rd@localhost:6389/0", "REDIS_URL='redis://u:pa$w0rd@localhost:6389/0'"},
		{"double quotes and an inline comment", `MINIO_URL="http://localhost:9000" # minio endpoint`, "http://localhost:9010", `MINIO_URL="http://localhost:9010" # minio endpoint`},
		{"bare with an inline comment", "ADMINER_PORT=8080 # adminer ui", "8090", "ADMINER_PORT=8090 # adminer ui"},
		{"export prefix and indentation", "  export API_PORT=4002", "4012", "  export API_PORT=4012"},
		{"spacing around the assignment", "API_PORT = 4002", "4012", "API_PORT = 4012"},
		{"a carriage return stays", "PORT=4002\r", "4012", "PORT=4012\r"},
		{"a quoted value and a carriage return", "URL='http://localhost:4002'\r", "http://localhost:4012", "URL='http://localhost:4012'\r"},
		{"single quotes rewritten to a hostname", "VITE_API_URL='http://localhost:4002/api'", "http://api.feat-x.monorepo.localhost:11080/api", "VITE_API_URL='http://api.feat-x.monorepo.localhost:11080/api'"},
		{"an empty value takes the new one", "COMPOSE_PROJECT_NAME=", "monorepo-feat-x", "COMPOSE_PROJECT_NAME=monorepo-feat-x"},
		{"an empty value before a comment", "COMPOSE_PROJECT_NAME= # set by wtm", "monorepo-feat-x", "COMPOSE_PROJECT_NAME=monorepo-feat-x # set by wtm"},
		{"a bare value needing quotes is quoted", "GREETING=hello", "hello world", `GREETING="hello world"`},
		{"a bare value gaining a dollar is single-quoted", "PW=secret", "pa$w0rd", "PW='pa$w0rd'"},
		{"a single-quoted value gaining a quote falls back", "Q='a'", "it's", "Q=it's"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rules.WithEnvValue(onlyPair(t, tc.line), tc.value)
			if got.Raw != tc.want {
				t.Fatalf("raw:\n got %q\nwant %q", got.Raw, tc.want)
			}
			if got.Value != tc.value {
				t.Fatalf("value = %q, want %q", got.Value, tc.value)
			}
			if back := onlyPair(t, got.Raw); back.Value != tc.value {
				t.Fatalf("re-parsed value = %q, want %q", back.Value, tc.value)
			}
		})
	}
}

func TestWithEnvValueSameValueIsIdentity(t *testing.T) {
	line := onlyPair(t, "A='x'   # c\r")
	if got := rules.WithEnvValue(line, "x"); got != line {
		t.Fatalf("got %+v, want the line unchanged", got)
	}
}

func TestRenderEnvKeepsACRLFFileCRLF(t *testing.T) {
	content := "# header\r\nA=1\r\nPORT=4002\r\n\r\nB='b'\r\n"
	lines := rules.ParseEnv(content)
	for i, line := range lines {
		if line.Key == "PORT" {
			lines[i] = rules.WithEnvValue(line, "4012")
		}
	}
	lines, _ = rules.UpsertEnvPair(rules.UpsertEnvPairParams{Lines: lines, Key: "COMPOSE_PROJECT_NAME", Value: "p"})

	want := "# header\r\nA=1\r\nPORT=4012\r\n\r\nB='b'\r\nCOMPOSE_PROJECT_NAME=p\r\n"
	if got := rules.RenderEnv(lines); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestRenderEnvLeavesAnLFFileLF(t *testing.T) {
	lines, _ := rules.UpsertEnvPair(rules.UpsertEnvPairParams{Lines: rules.ParseEnv("A=1\n"), Key: "B", Value: "2"})
	if got := rules.RenderEnv(lines); got != "A=1\nB=2\n" {
		t.Fatalf("got %q", got)
	}
}

// The port pass end to end over one file: every linked value moves, and every
// byte around it — quotes, comments, the export, the line endings — stays.
func TestApplyEnvPortsPreservesEveryLine(t *testing.T) {
	content := "export API_PORT=4002 # api\r\n" +
		"REDIS_URL='redis://u:pa$w0rd@localhost:6379/0'\r\n" +
		"MINIO_URL=\"http://localhost:9000\" # minio endpoint\r\n" +
		"UNRELATED='keep $me'\r\n"
	bases := map[domain.PortRef]int{
		{Job: "api", Name: "PORT"}:   4002,
		{Job: "redis", Name: "PORT"}: 6379,
		{Job: "minio", Name: "PORT"}: 9000,
	}
	lines := rules.ParseEnv(content)
	plan := rules.PlanEnvPorts(rules.PlanEnvPortsParams{
		Links: []domain.EnvPortLink{
			{File: ".env", Key: "API_PORT", Job: "api", Port: "PORT"},
			{File: ".env", Key: "REDIS_URL", Job: "redis", Port: "PORT"},
			{File: ".env", Key: "MINIO_URL", Job: "minio", Port: "PORT"},
		},
		Bases:  bases,
		Block:  10,
		Offset: 10,
		Lines:  map[string][]domain.EnvLine{".env": lines},
	})

	want := "export API_PORT=4012 # api\r\n" +
		"REDIS_URL='redis://u:pa$w0rd@localhost:6389/0'\r\n" +
		"MINIO_URL=\"http://localhost:9010\" # minio endpoint\r\n" +
		"UNRELATED='keep $me'\r\n"
	if got := rules.RenderEnv(rules.ApplyEnvPorts(lines, plan.Entries)); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestApplyEnvDiffAddsTheSourceLineAsWritten(t *testing.T) {
	child := rules.ParseEnv("A=1\r\n")
	main := rules.ParseEnv("A=1\nPW='pa$w0rd' # vault\n")
	diff := rules.DiffEnv(rules.EnvDiffParams{Main: main, Child: child, Mode: domain.EnvModeAdd})

	out := rules.ApplyEnvDiff(rules.ApplyEnvDiffParams{Child: child, Diff: diff})
	if got, want := rules.RenderEnv(out), "A=1\r\nPW='pa$w0rd' # vault\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyEnvDiffOverwriteKeepsTheChildLine(t *testing.T) {
	child := rules.ParseEnv("PW='old$1' # vault\n")
	main := rules.ParseEnv("PW=new$2\n")
	diff := rules.DiffEnv(rules.EnvDiffParams{Main: main, Child: child, Mode: domain.EnvModeRefresh})

	out := rules.ApplyEnvDiff(rules.ApplyEnvDiffParams{
		Child: child, Diff: diff,
		Decisions: map[string]domain.EnvConflictDecision{"PW": domain.EnvDecisionOverwrite},
	})
	if got, want := rules.RenderEnv(out), "PW='new$2' # vault\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

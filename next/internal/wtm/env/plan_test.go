package env

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"

	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestALinkedValueIsRewrittenOnlyWhereItsBaseSitsOnce(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		base       int
		offset     int
		wantStatus Status
		wantValue  string
	}{
		{"bare port", "URL=5432\n", 5432, 10, StatusRewrite, "5442"},
		{"port inside a database url", "URL=postgres://u:pw@localhost:5432/app\n", 5432, 10, StatusRewrite, "postgres://u:pw@localhost:5442/app"},
		{"port inside an http url", "URL=http://localhost:3000/api\n", 3000, 10, StatusRewrite, "http://localhost:3010/api"},
		{"quoted value", "URL=\"http://localhost:3000\"\n", 3000, 10, StatusRewrite, "http://localhost:3010"},
		{"offset zero leaves the value alone", "URL=postgres://localhost:5432/app\n", 5432, 0, StatusUnchanged, ""},
		{"already carries the resolved port", "URL=postgres://localhost:5442/app\n", 5432, 10, StatusUnchanged, ""},
		{"base absent from the value", "URL=postgres://localhost:6000/app\n", 5432, 10, StatusNotFound, ""},
		{"base twice in the value", "URL=http://localhost:3000/v3000\n", 3000, 10, StatusAmbiguous, ""},
		{"key absent from the file", "OTHER=1\n", 5432, 10, StatusMissingKey, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := planOne(t, planOneParams{Content: tc.content, Base: tc.base, Offset: tc.offset})

			assert.Equal(t, tc.wantStatus, got.Status)
			assert.Equal(t, tc.wantValue, got.NewValue)
		})
	}
}

func TestABaseNeverMatchesInsideALongerNumber(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		wantStatus Status
		wantValue  string
	}{
		{"base is a prefix of a longer number", "URL=http://localhost:54321/x\n", StatusNotFound, ""},
		{"base is a suffix of a longer number", "URL=http://localhost:15432/x\n", StatusNotFound, ""},
		{"longer number alongside a real match", "URL=http://a:54321/b:5432\n", StatusRewrite, "http://a:54321/b:5442"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := planOne(t, planOneParams{Content: tc.content, Base: 5432, Offset: 10})

			assert.Equal(t, tc.wantStatus, got.Status)
			assert.Equal(t, tc.wantValue, got.NewValue)
		})
	}
}

func TestALinkToAnUndeclaredPortIsLeftOutOfThePlan(t *testing.T) {
	plan := Writes{bases: map[portRef]int{{Job: "svc", Name: "DB_PORT"}: 5432}, block: 10, worktree: settling{Offset: 10}}.plan(planOfParams{
		Links: []project.EnvPortLink{{File: ".env", Key: "URL", Job: "svc", Port: "GONE"}},
		Lines: map[string][]dotenv.Line{".env": dotenv.Parse([]byte("URL=5432\n"))},
	})

	assert.Empty(t, plan.Entries)
}

func TestApplyingThePlanRewritesTheValueAndKeepsTheRest(t *testing.T) {
	content := "# header\nOTHER=keep\nURL=postgres://u:pw@localhost:5432/app\n"
	entry := planOne(t, planOneParams{Content: content, Base: 5432, Offset: 10})

	got := renderPlanned(applyPorts(dotenv.Parse([]byte(content)), []Entry{entry}))

	assert.Equal(t, "# header\nOTHER=keep\nURL=postgres://u:pw@localhost:5442/app\n", got)
}

func TestApplyingAnEntryThatIsNoRewriteChangesNothing(t *testing.T) {
	content := "URL=postgres://localhost:6000/app\n"
	entry := planOne(t, planOneParams{Content: content, Base: 5432, Offset: 10})

	assert.Equal(t, content, renderPlanned(applyPorts(dotenv.Parse([]byte(content)), []Entry{entry})))
}

func TestApplyingThePlanTwiceChangesNothingTheSecondTime(t *testing.T) {
	content := "URL=postgres://localhost:5432/app\n"

	first := renderPlanned(applyPorts(dotenv.Parse([]byte(content)), []Entry{planOne(t, planOneParams{Content: content, Base: 5432, Offset: 10})}))
	second := renderPlanned(applyPorts(dotenv.Parse([]byte(first)), []Entry{planOne(t, planOneParams{Content: first, Base: 5432, Offset: 10})}))

	assert.Equal(t, first, second)
}

func TestApplyingThePlanKeepsEveryByteAroundTheValues(t *testing.T) {
	content := "export API_PORT=4002 # api\r\n" +
		"REDIS_URL='redis://u:pa$w0rd@localhost:6379/0'\r\n" +
		"MINIO_URL=\"http://localhost:9000\" # minio endpoint\r\n" +
		"UNRELATED='keep $me'\r\n"
	lines := dotenv.Parse([]byte(content))
	plan := Writes{bases: map[portRef]int{
		{Job: "api", Name: "PORT"}:   4002,
		{Job: "redis", Name: "PORT"}: 6379,
		{Job: "minio", Name: "PORT"}: 9000,
	}, block: 10, worktree: settling{Offset: 10}}.plan(planOfParams{
		Links: []project.EnvPortLink{
			{File: ".env", Key: "API_PORT", Job: "api", Port: "PORT"},
			{File: ".env", Key: "REDIS_URL", Job: "redis", Port: "PORT"},
			{File: ".env", Key: "MINIO_URL", Job: "minio", Port: "PORT"},
		},
		Lines: map[string][]dotenv.Line{".env": lines},
	})

	want := "export API_PORT=4012 # api\r\n" +
		"REDIS_URL='redis://u:pa$w0rd@localhost:6389/0'\r\n" +
		"MINIO_URL=\"http://localhost:9010\" # minio endpoint\r\n" +
		"UNRELATED='keep $me'\r\n"
	assert.Equal(t, want, renderPlanned(applyPorts(lines, plan.Entries)))
}

func TestAPortValueRewindsToItsBase(t *testing.T) {
	cases := []struct {
		name  string
		value string
		base  int
		want  string
	}{
		{"a shifted port rewinds to its base", "postgres://localhost:5442/app", 5432, "postgres://localhost:5432/app"},
		{"the base is already the base", "postgres://localhost:5432/app", 5432, "postgres://localhost:5432/app"},
		{"another worktree's offset rewinds too", "postgres://localhost:5472/app", 5432, "postgres://localhost:5432/app"},
		{"a port off the block is left alone", "postgres://localhost:6000/app", 5432, "postgres://localhost:6000/app"},
		{"a port below the base is left alone", "postgres://localhost:5422/app", 5432, "postgres://localhost:5422/app"},
		{"two candidates leave the value alone", "http://a:3000/b:3010", 3000, "http://a:3000/b:3010"},
		{"an unrelated number is ignored", "http://localhost:3010/api/v1", 3000, "http://localhost:3000/api/v1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, reducePortValue(reducePortParams{Value: tc.value, Base: tc.base, Block: 10}))
		})
	}
}

func TestTwoWorktreesSpellingsOfOneValueReduceEqual(t *testing.T) {
	reduce := func(value string) string {
		return reducePortValue(reducePortParams{Value: value, Base: 5432, Block: 10})
	}

	main := reduce("postgres://u:pw@localhost:5432/app")

	assert.Equal(t, main, reduce("postgres://u:pw@localhost:5442/app"))
	assert.Equal(t, main, reduce("postgres://u:pw@localhost:5472/app"))
}

func TestAGenuineDifferenceSurvivesTheReduction(t *testing.T) {
	reduce := func(value string) string {
		return reducePortValue(reducePortParams{Value: value, Base: 5432, Block: 10})
	}

	assert.NotEqual(t, reduce("postgres://u:old@localhost:5432/app"), reduce("postgres://u:new@localhost:5442/app"))
}

func TestPortBasesAreKeyedByTheirJobAndName(t *testing.T) {
	config := project.Run{Jobs: []project.Job{
		{Name: "db", Ports: map[string]int{"POSTGRES_PORT": 5432}},
		{Name: "api", Ports: map[string]int{"API_PORT": 3000}},
	}}

	assert.Equal(t, map[portRef]int{{Job: "db", Name: "POSTGRES_PORT"}: 5432, {Job: "api", Name: "API_PORT"}: 3000}, portBases(config))
}

func TestAKeyFollowingTwoPortsMovesBoth(t *testing.T) {
	plan := Writes{bases: map[portRef]int{
		{Job: "web", Name: "VITE_PORT"}:   5173,
		{Job: "admin", Name: "VITE_PORT"}: 5174,
	}, block: 10, worktree: settling{Offset: 10}}.plan(planOfParams{
		Links: []project.EnvPortLink{
			{File: ".env", Key: "CORS_ORIGIN", Job: "web", Port: "VITE_PORT"},
			{File: ".env", Key: "CORS_ORIGIN", Job: "admin", Port: "VITE_PORT"},
		},
		Lines: map[string][]dotenv.Line{".env": dotenv.Parse([]byte("CORS_ORIGIN=http://localhost:5173,http://localhost:5174\n"))},
	})

	require.Len(t, plan.Entries, 1)
	entry := plan.Entries[0]
	assert.Equal(t, StatusRewrite, entry.Status)
	assert.Equal(t, "http://localhost:5183,http://localhost:5184", entry.NewValue)
	assert.Len(t, entry.Moves, 2)
}

func TestAPortNoJobDeclaresIsLeftAlone(t *testing.T) {
	plan := Writes{bases: map[portRef]int{{Job: "web", Name: "VITE_PORT"}: 5173}, block: 10, worktree: settling{Offset: 10}}.plan(planOfParams{
		Links: []project.EnvPortLink{{File: ".env", Key: "CORS_ORIGIN", Job: "web", Port: "VITE_PORT"}},
		Lines: map[string][]dotenv.Line{".env": dotenv.Parse([]byte("CORS_ORIGIN=http://localhost:5173,https://app.example.com:8443\n"))},
	})

	require.Len(t, plan.Entries, 1)
	assert.Equal(t, "http://localhost:5183,https://app.example.com:8443", plan.Entries[0].NewValue)
}

func TestARedactedPlanCarriesNoValueReadFromAFile(t *testing.T) {
	redacted := Redact(redactedPlanFixture())

	for _, entry := range redacted.Entries {
		assert.Empty(t, entry.CurrentValue, entry.Key)
		assert.Empty(t, entry.NewValue, entry.Key)
		assert.Empty(t, entry.ForeignHost, entry.Key)
	}
	assert.Equal(t, StatusRewrite, redacted.Entries[1].Status)
	assert.Equal(t, 5432, redacted.Entries[1].Base)
}

func TestARedactedPlanKeepsTheValuesWtmWrites(t *testing.T) {
	redacted := Redact(redactedPlanFixture())

	assert.Equal(t, "app-feat-a", redacted.Owned[0].Value)
}

func TestRedactingLeavesThePlanItReadIntact(t *testing.T) {
	plan := redactedPlanFixture()

	Redact(plan)

	assert.Equal(t, redactedPlanFixture(), plan)
}

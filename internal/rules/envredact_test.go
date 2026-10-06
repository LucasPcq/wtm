package rules

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

const fakeSecret = "s3cr3t-value"

func redactFixture() domain.EnvSyncResult {
	return domain.EnvSyncResult{
		Files: []domain.EnvFileResult{{
			Target: ".env",
			Diff: domain.EnvDiff{Mode: domain.EnvModeRefresh, Entries: []domain.EnvKeyDiff{
				{Key: "WEB_PORT", Status: domain.EnvKeyResolved, CurrentValue: "3010"},
				{Key: "REALM", Status: domain.EnvKeyResolved, CurrentValue: "app-feat-a"},
				{Key: domain.EnvComposeProjectName, Status: domain.EnvKeyResolved, CurrentValue: "app-feat-a"},
				{Key: "CLIENT_SECRET", Status: domain.EnvKeyConflict, CurrentValue: fakeSecret, ResolvedValue: fakeSecret + "-main", Source: domain.EnvSourceMain},
				{Key: "NEW_SECRET", Status: domain.EnvKeyResolved, ResolvedValue: fakeSecret + "-new", Source: domain.EnvSourceMain},
				{Key: "OLD_SECRET", Status: domain.EnvKeyOrphan, CurrentValue: fakeSecret + "-old"},
				{Key: "BLANK", Status: domain.EnvKeyResolved},
				{Key: "TODO", Status: domain.EnvKeyMissing, Placeholder: "change-me"},
			}},
		}},
		Ports: domain.EnvPortPlan{
			Entries: []domain.EnvPortEntry{{File: ".env", Key: "WEB_PORT", Status: domain.EnvPortStatusUnchanged, CurrentValue: "3010"}},
			Owned:   []domain.EnvOwnedEntry{{File: ".env", Key: "REALM", Value: "app-feat-a"}},
		},
	}
}

func entryOf(t *testing.T, result domain.EnvSyncResult, key string) domain.EnvKeyDiff {
	t.Helper()
	for _, e := range result.Files[0].Diff.Entries {
		if e.Key == key {
			return e
		}
	}
	t.Fatalf("no entry for %s", key)
	return domain.EnvKeyDiff{}
}

// The leak this guards against: `wtm env --check --output json` wrote every
// key's value, secrets included, into an agent's context.
func TestRedactEnvResultWithholdsTheValuesWtmDoesNotWrite(t *testing.T) {
	redacted := RedactEnvResult(redactFixture())

	body, err := json.Marshal(redacted)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), fakeSecret) {
		t.Errorf("redacted JSON still carries a secret: %s", body)
	}
	for _, key := range []string{"CLIENT_SECRET", "NEW_SECRET", "OLD_SECRET"} {
		e := entryOf(t, redacted, key)
		if !e.Redacted || e.CurrentValue != "" || e.ResolvedValue != "" {
			t.Errorf("%s = %+v, want its values withheld and redacted set", key, e)
		}
	}
	if added := entryOf(t, redacted, "NEW_SECRET"); added.Source != domain.EnvSourceMain {
		t.Errorf("an addition keeps its source, so a reader still tells it from a key in sync: %+v", added)
	}
}

func TestRedactEnvResultKeepsTheValuesWtmWrites(t *testing.T) {
	redacted := RedactEnvResult(redactFixture())

	for key, want := range map[string]string{"WEB_PORT": "3010", "REALM": "app-feat-a", domain.EnvComposeProjectName: "app-feat-a"} {
		e := entryOf(t, redacted, key)
		if e.Redacted || e.CurrentValue != want {
			t.Errorf("%s = %+v, want its value %q shown", key, e, want)
		}
	}
}

// A flag that says "a value is here and you cannot see it" is only true when
// one was withheld: an empty value or a key the .env lacks hides nothing, and
// a placeholder comes from the committed template.
func TestRedactEnvResultFlagsOnlyAWithheldValue(t *testing.T) {
	redacted := RedactEnvResult(redactFixture())

	if e := entryOf(t, redacted, "BLANK"); e.Redacted {
		t.Errorf("BLANK = %+v, want no redacted flag for an empty value", e)
	}
	if e := entryOf(t, redacted, "TODO"); e.Redacted || e.Placeholder != "change-me" {
		t.Errorf("TODO = %+v, want the template placeholder kept and no flag", e)
	}
}

func TestRedactEnvResultLeavesItsInputIntact(t *testing.T) {
	result := redactFixture()
	RedactEnvResult(result)

	if e := entryOf(t, result, "CLIENT_SECRET"); e.CurrentValue != fakeSecret {
		t.Errorf("input entry = %+v, want it untouched — the report classifies on it", e)
	}
}

func TestEnvKeyRowsWithholdAConflictValueWtmDoesNotWrite(t *testing.T) {
	result := redactFixture()

	rows := EnvKeyRows(EnvKeyRowsParams{File: result.Files[0], Check: true, Managed: EnvManagedKeys(EnvManagedKeysParams{Plan: result.Ports, Target: ".env"})})

	for _, row := range rows {
		if strings.Contains(row.Text, fakeSecret) {
			t.Errorf("row %q prints a secret", row.Text)
		}
	}
	if !strings.Contains(rowText(rows, "CLIENT_SECRET"), "differs from main") {
		t.Errorf("conflict row = %q, want it to say the values differ", rowText(rows, "CLIENT_SECRET"))
	}
}

func TestEnvKeyRowsShowEveryValueWhenAsked(t *testing.T) {
	result := redactFixture()

	rows := EnvKeyRows(EnvKeyRowsParams{File: result.Files[0], Check: true, ShowValues: true})

	if !strings.Contains(rowText(rows, "CLIENT_SECRET"), fakeSecret) {
		t.Errorf("conflict row = %q, want both values with --show-values", rowText(rows, "CLIENT_SECRET"))
	}
}

func rowText(rows []domain.EnvKeyRow, key string) string {
	for _, row := range rows {
		if strings.HasPrefix(row.Text, key+" ") {
			return row.Text
		}
	}
	return ""
}

func TestMaskURLPasswordMasksOnlyAURLPassword(t *testing.T) {
	cases := map[string]string{
		"postgres://app:hunter2@localhost:5432/db": "postgres://app:***@localhost:5432/db",
		"redis://:hunter2@127.0.0.1:6379":          "redis://:***@127.0.0.1:6379",
		"amqp://u:p%40ss@localhost:5672/vhost?x=1": "amqp://u:***@localhost:5672/vhost?x=1",
		"3010":           "3010",
		"localhost:3010": "localhost:3010",
		"http://localhost:3010/callback?next=/a@b":   "http://localhost:3010/callback?next=/a@b",
		"postgres://app@localhost:5432/db":           "postgres://app@localhost:5432/db",
		"postgres://app:@localhost:5432/db":          "postgres://app:***@localhost:5432/db",
		"http://[::1:3010":                           "http://[::1:3010",
		"not a url: user:pass@host":                  "not a url: user:pass@host",
		"":                                           "",
		"http://localhost:3010,http://a:b@localhost": "http://localhost:3010,http://a:b@localhost",
	}
	for in, want := range cases {
		if got := MaskURLPassword(in); got != want {
			t.Errorf("MaskURLPassword(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactEnvResultMasksThePasswordOfAPortLinkedURL(t *testing.T) {
	result := domain.EnvSyncResult{
		Files: []domain.EnvFileResult{{Target: ".env", Diff: domain.EnvDiff{Entries: []domain.EnvKeyDiff{
			{Key: "DATABASE_URL", Status: domain.EnvKeyResolved, CurrentValue: "postgres://app:" + fakeSecret + "@localhost:5442/db"},
		}}}},
		Ports: domain.EnvPortPlan{Entries: []domain.EnvPortEntry{{
			File: ".env", Key: "DATABASE_URL", Status: domain.EnvPortStatusRewrite,
			CurrentValue: "postgres://app:" + fakeSecret + "@localhost:5432/db",
			NewValue:     "postgres://app:" + fakeSecret + "@localhost:5442/db",
		}}},
	}

	redacted := RedactEnvResult(result)

	body, _ := json.Marshal(redacted)
	if strings.Contains(string(body), fakeSecret) {
		t.Errorf("redacted JSON carries the password: %s", body)
	}
	if got := redacted.Ports.Entries[0].NewValue; got != "postgres://app:***@localhost:5442/db" {
		t.Errorf("new_value = %q, want only the password masked", got)
	}
	if got := entryOf(t, redacted, "DATABASE_URL"); got.Redacted || got.CurrentValue != "postgres://app:***@localhost:5442/db" {
		t.Errorf("entry = %+v, want its value shown with the password masked", got)
	}
	if result.Ports.Entries[0].NewValue == redacted.Ports.Entries[0].NewValue {
		t.Error("the input plan was masked in place")
	}
}

func TestEnvKeyRowsMaskThePasswordOfAManagedConflict(t *testing.T) {
	file := fileWith(domain.EnvKeyDiff{Key: "DATABASE_URL", Status: domain.EnvKeyConflict, Source: domain.EnvSourceMain,
		CurrentValue: "postgres://app:" + fakeSecret + "@localhost:5442/db", ResolvedValue: "postgres://app:other@localhost:5442/db"})
	managed := map[string]bool{"DATABASE_URL": true}

	masked := EnvKeyRows(EnvKeyRowsParams{File: file, Check: true, Managed: managed})
	shown := EnvKeyRows(EnvKeyRowsParams{File: file, Check: true, Managed: managed, ShowValues: true})

	if strings.Contains(masked[0].Text, fakeSecret) || !strings.Contains(masked[0].Text, "app:***@") {
		t.Errorf("row = %q, want the password masked", masked[0].Text)
	}
	if !strings.Contains(shown[0].Text, fakeSecret) {
		t.Errorf("row = %q, want the full value with --show-values", shown[0].Text)
	}
}

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
				{Key: "DATABASE_URL", Status: domain.EnvKeyConflict, CurrentValue: "postgres://app:" + fakeSecret + "@localhost:5442/db", ResolvedValue: "postgres://app:" + fakeSecret + "@localhost:5432/db", Source: domain.EnvSourceMain},
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
			Entries: []domain.EnvPortEntry{
				{File: ".env", Key: "WEB_PORT", Status: domain.EnvPortStatusUnchanged, CurrentValue: "3010"},
				{File: ".env", Key: "DATABASE_URL", Status: domain.EnvPortStatusRewrite,
					CurrentValue: "postgres://app:" + fakeSecret + "@localhost:5432/db", NewValue: "postgres://app:" + fakeSecret + "@localhost:5442/db"},
			},
			Owned: []domain.EnvOwnedEntry{{File: ".env", Key: "REALM", Value: "app-feat-a"}},
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
	for _, key := range []string{"CLIENT_SECRET", "NEW_SECRET", "OLD_SECRET", "DATABASE_URL"} {
		e := entryOf(t, redacted, key)
		if !e.Redacted || e.CurrentValue != "" || e.ResolvedValue != "" {
			t.Errorf("%s = %+v, want its values withheld and redacted set", key, e)
		}
	}
	if added := entryOf(t, redacted, "NEW_SECRET"); added.Source != domain.EnvSourceMain {
		t.Errorf("an addition keeps its source, so a reader still tells it from a key in sync: %+v", added)
	}
}

// LUC-279: what wtm writes reaches the report through the port plan — an owned
// value whole, a link's origins — never through the values the reconciliation
// read from the files, which are the user's.
func TestRedactEnvResultKeepsWhatWtmWrites(t *testing.T) {
	redacted := RedactEnvResult(redactFixture())

	if got := redacted.Ports.Owned[0].Value; got != "app-feat-a" {
		t.Errorf("owned value = %q, want wtm's value whole", got)
	}
	if link := redacted.Ports.Entries[1]; link.CurrentValue != "" || link.NewValue != "" {
		t.Errorf("link = %+v, want no value", link)
	}
	for _, key := range []string{"WEB_PORT", "REALM", domain.EnvComposeProjectName} {
		if e := entryOf(t, redacted, key); !e.Redacted || e.CurrentValue != "" {
			t.Errorf("%s = %+v, want its value withheld: the file's value is not wtm's", key, e)
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
	if got := result.Ports.Entries[1].NewValue; !strings.Contains(got, fakeSecret) {
		t.Errorf("input plan = %q, want it untouched", got)
	}
}

func TestEnvKeyRowsWithholdEveryConflictValue(t *testing.T) {
	result := redactFixture()

	rows := EnvKeyRows(EnvKeyRowsParams{File: result.Files[0], Check: true})

	for _, row := range rows {
		if strings.Contains(row.Text, fakeSecret) {
			t.Errorf("row %q prints a secret", row.Text)
		}
	}
	for _, key := range []string{"CLIENT_SECRET", "DATABASE_URL"} {
		if !strings.Contains(rowText(rows, key), "differs from main") {
			t.Errorf("conflict row = %q, want it to say the values differ", rowText(rows, key))
		}
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

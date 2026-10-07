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
		"3010":                              "3010",
		"localhost:3010":                    "localhost:3010",
		"postgres://app@localhost:5432/db":  "postgres://app@localhost:5432/db",
		"postgres://app:@localhost:5432/db": "postgres://app:***@localhost:5432/db",
		"http://[::1:3010":                  "http://[::1:3010",
		"not a url: user:pass@host":         "not a url:***@host",
		"":                                  "",
		"http://localhost:3010,http://a:b@localhost": "http://localhost:3010,http://a:***@localhost",
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

// LUC-274 F3: every value below carries fakeSecret where net/url either fails
// or finds no userinfo, and each one used to come out whole.
func TestMaskURLPasswordMasksWhatNetURLCannotRead(t *testing.T) {
	cases := map[string]string{
		"mongodb://app:" + fakeSecret + "@h1:27017,h2:27018/db":                 "mongodb://app:***@h1:27017,h2:27018/db",
		"postgres://app:pa#" + fakeSecret + "@localhost:5432/db":                "postgres://app:***@localhost:5432/db",
		"postgres://app:pa/" + fakeSecret + "@localhost:5432/db":                "postgres://app:***@localhost:5432/db",
		"postgres://app:pa%zz" + fakeSecret + "@localhost:5432/db":              "postgres://app:***@localhost:5432/db",
		"http://localhost:3010,http://a:" + fakeSecret + "@localhost:3011":      "http://localhost:3010,http://a:***@localhost:3011",
		"host=localhost port=5432 password=" + fakeSecret + " dbname=app":       "host=localhost port=5432 password=*** dbname=app",
		"host=localhost password='" + fakeSecret + " x' dbname=app":             "host=localhost password=*** dbname=app",
		"postgres://localhost:5432/db?user=app&password=" + fakeSecret + "&x=1": "postgres://localhost:5432/db?user=app&password=***&x=1",
		"jdbc:postgresql://localhost:5432/db?user=app&password=" + fakeSecret:   "jdbc:postgresql://localhost:5432/db?user=app&password=***",
		"jdbc:sqlserver://localhost:1433;user=app;Password=" + fakeSecret:       "jdbc:sqlserver://localhost:1433;user=app;Password=***",
		"postgres://app:12#" + fakeSecret + "@localhost:5432/db":                "postgres://app:***@localhost:5432/db",
		"postgres://app:12/" + fakeSecret + "@localhost:5432/db":                "postgres://app:***@localhost:5432/db",
		"jdbc:mysql://app:" + fakeSecret + "@localhost:3306/db":                 "jdbc:mysql://app:***@localhost:3306/db",
		" postgres://app:" + fakeSecret + "@localhost:5432/db ":                 " postgres://app:***@localhost:5432/db ",
	}
	for in, want := range cases {
		if got := MaskURLPassword(in); got != want {
			t.Errorf("MaskURLPassword(%q) = %q, want %q", in, got, want)
		}
	}
}

// LUC-274: an "@" past a URL's authority cannot be told from a password that
// net/url read as a path or a fragment ("app:12#x@h"), so it is masked too.
func TestMaskURLPasswordOverMasksAnAtPastTheAuthority(t *testing.T) {
	cases := map[string]string{
		"http://localhost:3010/a@b?next=c@d": "http://localhost:***@d",
		"http://host:3000/users/@me":         "http://host:***@me",
		"http://host/users/@me":              "http://***@me",
	}
	for in, want := range cases {
		if got := MaskURLPassword(in); got != want {
			t.Errorf("MaskURLPassword(%q) = %q, want %q", in, got, want)
		}
	}
}

// LUC-274 F3: the shapes net/url already read must stay exactly as they were.
func TestMaskURLPasswordKeepsTheShapesItAlreadyHandled(t *testing.T) {
	cases := map[string]string{
		"postgres://app:p%40ss@localhost:5432/db":     "postgres://app:***@localhost:5432/db",
		"postgres://app:p@ss@localhost:5432/db":       "postgres://app:***@localhost:5432/db",
		"postgres://app:pw@[::1]:5432/db":             "postgres://app:***@[::1]:5432/db",
		"postgres://app:@localhost:5432/db":           "postgres://app:***@localhost:5432/db",
		"redis://:pw@localhost:6379":                  "redis://:***@localhost:6379",
		"http://localhost:3010,http://localhost:3011": "http://localhost:3010,http://localhost:3011",
		"localhost:3010":                              "localhost:3010",
	}
	for in, want := range cases {
		if got := MaskURLPassword(in); got != want {
			t.Errorf("MaskURLPassword(%q) = %q, want %q", in, got, want)
		}
	}
}

// LUC-274 F1: a switch to verbatim reported the source's value it put back
// whole — the restored keys are all ones wtm writes, a port-linked URL among them.
func TestRedactEnvResultMasksTheRestoredValues(t *testing.T) {
	result := domain.EnvSyncResult{Restored: []domain.EnvRestoredEntry{{
		File: ".env", Key: "DATABASE_URL",
		From: "postgres://app:" + fakeSecret + "@localhost:5442/db",
		To:   "postgres://app:" + fakeSecret + "@localhost:5432/db",
	}}}

	redacted := RedactEnvResult(result)

	body, _ := json.Marshal(redacted)
	if strings.Contains(string(body), fakeSecret) {
		t.Errorf("redacted JSON carries the password: %s", body)
	}
	if got := redacted.Restored[0]; got.From != "postgres://app:***@localhost:5442/db" || got.To != "postgres://app:***@localhost:5432/db" {
		t.Errorf("restored = %+v, want both values with the password masked", got)
	}
	if result.Restored[0].To == redacted.Restored[0].To {
		t.Error("the input restored entries were masked in place")
	}
}

// LUC-274 F2: an [[env]] key's current value is read before wtm writes its
// own, so it can be anything the user put there.
func TestRedactEnvResultMasksThePasswordOfAnOwnedKey(t *testing.T) {
	result := domain.EnvSyncResult{
		Files: []domain.EnvFileResult{{Target: ".env", Diff: domain.EnvDiff{Entries: []domain.EnvKeyDiff{
			{Key: "REALM", Status: domain.EnvKeyConflict, CurrentValue: "postgres://app:" + fakeSecret + "@localhost:5432/db", ResolvedValue: "app-feat-a"},
		}}}},
		Ports: domain.EnvPortPlan{Owned: []domain.EnvOwnedEntry{{File: ".env", Key: "REALM", Value: "app-feat-a"}}},
	}

	redacted := RedactEnvResult(result)

	if got := entryOf(t, redacted, "REALM"); got.Redacted || got.CurrentValue != "postgres://app:***@localhost:5432/db" || got.ResolvedValue != "app-feat-a" {
		t.Errorf("entry = %+v, want its values shown with the password masked", got)
	}
}

// LUC-274 F3: the text conflict line of a managed key printed what
// MaskURLPassword could not read.
func TestEnvKeyRowsMaskAConflictNetURLCannotRead(t *testing.T) {
	file := fileWith(domain.EnvKeyDiff{Key: "DATABASE_URL", Status: domain.EnvKeyConflict, Source: domain.EnvSourceMain,
		CurrentValue: "mongodb://app:" + fakeSecret + "@h1:3010,h2:3011/db", ResolvedValue: "host=h1 port=3010 password=" + fakeSecret})

	rows := EnvKeyRows(EnvKeyRowsParams{File: file, Check: true, Managed: map[string]bool{"DATABASE_URL": true}})

	if strings.Contains(rows[0].Text, fakeSecret) {
		t.Errorf("row = %q, want the passwords masked", rows[0].Text)
	}
}

// LUC-274 F3: the port table and the restored rows cut a value at its last
// "@", which a password outside a URL's userinfo does not have.
func TestElideEnvValueMasksAPasswordWithoutUserinfo(t *testing.T) {
	got := ElideEnvValue(ElideEnvValueParams{Value: "host=h port=3010 password=" + fakeSecret})

	if strings.Contains(got, fakeSecret) {
		t.Errorf("ElideEnvValue() = %q, want the password masked", got)
	}
}

// LUC-278: a comma inside a URL's userinfo split the value before net/url read
// it, so neither half looked like a credential and the password came out whole.
func TestMaskURLPasswordKeepsACommaInsideAPassword(t *testing.T) {
	cases := map[string]string{
		"postgres://app:ab," + fakeSecret + "@localhost:5432/db":                    "postgres://app:***@localhost:5432/db",
		"host=localhost password=ab," + fakeSecret + " dbname=app":                  "host=localhost password=*** dbname=app",
		"http://localhost:3010,postgres://app:ab," + fakeSecret + "@localhost:5432": "http://localhost:3010,postgres://app:***@localhost:5432",
		"mongodb://app:pw@h1:27017,h2:27018/db":                                     "mongodb://app:***@h1:27017,h2:27018/db",
	}
	for in, want := range cases {
		if got := MaskURLPassword(in); got != want {
			t.Errorf("MaskURLPassword(%q) = %q, want %q", in, got, want)
		}
	}
}

// LUC-278: a credential without a scheme — a go-sql-driver DSN, a redis
// address — was printed whole; the restored rows used to elide it to "…@host".
func TestMaskURLPasswordMasksASchemelessCredential(t *testing.T) {
	cases := map[string]string{
		"app:" + fakeSecret + "@tcp(localhost:3306)/db": "app:***@tcp(localhost:3306)/db",
		"app:" + fakeSecret + "@localhost:6379":         "app:***@localhost:6379",
		"not a url: user:" + fakeSecret + "@host":       "not a url:***@host",
		"noreply@example.com":                           "noreply@example.com",
	}
	for in, want := range cases {
		if got := MaskURLPassword(in); got != want {
			t.Errorf("MaskURLPassword(%q) = %q, want %q", in, got, want)
		}
	}
}

// LUC-278: net/url ends the authority at "?" or "#", but the host a client
// connects to follows the last "@" before the path, so the text between is the
// password.
func TestMaskURLPasswordReadsTheAuthorityUpToThePath(t *testing.T) {
	cases := map[string]string{
		"mongodb://app:pw@x?" + fakeSecret + "@h1:27017/db": "mongodb://app:***@h1:27017/db",
		"mongodb://app@x#:" + fakeSecret + "@h1:27017/db":   "mongodb://app@x#:***@h1:27017/db",
		"postgres://app:pw@x#" + fakeSecret + "@host/db":    "postgres://app:***@host/db",
	}
	for in, want := range cases {
		if got := MaskURLPassword(in); got != want {
			t.Errorf("MaskURLPassword(%q) = %q, want %q", in, got, want)
		}
	}
}

// LUC-278: a password= value ends where the format it is written in ends it —
// whitespace in a libpq DSN, ";" in an ADO.NET or jdbc string, "&" in a query —
// and each one left the rest of the password visible.
func TestMaskURLPasswordMasksAPasswordPairToItsEnd(t *testing.T) {
	cases := map[string]string{
		"host=localhost password = " + fakeSecret + " dbname=app":               "host=localhost password = *** dbname=app",
		"host=localhost password=ab;" + fakeSecret + " dbname=app":              "host=localhost password=*** dbname=app",
		"host=localhost password=ab&" + fakeSecret + " dbname=app":              "host=localhost password=*** dbname=app",
		`host=localhost password='ab\'` + fakeSecret + `' dbname=app`:           "host=localhost password=*** dbname=app",
		`host=localhost password=ab\ ` + fakeSecret + " dbname=app":             "host=localhost password=*** dbname=app",
		"host=localhost sslpassword=" + fakeSecret + " dbname=app":              "host=localhost sslpassword=*** dbname=app",
		"Server=x;Password=ab " + fakeSecret + ";Database=d":                    "Server=x;Password=***;Database=d",
		`Server=x;Password="ab;` + fakeSecret + `";Database=d`:                  "Server=x;Password=***;Database=d",
		`Server=x;Password='ab;` + fakeSecret + `';Database=d`:                  "Server=x;Password=***;Database=d",
		"jdbc:sqlserver://h:1433;user=app;password={ab;" + fakeSecret + "};x=1": "jdbc:sqlserver://h:1433;user=app;password=***;x=1",
		"postgres://h/db?user=app&password=ab'" + fakeSecret + "&x=1":           "postgres://h/db?user=app&password=***&x=1",
		"postgres://h/db?PASSWORD=" + fakeSecret + "#frag":                      "postgres://h/db?PASSWORD=***#frag",
	}
	for in, want := range cases {
		if got := MaskURLPassword(in); got != want {
			t.Errorf("MaskURLPassword(%q) = %q, want %q", in, got, want)
		}
	}
}

// LUC-278: the restored rows print a value whole since LUC-274, so a
// credential MaskURLPassword misses is no longer cut away at its last "@".
func TestEnvRestoredRowsMaskASchemelessCredential(t *testing.T) {
	rows := EnvRestoredRows([]domain.EnvRestoredEntry{{
		File: ".env", Key: "MYSQL_DSN",
		From: "app:" + fakeSecret + "@tcp(localhost:3316)/db",
		To:   "app:" + fakeSecret + "@tcp(localhost:3306)/db",
	}}, ".env")

	if len(rows) != 1 || strings.Contains(rows[0], fakeSecret) {
		t.Errorf("rows = %q, want the password masked", rows)
	}
}

// LUC-278: ODBC and ADO.NET spell the password key "Pwd", which was printed whole.
func TestMaskURLPasswordMasksAPwdPair(t *testing.T) {
	cases := map[string]string{
		"Driver={ODBC};Server=x;Uid=app;Pwd=" + fakeSecret + ";Database=d": "Driver={ODBC};Server=x;Uid=app;Pwd=***;Database=d",
		"Server=x;PWD={ab;" + fakeSecret + "}":                             "Server=x;PWD=***",
		"host=localhost pwd = " + fakeSecret + " dbname=app":               "host=localhost pwd = *** dbname=app",
		"postgres://h/db?pwd=" + fakeSecret + "&x=1":                       "postgres://h/db?pwd=***&x=1",
	}
	for in, want := range cases {
		if got := MaskURLPassword(in); got != want {
			t.Errorf("MaskURLPassword(%q) = %q, want %q", in, got, want)
		}
	}
}

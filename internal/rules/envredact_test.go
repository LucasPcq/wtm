package rules

import (
	"encoding/json"
	"slices"
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
	link := redacted.Ports.Entries[1]
	want := []domain.EnvOriginMove{{From: "localhost:5432", To: "localhost:5442"}}
	if !slices.Equal(link.Origins, want) || link.CurrentValue != "" || link.NewValue != "" {
		t.Errorf("link = %+v, want its origins %v and no value", link, want)
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

// leakShapes are every value the LUC-274 and LUC-278 reviews found a password
// printed from, and the shapes the deny-list parser was extended for. Each
// carries fakeSecret where a client reads a credential.
var leakShapes = []string{
	"postgres://app:" + fakeSecret + "@localhost:5432/db",
	"redis://:" + fakeSecret + "@127.0.0.1:6379",
	"amqp://u:" + fakeSecret + "%40ss@localhost:5672/vhost?x=1",
	"not a url: user:" + fakeSecret + "@localhost:3000",
	"http://localhost:3000,http://a:" + fakeSecret + "@localhost:3001",
	"mongodb://app:" + fakeSecret + "@localhost:27017,localhost:27018/db",
	"postgres://app:pa#" + fakeSecret + "@localhost:5432/db",
	"postgres://app:pa/" + fakeSecret + "@localhost:5432/db",
	"postgres://app:pa%zz" + fakeSecret + "@localhost:5432/db",
	"postgres://app:12#" + fakeSecret + "@localhost:5432/db",
	"postgres://app:12/" + fakeSecret + "@localhost:5432/db",
	"postgres://app:ab," + fakeSecret + "@localhost:5432/db",
	"postgres://app:pw@[::1]:5432/" + fakeSecret,
	"host=localhost port=5432 password=" + fakeSecret + " dbname=app",
	"host=localhost port=5432 password='" + fakeSecret + " x' dbname=app",
	"host=localhost port=5432 password = " + fakeSecret + " dbname=app",
	`host=localhost port=5432 password='ab\'` + fakeSecret + `' dbname=app`,
	`host=localhost port=5432 password=ab\ ` + fakeSecret + " dbname=app",
	"host=localhost port=5432 sslpassword=" + fakeSecret + " dbname=app",
	"host=localhost port=5432 password=ab," + fakeSecret + " dbname=app",
	"host=h port=5432 password=ab,c://" + fakeSecret + " dbname=x",
	"host=a; port=5432; password=ab;" + fakeSecret + " dbname=y",
	"host=localhost port=5432 pwd = " + fakeSecret + " dbname=app",
	"postgres://localhost:5432/db?user=app&password=" + fakeSecret + "&x=1",
	"postgres://localhost:5432/db?PASSWORD=" + fakeSecret + "#frag",
	"postgres://localhost:5432/db?pwd=" + fakeSecret + "&x=1",
	"jdbc:postgresql://localhost:5432/db?user=app&password=" + fakeSecret,
	"jdbc:sqlserver://localhost:1433;user=app;Password=" + fakeSecret,
	"jdbc:sqlserver://localhost:1433;user=app;password={ab;" + fakeSecret + "};x=1",
	"jdbc:mysql://app:" + fakeSecret + "@localhost:3306/db",
	"jdbc:mysql://app:" + fakeSecret + "@localhost:3306/db?next=http://x/y",
	"Server=localhost,1433;Password=ab " + fakeSecret + ";Database=d",
	`Server=localhost,1433;Password="ab;` + fakeSecret + `";Database=d`,
	"Driver={ODBC};Server=localhost,1433;Uid=app;Pwd=" + fakeSecret + ";Database=d",
	"app:" + fakeSecret + "@tcp(localhost:3306)/db",
	"app:" + fakeSecret + "@localhost:6379",
	"app:" + fakeSecret + "@tcp(localhost:3306)/db?redirect=http://x",
	"mongodb://app:pw@x?" + fakeSecret + "@localhost:27017/db",
	"mongodb://app@x#:" + fakeSecret + "@localhost:27017/db",
	"postgres://app:pw@localhost:5432/db?next=http://u:" + fakeSecret + "@x",
	"http://user:" + fakeSecret + "#x@api.staging.example.com/v1",
	" postgres://app:" + fakeSecret + "@localhost:5432/db ",
	"http://localhost:3000/a@" + fakeSecret + "?next=c@d",
	"postgres://app:5432@localhost:5432/" + fakeSecret,
}

var leakPorts = strings.NewReplacer("5432", "5442", "6379", "6389", "5672", "5682", "3000", "3010",
	"3001", "3011", "27017", "27027", "27018", "27028", "1433", "1443", "3306", "3316")

// LUC-279: a value's secret reaches no surface of `wtm env` without
// --show-values — whatever its shape, since the report keeps only what wtm
// wrote rather than masking what a parser recognises.
func TestEnvReportCarriesNoSecretWithoutShowValues(t *testing.T) {
	for _, from := range leakShapes {
		result := leakResult(from, leakPorts.Replace(from))

		body, err := json.Marshal(RedactEnvResult(result))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), fakeSecret) {
			t.Errorf("JSON for %q carries the secret: %s", from, body)
		}
		for _, line := range leakTextLines(result) {
			if strings.Contains(line, fakeSecret) {
				t.Errorf("text for %q carries the secret: %q", from, line)
			}
		}
	}
}

func TestEnvReportShowsTheValuesWhenAsked(t *testing.T) {
	result := leakResult(leakShapes[0], leakPorts.Replace(leakShapes[0]))

	body, _ := json.Marshal(AnnotateEnvOrigins(result))
	if !strings.Contains(string(body), fakeSecret) {
		t.Errorf("--show-values JSON = %s, want every value whole", body)
	}
	rows := EnvRestoredRows(EnvRestoredRowsParams{Entries: result.Restored, File: ".env", ShowValues: true})
	if !strings.Contains(rows[0], fakeSecret) {
		t.Errorf("--show-values restored row = %q, want the values whole", rows[0])
	}
}

func leakResult(from, to string) domain.EnvSyncResult {
	foreign := RewriteOrigin(RewriteOriginParams{Value: from, Origin: "http://web.feat-x.app.localhost:1355", JobLabel: "web", Project: "app", Base: 3000, Resolved: 3010})
	return domain.EnvSyncResult{
		Check: true,
		Files: []domain.EnvFileResult{{Target: ".env", Diff: domain.EnvDiff{Entries: []domain.EnvKeyDiff{
			{Key: "URL", Status: domain.EnvKeyConflict, CurrentValue: to, ResolvedValue: from, Source: domain.EnvSourceMain},
			{Key: "NEW", Status: domain.EnvKeyResolved, ResolvedValue: from, Source: domain.EnvSourceMain},
			{Key: "OLD", Status: domain.EnvKeyOrphan, CurrentValue: from},
		}}}},
		Ports: domain.EnvPortPlan{Entries: []domain.EnvPortEntry{
			{File: ".env", Key: "URL", Port: "db", Status: domain.EnvPortStatusRewrite, CurrentValue: from, NewValue: to},
			{File: ".env", Key: "API", Port: "web", Status: foreign.Status, CurrentValue: from, NewValue: foreign.Value, ForeignHost: foreign.ForeignHost},
		}},
		Restored: []domain.EnvRestoredEntry{
			{File: ".env", Key: "URL", From: to, To: from},
			{File: ".env", Key: "GONE", From: to, Removed: true},
		},
	}
}

func leakTextLines(result domain.EnvSyncResult) []string {
	var lines []string
	for _, row := range EnvKeyRows(EnvKeyRowsParams{File: result.Files[0], Check: true}) {
		lines = append(lines, row.Text)
	}
	lines = append(lines, EnvRestoredRows(EnvRestoredRowsParams{Entries: result.Restored, File: ".env"})...)
	lines = append(lines, EnvRestoreRecapLines(result.Restored)...)
	lines = append(lines, EnvPortTableLines(EnvPortTableParams{Plan: result.Ports})...)
	lines = append(lines, EnvPortAnomalyLines(result.Ports)...)
	return lines
}

func TestEnvOriginMoves(t *testing.T) {
	cases := []struct {
		from, to string
		want     []domain.EnvOriginMove
	}{
		{"3000", "3010", []domain.EnvOriginMove{{From: "3000", To: "3010"}}},
		{"postgres://app:pw@localhost:5432/db", "postgres://app:pw@localhost:5442/db", []domain.EnvOriginMove{{From: "localhost:5432", To: "localhost:5442"}}},
		{"postgres://app:pw@db.internal:5432/db", "postgres://app:pw@db.internal:5442/db", []domain.EnvOriginMove{{From: ":5432", To: ":5442"}}},
		{"host=localhost port=5432 password=x", "host=localhost port=5442 password=x", []domain.EnvOriginMove{{From: ":5432", To: ":5442"}}},
		{"http://[::1]:3000", "http://[::1]:3010", []domain.EnvOriginMove{{From: "[::1]:3000", To: "[::1]:3010"}}},
		{"http://localhost:3000/cb", "http://web.feat-x.app.localhost:1355/cb", []domain.EnvOriginMove{{From: "localhost:3000", To: "web.feat-x.app.localhost:1355"}}},
		{"http://localhost:3000,http://localhost:3001", "http://localhost:3010,http://localhost:3011",
			[]domain.EnvOriginMove{{From: "localhost:3000", To: "localhost:3010"}, {From: "localhost:3001", To: "localhost:3011"}}},
	}
	for _, c := range cases {
		got, ok := EnvOriginMoves(EnvOriginMovesParams{From: c.from, To: c.to})
		if !ok || !slices.Equal(got, c.want) {
			t.Errorf("EnvOriginMoves(%q, %q) = %v, %v; want %v", c.from, c.to, got, ok, c.want)
		}
	}
}

// A difference wtm did not make — a password, a user, a database — is not an
// origin, and reading it as one would print it.
func TestEnvOriginMovesRefusesADifferenceThatIsNotAPort(t *testing.T) {
	cases := [][2]string{
		{"postgres://app:12345@localhost:5432/db", "postgres://app:54321@localhost:5432/db"},
		{"host=localhost port=5432 password=12345", "host=localhost port=5432 password=54321"},
		{"postgres://app:old@localhost:5432/db", "postgres://app:new@localhost:5442/db"},
		{"postgres://localhost:5432/a", "postgres://localhost:5432/b"},
		{"same", "same"},
	}
	for _, c := range cases {
		if got, ok := EnvOriginMoves(EnvOriginMovesParams{From: c[0], To: c[1]}); ok {
			t.Errorf("EnvOriginMoves(%q, %q) = %v, want no reading", c[0], c[1], got)
		}
	}
}

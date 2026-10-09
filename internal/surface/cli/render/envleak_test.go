package output

import (
	"bytes"
	"strings"
	"testing"
	"unicode"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// leakMarker is where a shape carries its credential. The guarantee does not
// rest on it: the secret is also inserted at every position of the value.
const leakMarker = "\x00"

var leakShapes = []string{
	"postgres://app:" + leakMarker + "@localhost:5432/db",
	"redis://:" + leakMarker + "@127.0.0.1:6379",
	"amqp://u:" + leakMarker + "%40ss@localhost:5672/vhost?x=1",
	"not a url: user:" + leakMarker + "@localhost:3000",
	"http://localhost:3000,http://a:" + leakMarker + "@localhost:3001",
	"mongodb://app:" + leakMarker + "@localhost:27017,localhost:27018/db",
	"postgres://app:pa#" + leakMarker + "@localhost:5432/db",
	"postgres://app:pa/" + leakMarker + "@localhost:5432/db",
	"postgres://app:pa%zz" + leakMarker + "@localhost:5432/db",
	"postgres://app:12#" + leakMarker + "@localhost:5432/db",
	"postgres://app:12/" + leakMarker + "@localhost:5432/db",
	"postgres://app:ab," + leakMarker + "@localhost:5432/db",
	"postgres://app:pw@[::1]:5432/" + leakMarker,
	"host=localhost port=5432 password=" + leakMarker + " dbname=app",
	"host=localhost port=5432 password='" + leakMarker + " x' dbname=app",
	"host=localhost port=5432 password = " + leakMarker + " dbname=app",
	`host=localhost port=5432 password='ab\'` + leakMarker + `' dbname=app`,
	`host=localhost port=5432 password=ab\ ` + leakMarker + " dbname=app",
	"host=localhost port=5432 sslpassword=" + leakMarker + " dbname=app",
	"host=localhost port=5432 password=ab," + leakMarker + " dbname=app",
	"host=h port=5432 password=ab,c://" + leakMarker + " dbname=x",
	"host=a; port=5432; password=ab;" + leakMarker + " dbname=y",
	"host=localhost port=5432 pwd = " + leakMarker + " dbname=app",
	"postgres://localhost:5432/db?user=app&password=" + leakMarker + "&x=1",
	"postgres://localhost:5432/db?PASSWORD=" + leakMarker + "#frag",
	"postgres://localhost:5432/db?pwd=" + leakMarker + "&x=1",
	"jdbc:postgresql://localhost:5432/db?user=app&password=" + leakMarker,
	"jdbc:sqlserver://localhost:1433;user=app;Password=" + leakMarker,
	"jdbc:sqlserver://localhost:1433;user=app;password={ab;" + leakMarker + "};x=1",
	"jdbc:mysql://app:" + leakMarker + "@localhost:3306/db",
	"jdbc:mysql://app:" + leakMarker + "@localhost:3306/db?next=http://x/y",
	"Server=localhost,1433;Password=ab " + leakMarker + ";Database=d",
	`Server=localhost,1433;Password="ab;` + leakMarker + `";Database=d`,
	"Driver={ODBC};Server=localhost,1433;Uid=app;Pwd=" + leakMarker + ";Database=d",
	"app:" + leakMarker + "@tcp(localhost:3306)/db",
	"app:" + leakMarker + "@localhost:6379",
	"app:" + leakMarker + "@tcp(localhost:3306)/db?redirect=http://x",
	"mongodb://app:pw@x?" + leakMarker + "@localhost:27017/db",
	"mongodb://app@x#:" + leakMarker + "@localhost:27017/db",
	"postgres://app:pw@localhost:5432/db?next=http://u:" + leakMarker + "@x",
	"http://user:" + leakMarker + "#x@api.staging.example.com/v1",
	" postgres://app:" + leakMarker + "@localhost:5432/db ",
	"http://localhost:3000/a@" + leakMarker + "?next=c@d",
	"postgres://app:5432@localhost:5432/" + leakMarker,
}

// auditShapes are the LUC-279 audit's repros and the shapes around them: a
// password ending like a local host, a number after a ":" in a password, a
// host or a port written by the user next to the credentials.
var auditShapes = []string{
	"postgres://app:" + leakMarker + ".localhost:5432/zz@db/app",
	"host=localhost password=" + leakMarker + ".localhost:5432",
	"Server=db;Password=" + leakMarker + ".localhost:5432;",
	"redis://:" + leakMarker + ".localhost:5432",
	"Server=db;Password=ab:" + leakMarker + ";",
	"host=db password=ab:" + leakMarker + " dbname=x",
	"host=db password=x.localhost:" + leakMarker,
	"host=localhost password=localhost:5432 user=" + leakMarker,
	"postgres://" + leakMarker + "@localhost:5432/db",
	"postgres://app:" + leakMarker + "@[::1]:5432/db",
	"http://" + leakMarker + ":pw@localhost:3000/cb",
	"http://localhost:3000/cb?token=" + leakMarker + "#" + leakMarker,
	"http://user:" + leakMarker + "#x@api.staging.example.com/v1",
	"http://" + leakMarker + ".example.com:3000/",
	"http://localhost:3000," + leakMarker + ",http://localhost:3001",
	"5432",
	"http://localhost:3000",
}

const leakProbe = "Zq7Wx3Kp9"

// LUC-279: no surface of `wtm env`, nor the env_ports of create, extract and
// checkout, prints any text read from a value without --show-values: a secret
// put anywhere in any value shape never comes out. It sweeps every position of
// every shape; FuzzEnvReportNeverLeaks searches beyond them.
func TestEnvReportNeverLeaksAtAnyPosition(t *testing.T) {
	for _, shape := range append(leakShapes, auditShapes...) {
		for at := 0; at <= len(shape); at++ {
			assertNoLeak(t, leakCase{Shape: shape, Secret: leakProbe, At: at})
		}
	}
}

func FuzzEnvReportNeverLeaks(f *testing.F) {
	for _, shape := range append(leakShapes, auditShapes...) {
		f.Add(shape, leakProbe, uint16(0))
		f.Add(shape, "s3cr3tValue", uint16(len(shape)/2))
	}
	f.Fuzz(func(t *testing.T, shape, secret string, at uint16) {
		if !isLeakProbe(secret) || strings.ContainsAny(shape, "\n\r") {
			t.Skip()
		}
		assertNoLeak(t, leakCase{Shape: shape, Secret: secret, At: int(at) % (len(shape) + 1)})
	})
}

// isLeakProbe is a secret no output can hold by chance: long, and mixing
// letters and digits, so it cannot be a port, a key or a word of a message.
func isLeakProbe(secret string) bool {
	if len(secret) < 8 || len(secret) > 40 {
		return false
	}
	letter, digit := false, false
	for _, r := range secret {
		switch {
		case r < unicode.MaxASCII && unicode.IsLetter(r):
			letter = true
		case unicode.IsDigit(r):
			digit = true
		default:
			return false
		}
	}
	return letter && digit
}

type leakCase struct {
	Shape  string
	Secret string
	At     int
}

func assertNoLeak(t *testing.T, c leakCase) {
	t.Helper()
	other := otherSecret(c.Secret)
	baseline := strings.Join(renderEnvSurfaces(t, leakValue(c, ""), leakValue(c, "")), "\n")
	if strings.Contains(baseline, c.Secret) || strings.Contains(baseline, other) {
		t.Skip("the probe is part of the output without it")
	}
	for _, out := range renderEnvSurfaces(t, leakValue(c, c.Secret), leakValue(c, other)) {
		if strings.Contains(out, c.Secret) || strings.Contains(out, other) {
			t.Fatalf("shape %q, secret %q at %d leaked:\n%s", c.Shape, c.Secret, c.At, out)
		}
	}
}

func leakValue(c leakCase, secret string) string {
	at := min(c.At, len(c.Shape))
	value := c.Shape[:at] + secret + c.Shape[at:]
	return strings.ReplaceAll(value, leakMarker, secret)
}

// otherSecret is the source's credential, so the worktree's and the source's
// values differ the way a hand edit makes them.
func otherSecret(secret string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '8', r >= 'a' && r <= 'y', r >= 'A' && r <= 'Y':
			return r + 1
		case r == '9':
			return '0'
		case r == 'z':
			return 'a'
		default:
			return 'A'
		}
	}, secret)
}

// renderEnvSurfaces runs a worktree whose .env holds value through the real
// plan and restore, the source holding source, and returns every surface a
// report has without --show-values.
func renderEnvSurfaces(t *testing.T, value, source string) []string {
	t.Helper()
	web := domain.JobConfig{Name: "web", Kind: domain.JobKindService, Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}}
	links := []domain.EnvPortLink{
		{File: ".env", Key: "K", Job: "db", Port: "P"},
		{File: ".env", Key: "K", Job: "web", Port: "PORT"},
		{File: ".env", Key: "L", Job: "web", Port: "PORT"},
	}
	bases := map[domain.PortRef]int{{Job: "db", Name: "P"}: 5432, {Job: "web", Name: "PORT"}: 3000}
	body := "K=" + value + "\nL=" + value + "\n"
	plan := func(origins rules.OriginContext) domain.EnvPortPlan {
		return rules.PlanEnvPorts(rules.PlanEnvPortsParams{
			Links: links, Bases: bases, Offset: 10, Origins: origins,
			Lines: map[string][]domain.EnvLine{".env": rules.ParseEnv(body)},
		})
	}
	ports := plan(rules.OriginContext{})
	names := plan(rules.OriginContext{
		Addressing: domain.AddressingNames, Jobs: map[string]domain.JobConfig{"web": web},
		Worktree: "feat-x", Project: "app", PublicPort: 1355,
	})
	ports.Owned = []domain.EnvOwnedEntry{{File: ".env", Key: "REALM", Value: "app-feat-x", Changed: true}}

	_, restored := rules.RestoreOwnedEnv(rules.RestoreOwnedEnvParams{
		File:      ".env",
		Child:     rules.ApplyEnvPorts(rules.ParseEnv(body), ports.Entries),
		Source:    rules.ParseEnv("K=" + source + "\nL=" + source + "\n"),
		Keys:      []string{"K", "L", "GONE"},
		PortBases: map[string][]int{"K": {5432, 3000}, "L": {3000}},
	})
	_, removed := rules.RestoreOwnedEnv(rules.RestoreOwnedEnvParams{
		File: ".env", Child: rules.ParseEnv("K=" + value + "\n"), Keys: []string{"K"},
	})
	restored = append(restored, removed...)

	file := domain.EnvFileResult{Target: ".env", Strategy: domain.EnvStrategyMain, Source: "main", Diff: domain.EnvDiff{Entries: []domain.EnvKeyDiff{
		{Key: "K", Status: domain.EnvKeyConflict, CurrentValue: value, ResolvedValue: source, Source: domain.EnvSourceMain},
		{Key: "NEW", Status: domain.EnvKeyResolved, ResolvedValue: value, Source: domain.EnvSourceMain},
		{Key: "OLD", Status: domain.EnvKeyOrphan, CurrentValue: value},
		{Key: "KEPT", Status: domain.EnvKeyConflict, CurrentValue: value, ResolvedValue: source, Action: domain.EnvActionKept},
	}}}

	var outs []string
	for _, plan := range []domain.EnvPortPlan{ports, names} {
		for _, check := range []bool{true, false} {
			result := domain.EnvSyncResult{Branch: "feat/x", Mode: domain.EnvModeRefresh, Check: check, Files: []domain.EnvFileResult{file}, Ports: plan, Restored: restored}
			var text, json bytes.Buffer
			PrintEnvReport(&text, EnvReportParams{Result: result})
			if err := WriteEnvJSON(&json, EnvReportParams{Result: result}); err != nil {
				t.Fatal(err)
			}
			outs = append(outs, text.String(), json.String())
		}
		var create, extract, checkout bytes.Buffer
		if err := WriteWorktreeCreateJSON(&create, domain.CreateBatchResult{Results: []domain.CreateResult{{EnvPorts: plan}}}); err != nil {
			t.Fatal(err)
		}
		if err := WriteExtractJSON(&extract, domain.ExtractResult{EnvPorts: plan}); err != nil {
			t.Fatal(err)
		}
		if err := WritePRCheckoutJSON(&checkout, PRCheckoutJSON{EnvPorts: plan}); err != nil {
			t.Fatal(err)
		}
		outs = append(outs, create.String(), extract.String(), checkout.String())
		outs = append(outs, rules.EnvPortRecapLines(plan)...)
		outs = append(outs, rules.EnvPortTableLines(rules.EnvPortTableParams{Plan: plan, Width: 200})...)
	}
	outs = append(outs, rules.EnvRestoreRecapLines(restored)...)
	return outs
}

package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// The port pass never lists the values it moved, applied or not: the .env holds
// them, and a result report that repeats them buries the two lines a reader acts
// on. A declined pass still says it was declined.
func TestPrintEnvReportNeverListsTheValuesItMoved(t *testing.T) {
	plan := rules.PlanEnvPorts(rules.PlanEnvPortsParams{
		Links:  []domain.EnvPortLink{{File: ".env", Key: "DATABASE_URL", Job: "svc", Port: "POSTGRES_PORT"}},
		Bases:  map[domain.PortRef]int{{Job: "svc", Name: "POSTGRES_PORT"}: 5432},
		Offset: 10,
		Lines:  map[string][]domain.EnvLine{".env": rules.ParseEnv("DATABASE_URL=postgres://localhost:5432/app\n")},
	})
	result := domain.EnvSyncResult{
		Branch: "feat/x",
		Mode:   domain.EnvModeAdd,
		Files:  []domain.EnvFileResult{{Target: ".env", Applied: true}},
		Ports:  plan,
	}

	var declined bytes.Buffer
	PrintEnvReport(&declined, result)
	if strings.Contains(declined.String(), "BECOMES") {
		t.Errorf("a declined pass printed its table:\n%s", declined.String())
	}
	if !strings.Contains(declined.String(), "left alone") {
		t.Errorf("a declined pass never says so:\n%s", declined.String())
	}

	result.Ports.Applied = true
	var applied bytes.Buffer
	PrintEnvReport(&applied, result)
	if strings.Contains(applied.String(), "BECOMES") {
		t.Errorf("an applied pass printed its table:\n%s", applied.String())
	}
	// The trailing summary is what counts an applied pass; saying it twice is
	// what the table used to do.
	if strings.Contains(applied.String(), "left alone") {
		t.Errorf("an applied pass reported itself as declined:\n%s", applied.String())
	}
	if !strings.Contains(applied.String(), "settled 1 linked value(s)") {
		t.Errorf("an applied pass never says what it settled:\n%s", applied.String())
	}
}

// A --check run has nothing else to say: no file was written, so the count and
// the offset are the whole of the preview.
func TestPrintEnvReportPreviewsThePortPassAsACount(t *testing.T) {
	plan := rules.PlanEnvPorts(rules.PlanEnvPortsParams{
		Links:  []domain.EnvPortLink{{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"}},
		Bases:  map[domain.PortRef]int{{Job: "web", Name: "PORT"}: 3000},
		Offset: 10,
		Lines:  map[string][]domain.EnvLine{".env": rules.ParseEnv("WEB_PORT=3000\n")},
	})

	var buf bytes.Buffer
	PrintEnvReport(&buf, domain.EnvSyncResult{
		Branch: "feat/x",
		Mode:   domain.EnvModeAdd,
		Check:  true,
		Files:  []domain.EnvFileResult{{Target: ".env"}},
		Ports:  plan,
	})

	if strings.Contains(buf.String(), "BECOMES") {
		t.Errorf("a preview printed its table:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "would be shifted (offset +10)") {
		t.Errorf("a preview never says how much would move:\n%s", buf.String())
	}
}

// The report has to name what it ran on: a reader who omitted the argument and
// picked interactively otherwise gets no confirmation of what was reconciled.
func TestPrintEnvReportNamesTheWorktreeAndMode(t *testing.T) {
	var buf bytes.Buffer
	PrintEnvReport(&buf, domain.EnvSyncResult{
		Branch: "feat/x",
		Mode:   domain.EnvModeRefresh,
		Check:  true,
		Files:  []domain.EnvFileResult{{Target: ".env"}},
	})

	for _, want := range []string{"feat/x", "refresh", "read-only check"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("report never mentions %q:\n%s", want, buf.String())
		}
	}
}

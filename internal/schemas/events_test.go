package schemas_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/schemas"
	"github.com/LucasPcq/wtm/internal/testutil/schematest"
)

const repo = `"repo":{"root":"/r","common_dir":"/r/.git"}`
const identity = `"worktree":{"branch":"a","path":"/p","parent":"main","ordinal":1,"isolation":"isolated","is_main":false,"created_at":""}`

func TestTheEventsSchemaAcceptsAReady(t *testing.T) {
	s := schematest.Compile(t, schemas.Events)
	if err := schematest.Validate(t, s, []byte(`{"v":1,"type":"ready","ts":"2026-10-03T10:00:00Z"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestTheEventsSchemaRefusesAnUpdateWithoutChanged(t *testing.T) {
	s := schematest.Compile(t, schemas.Events)
	doc := `{"v":1,"type":"worktree.updated","ts":"2026-10-03T10:00:00Z",` + repo + `,` + identity + `}`
	if err := schematest.Validate(t, s, []byte(doc)); err == nil {
		t.Fatal("an update naming no changed field must not validate")
	}
}

func TestTheEventsSchemaRefusesANewerVersion(t *testing.T) {
	s := schematest.Compile(t, schemas.Events)
	if err := schematest.Validate(t, s, []byte(`{"v":2,"type":"ready","ts":"2026-10-03T10:00:00Z"}`)); err == nil {
		t.Fatal("v1's schema must not accept a v2 event")
	}
}

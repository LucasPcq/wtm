package domain

// EnvMode selects how DiffEnv classifies keys already present in the child .env.
// Add only fills gaps and never reports a value conflict; refresh additionally
// flags keys whose child value differs from the resolved source value.
type EnvMode string

const (
	// EnvModeAdd adds expected-but-missing keys and never touches an existing value.
	EnvModeAdd EnvMode = "add"
	// EnvModeRefresh also reconciles existing values, surfacing differences as conflicts.
	EnvModeRefresh EnvMode = "refresh"
)

// EnvKeyStatus is the reconciliation verdict for one key in an EnvDiff.
type EnvKeyStatus string

const (
	// EnvKeyResolved means the key needs no decision: either the child already holds
	// the intended value, or it is missing and a source value can be added silently.
	EnvKeyResolved EnvKeyStatus = "resolved"
	// EnvKeyMissing means the key is expected but absent from the child and no real
	// source value (parent/main) resolves it; only a template placeholder is known,
	// so a value must be prompted before it can be added.
	EnvKeyMissing EnvKeyStatus = "missing_unresolved"
	// EnvKeyConflict means (refresh only) the child holds a value that differs from the
	// resolved source value; the difference must be settled keep/overwrite/edit.
	EnvKeyConflict EnvKeyStatus = "conflict"
	// EnvKeyOrphan means the key exists only in the child and in none of the expected
	// sources; it is a candidate for pruning.
	EnvKeyOrphan EnvKeyStatus = "orphan"
)

// EnvConflictDecision is how ApplyEnvDiff settles an EnvKeyConflict. An explicit
// edited value is passed out of band through ApplyEnvDiffParams.FilledValues and
// takes precedence over the decision.
type EnvConflictDecision string

const (
	// EnvDecisionKeep retains the child's current value. This is the safe default when
	// no decision is supplied for a conflict.
	EnvDecisionKeep EnvConflictDecision = "keep"
	// EnvDecisionOverwrite replaces the child value with the resolved source value.
	EnvDecisionOverwrite EnvConflictDecision = "overwrite"
)

// EnvKeyAction is what an apply did to a key; a read-only run leaves it empty,
// and so does an apply for a key it left as it found it.
type EnvKeyAction string

const (
	EnvActionAdded       EnvKeyAction = "added"
	EnvActionFilled      EnvKeyAction = "filled"
	EnvActionOverwritten EnvKeyAction = "overwritten"
	EnvActionKept        EnvKeyAction = "kept"
	EnvActionPruned      EnvKeyAction = "pruned"
	EnvActionSkipped     EnvKeyAction = "skipped"
)

// EnvSourceParent, EnvSourceMain label where a resolved value came from in the
// parent -> main cascade. An empty Source means no real source value was found.
const (
	EnvSourceParent = "parent"
	EnvSourceMain   = "main"
)

// EnvKeyDiff is the reconciliation verdict for a single key. CurrentValue is the
// child's value ("" when the key is absent from the child). ResolvedValue is the
// real source value (parent/main) when one exists. Placeholder is the template
// value, meaningful for EnvKeyMissing. Source names the cascade level that produced
// ResolvedValue. Export mirrors the source/template export flag for added keys.
type EnvKeyDiff struct {
	Key           string       `json:"key"`
	Status        EnvKeyStatus `json:"status"`
	CurrentValue  string       `json:"current_value,omitempty"`
	ResolvedValue string       `json:"resolved_value,omitempty"`
	Placeholder   string       `json:"placeholder,omitempty"`
	Source        string       `json:"source,omitempty"`
	Export        bool         `json:"export,omitempty"`
	Action        EnvKeyAction `json:"action,omitempty"`
	// Redacted says a value was withheld: the key is not one wtm writes, and
	// the report was not asked to show values.
	Redacted bool `json:"redacted,omitempty"`
	// SourceLine is the line an added key is copied from, so it lands quoted
	// and commented the way its source wrote it.
	SourceLine EnvLine `json:"-"`
}

// EnvDiff is the full per-key reconciliation of a child .env against its template
// and value sources. Entries are ordered stably: existing child keys in file order
// first, then keys added in template -> parent -> main order.
type EnvDiff struct {
	Mode    EnvMode      `json:"mode"`
	Entries []EnvKeyDiff `json:"entries"`
}

// EnvKeyRow is one key of a file block, ready to render: Status picks the glyph
// and the colour, Text is the already-aligned "KEY  detail".
type EnvKeyRow struct {
	Status EnvKeyStatus
	Text   string
}

package rules

import (
	"github.com/LucasPcq/wtm/internal/domain"
)

// EnvDiffParams holds the four parsed .env documents to reconcile plus the mode.
// Template is the committed schema, Parent and Main are value sources (parent
// worktree, then main), Child is the .env being reconciled.
type EnvDiffParams struct {
	Template []domain.EnvLine
	Parent   []domain.EnvLine
	Main     []domain.EnvLine
	Child    []domain.EnvLine
	Mode     domain.EnvMode
	// PortBases is the declared base of every key an [[env_port]] link follows,
	// and PortBlock the spacing between two worktrees. They exist so a value that
	// differs from its source only by the worktree's port offset is not reported
	// as a conflict between two spellings of the same setting.
	PortValues map[string]EnvValueRef
	PortBlock  int
	// Owned are the keys an [[env]] link writes in full in this file. Like the
	// identity keys, their value differs per worktree by construction, so they
	// are neither drift nor a conflict.
	Owned map[string]bool
}

// DiffEnv classifies every key of the child .env against its schema and value
// sources, without any I/O. Expected keys are keys(template) ∪ keys(parent) ∪
// keys(main). A real value is resolved through the parent -> main cascade (the
// template only supplies a placeholder, never a silent value; empty source values
// are skipped). Entries are ordered stably: child keys in file order, then the
// remaining expected keys in template -> parent -> main order.
func DiffEnv(params EnvDiffParams) domain.EnvDiff {
	child := pairsByKey(params.Child)
	template := pairsByKey(params.Template)
	parent := pairsByKey(params.Parent)
	main := pairsByKey(params.Main)

	seen := make(map[string]bool)
	entries := make([]domain.EnvKeyDiff, 0, len(child)+len(template)+len(parent)+len(main))
	appendKeys := func(lines []domain.EnvLine) {
		for _, l := range lines {
			if l.Kind != domain.EnvLinePair || seen[l.Key] {
				continue
			}
			seen[l.Key] = true
			entries = append(entries, classifyKey(classifyKeyParams{
				Key:        l.Key,
				Mode:       params.Mode,
				Child:      child,
				Template:   template,
				Parent:     parent,
				Main:       main,
				PortValues: params.PortValues,
				PortBlock:  params.PortBlock,
				Owned:      params.Owned,
			}))
		}
	}
	appendKeys(params.Child)
	appendKeys(params.Template)
	appendKeys(params.Parent)
	appendKeys(params.Main)

	return domain.EnvDiff{Mode: params.Mode, Entries: entries}
}

// classifyKeyParams holds one key and the indexed sources needed to classify it.
type classifyKeyParams struct {
	Key        string
	Mode       domain.EnvMode
	Child      map[string]domain.EnvLine
	Template   map[string]domain.EnvLine
	Parent     map[string]domain.EnvLine
	Main       map[string]domain.EnvLine
	PortValues map[string]EnvValueRef
	PortBlock  int
	Owned      map[string]bool
}

// differ compares a source value with the child's, ignoring the port offset that
// separates two worktrees' copies of the same setting. A key no link follows is
// compared verbatim.
func (p classifyKeyParams) differ(source, child string) bool {
	ref, linked := p.PortValues[p.Key]
	if !linked {
		return source != child
	}
	reduce := func(value string) string {
		return ReduceEnvPortValue(ReduceEnvPortParams{
			Value:    value,
			Base:     ref.Base,
			Block:    p.PortBlock,
			JobLabel: ref.JobLabel,
			Project:  ref.Project,
		})
	}
	return reduce(source) != reduce(child)
}

// classifyKey applies the reconciliation table for a single key.
func classifyKey(params classifyKeyParams) domain.EnvKeyDiff {
	k := params.Key
	childLine, inChild := params.Child[k]
	_, inTemplate := params.Template[k]
	_, inParent := params.Parent[k]
	_, inMain := params.Main[k]
	expected := inTemplate || inParent || inMain

	srcVal, srcLabel, srcExport, hasSrc := resolveSource(resolveSourceParams{
		Key:    k,
		Parent: params.Parent,
		Main:   params.Main,
	})

	diff := domain.EnvKeyDiff{Key: k}

	if inChild {
		diff.CurrentValue = childLine.Value
		// A key wtm derives from the worktree differs from every source by
		// construction. Absent from the child it is missing like any other, and
		// the owned pass writes it.
		if IsOwnedEnvKey(k) || params.Owned[k] {
			diff.Status = domain.EnvKeyResolved
			return diff
		}
		if !expected {
			diff.Status = domain.EnvKeyOrphan
			return diff
		}
		if params.Mode == domain.EnvModeRefresh && hasSrc && params.differ(srcVal, childLine.Value) {
			diff.Status = domain.EnvKeyConflict
			diff.ResolvedValue = srcVal
			diff.Source = srcLabel
			return diff
		}
		diff.Status = domain.EnvKeyResolved
		return diff
	}

	if hasSrc {
		diff.Status = domain.EnvKeyResolved
		diff.ResolvedValue = srcVal
		diff.Source = srcLabel
		diff.Export = srcExport
		return diff
	}

	diff.Status = domain.EnvKeyMissing
	if tmpl, ok := params.Template[k]; ok {
		diff.Placeholder = tmpl.Value
		diff.Export = tmpl.Export
	}
	return diff
}

// resolveSourceParams holds the key and the two value sources of the cascade.
type resolveSourceParams struct {
	Key    string
	Parent map[string]domain.EnvLine
	Main   map[string]domain.EnvLine
}

// resolveSource returns the first non-empty value in the parent -> main cascade,
// its source label and export flag. ok is false when neither source resolves.
func resolveSource(params resolveSourceParams) (value, source string, export, ok bool) {
	if l, present := params.Parent[params.Key]; present && l.Value != "" {
		return l.Value, domain.EnvSourceParent, l.Export, true
	}
	if l, present := params.Main[params.Key]; present && l.Value != "" {
		return l.Value, domain.EnvSourceMain, l.Export, true
	}
	return "", "", false, false
}

// ApplyEnvDiffParams holds the inputs to materialize a resolved diff. Child is the
// original document to edit in place (round-trip preserved via EnvLine.Raw).
// Decisions settle conflicts (keep/overwrite); FilledValues supplies prompted or
// edited values (missing keys, conflict edits, and edited additions); Prune drops
// every orphan key, while PruneKeys drops only the named orphans; SkipKeys omits a
// resolved/missing addition the user chose not to add (per-key from the wizard).
type ApplyEnvDiffParams struct {
	Child        []domain.EnvLine
	Diff         domain.EnvDiff
	Decisions    map[string]domain.EnvConflictDecision
	FilledValues map[string]string
	Prune        bool
	PruneKeys    map[string]bool
	SkipKeys     map[string]bool
}

// ApplyEnvDiff produces the reconciled .env lines from a diff and its decisions,
// without any I/O. It preserves every unrelated child line verbatim, mutates only
// keys with a decided conflict, drops orphan keys when Prune is set, and appends
// added keys (resolved values, plus missing keys with a supplied value) at the end
// in diff order. It is safe by default: an undecided conflict keeps the child value,
// and a missing key with no supplied value is not added.
func ApplyEnvDiff(params ApplyEnvDiffParams) []domain.EnvLine {
	byKey := make(map[string]domain.EnvKeyDiff, len(params.Diff.Entries))
	for _, e := range params.Diff.Entries {
		byKey[e.Key] = e
	}
	childKeys := make(map[string]bool)
	for _, l := range params.Child {
		if l.Kind == domain.EnvLinePair {
			childKeys[l.Key] = true
		}
	}

	out := make([]domain.EnvLine, 0, len(params.Child)+len(params.Diff.Entries))
	for _, l := range params.Child {
		if l.Kind != domain.EnvLinePair {
			out = append(out, l)
			continue
		}
		entry, ok := byKey[l.Key]
		if !ok {
			out = append(out, l)
			continue
		}
		switch entry.Status {
		case domain.EnvKeyOrphan:
			if params.Prune || params.PruneKeys[l.Key] {
				continue
			}
			out = append(out, l)
		case domain.EnvKeyConflict:
			out = append(out, resolveConflict(l, entry, params))
		default:
			out = append(out, l)
		}
	}

	for _, e := range params.Diff.Entries {
		if childKeys[e.Key] {
			continue
		}
		if params.SkipKeys[e.Key] {
			continue
		}
		if line, ok := addedLine(e, params.FilledValues); ok {
			out = append(out, line)
		}
	}

	return out
}

// resolveConflict returns the child line settled per FilledValues (edit) or the
// conflict decision (overwrite), else the unchanged line (keep, the safe default).
func resolveConflict(line domain.EnvLine, entry domain.EnvKeyDiff, params ApplyEnvDiffParams) domain.EnvLine {
	if v, ok := params.FilledValues[entry.Key]; ok {
		return mutatedPair(line, v)
	}
	if params.Decisions[entry.Key] == domain.EnvDecisionOverwrite {
		return mutatedPair(line, entry.ResolvedValue)
	}
	return line
}

// addedLine builds the pair to append for a key absent from the child: a resolved
// source value, or a missing key whose value was supplied. ok is false otherwise.
func addedLine(entry domain.EnvKeyDiff, filled map[string]string) (domain.EnvLine, bool) {
	switch entry.Status {
	case domain.EnvKeyResolved:
		if v, ok := filled[entry.Key]; ok {
			return newPair(entry.Key, v, entry.Export), true
		}
		return newPair(entry.Key, entry.ResolvedValue, entry.Export), true
	case domain.EnvKeyMissing:
		if v, ok := filled[entry.Key]; ok {
			return newPair(entry.Key, v, entry.Export), true
		}
		return domain.EnvLine{}, false
	default:
		return domain.EnvLine{}, false
	}
}

// mutatedPair returns line with a new value and Raw cleared so RenderEnv re-emits it.
func mutatedPair(line domain.EnvLine, value string) domain.EnvLine {
	line.Value = value
	line.Raw = ""
	return line
}

// newPair builds a fresh pair with no Raw, to be rendered canonically.
func newPair(key, value string, export bool) domain.EnvLine {
	return domain.EnvLine{Kind: domain.EnvLinePair, Key: key, Value: value, Export: export}
}

// pairsByKey indexes the pair lines by key, last occurrence winning.
func pairsByKey(lines []domain.EnvLine) map[string]domain.EnvLine {
	out := make(map[string]domain.EnvLine, len(lines))
	for _, l := range lines {
		if l.Kind != domain.EnvLinePair {
			continue
		}
		out[l.Key] = l
	}
	return out
}

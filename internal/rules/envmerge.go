package rules

import (
	"strings"

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
	// Owned are the keys the owned pass writes in full in this file: [[env]]
	// links and the identity keys. Their value differs per worktree by
	// construction, so they are neither drift nor a conflict — and absent from
	// the child, they are the pass's to add, not a gap asking for a value.
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
			if _, inChild := child[l.Key]; !inChild && params.Owned[l.Key] {
				continue
			}
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

	srcLine, srcLabel, hasSrc := resolveSource(resolveSourceParams{
		Key:    k,
		Parent: params.Parent,
		Main:   params.Main,
	})
	srcVal := srcLine.Value

	diff := domain.EnvKeyDiff{Key: k}

	if inChild {
		diff.CurrentValue = childLine.Value
		// A key wtm derives from the worktree differs from every source by
		// construction.
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
		diff.Export = srcLine.Export
		diff.SourceLine = srcLine
		return diff
	}

	diff.Status = domain.EnvKeyMissing
	if tmpl, ok := params.Template[k]; ok {
		diff.Placeholder = tmpl.Value
		diff.Export = tmpl.Export
		diff.SourceLine = tmpl
	}
	return diff
}

// resolveSourceParams holds the key and the two value sources of the cascade.
type resolveSourceParams struct {
	Key    string
	Parent map[string]domain.EnvLine
	Main   map[string]domain.EnvLine
}

// resolveSource returns the first line with a non-empty value in the parent ->
// main cascade and its source label. ok is false when neither source resolves.
func resolveSource(params resolveSourceParams) (line domain.EnvLine, source string, ok bool) {
	if l, present := params.Parent[params.Key]; present && l.Value != "" {
		return l, domain.EnvSourceParent, true
	}
	if l, present := params.Main[params.Key]; present && l.Value != "" {
		return l, domain.EnvSourceMain, true
	}
	return domain.EnvLine{}, "", false
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

	var added []domain.EnvLine
	for _, e := range params.Diff.Entries {
		if childKeys[e.Key] || params.SkipKeys[e.Key] {
			continue
		}
		if line, ok := addedLine(e, params.FilledValues); ok {
			added = append(added, line)
		}
	}
	return insertBeforeTrailingBlanks(out, added)
}

// EnvDiffActions records on each entry what ApplyEnvDiff does to it under the
// same params, so a report can count what was done instead of restating the
// diff it started from.
func EnvDiffActions(params ApplyEnvDiffParams) domain.EnvDiff {
	out := domain.EnvDiff{Mode: params.Diff.Mode, Entries: make([]domain.EnvKeyDiff, 0, len(params.Diff.Entries))}
	for _, entry := range params.Diff.Entries {
		entry.Action = envKeyAction(entry, params)
		out.Entries = append(out.Entries, entry)
	}
	return out
}

func envKeyAction(entry domain.EnvKeyDiff, params ApplyEnvDiffParams) domain.EnvKeyAction {
	_, filled := params.FilledValues[entry.Key]
	switch entry.Status {
	case domain.EnvKeyOrphan:
		if params.Prune || params.PruneKeys[entry.Key] {
			return domain.EnvActionPruned
		}
		return domain.EnvActionKept
	case domain.EnvKeyConflict:
		switch {
		case filled:
			return domain.EnvActionFilled
		case params.Decisions[entry.Key] == domain.EnvDecisionOverwrite:
			return domain.EnvActionOverwritten
		}
		return domain.EnvActionKept
	case domain.EnvKeyMissing:
		switch {
		case params.SkipKeys[entry.Key]:
			return domain.EnvActionSkipped
		case filled:
			return domain.EnvActionFilled
		}
	case domain.EnvKeyResolved:
		if !envAddition(entry) {
			return ""
		}
		switch {
		case params.SkipKeys[entry.Key]:
			return domain.EnvActionSkipped
		case filled:
			return domain.EnvActionFilled
		}
		return domain.EnvActionAdded
	}
	return ""
}

// insertBeforeTrailingBlanks lands new lines after the last one holding
// something: a parsed document keeps its final newline as a trailing blank line,
// and appending past it would drop that newline and open a gap instead.
func insertBeforeTrailingBlanks(lines, added []domain.EnvLine) []domain.EnvLine {
	if len(added) == 0 {
		return lines
	}
	at := len(lines)
	for at > 0 && lines[at-1].Kind == domain.EnvLineBlank && lines[at-1].Raw == "" {
		at--
	}
	out := make([]domain.EnvLine, 0, len(lines)+len(added))
	out = append(out, lines[:at]...)
	out = append(out, added...)
	return append(out, lines[at:]...)
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
			return newPair(entry, v), true
		}
		return newPair(entry, entry.ResolvedValue), true
	case domain.EnvKeyMissing:
		if v, ok := filled[entry.Key]; ok {
			return newPair(entry, v), true
		}
		return domain.EnvLine{}, false
	default:
		return domain.EnvLine{}, false
	}
}

func mutatedPair(line domain.EnvLine, value string) domain.EnvLine {
	return WithEnvValue(line, value)
}

// newPair copies the line the key comes from, its line ending left to the file
// it lands in, and renders a fresh one when there is none.
func newPair(entry domain.EnvKeyDiff, value string) domain.EnvLine {
	source := entry.SourceLine
	if source.Kind != domain.EnvLinePair || source.Raw == "" {
		return domain.EnvLine{Kind: domain.EnvLinePair, Key: entry.Key, Value: value, Export: entry.Export}
	}
	source.Raw = strings.TrimSuffix(source.Raw, domain.EnvCR)
	return WithEnvValue(source, value)
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

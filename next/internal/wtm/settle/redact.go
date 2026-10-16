package settle

import "slices"

type Redacted Plan

func Redact(plan Plan) Redacted {
	entries := slices.Clone(plan.Entries)
	for i := range entries {
		entries[i].CurrentValue = ""
		entries[i].NewValue = ""
		entries[i].ForeignHost = ""
	}
	plan.Entries = entries
	return Redacted(plan)
}

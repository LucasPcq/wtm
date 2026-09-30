package domain

// Tone is how loud a label reads, without naming a colour: a flow declares one,
// the surface rendering it decides what that looks like.
type Tone int

const (
	ToneNeutral Tone = iota
	ToneSuccess
	ToneWarning
	ToneDanger
)

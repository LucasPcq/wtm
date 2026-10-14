package kernel

type Code string

type Params map[string]string

const (
	CodeRequired      Code = "required"
	CodeRequiredOneOf Code = "required_one_of"
	CodeInvalid       Code = "invalid"
	CodeNotFound      Code = "not_found"
	CodeOneOf         Code = "one_of"
	CodeExclusiveWith Code = "exclusive_with"
	CodeRequires      Code = "requires"
	CodeDistinct      Code = "distinct"
	CodeSelfParent    Code = "self_parent"
	CodeTooShort      Code = "too_short"
	CodeTooLong       Code = "too_long"
	CodeTooFew        Code = "too_few"
	CodeTooMany       Code = "too_many"
	CodePattern       Code = "pattern"
	CodeUnknownField  Code = "unknown_field"

	CodeInvalidRequest Code = "request.invalid"
	CodeInternal       Code = "internal"
	CodeInterrupted    Code = "unit.interrupted"
	CodeStepFailed     Code = "unit.step_failed"
	CodeUndoFailed     Code = "unit.undo_failed"
	CodePhaseFailed    Code = "unit.phase_failed"

	CodeLockHeld         Code = "lock.held"
	CodeLocksUnavailable Code = "locks.unavailable"
	CodePlanStale        Code = "plan.stale"
	CodeStateSchemaNewer Code = "state.schema_newer"
)

const (
	ParamPath    = "path"
	ParamValue   = "value"
	ParamLimit   = "limit"
	ParamPattern = "pattern"
	ParamBecause = "because"
	ParamStep    = "step"
	ParamPhase   = "phase"
	ParamLeft    = "left"

	ParamKey       = "key"
	ParamOp        = "op"
	ParamCommand   = "command"
	ParamPID       = "pid"
	ParamSurface   = "surface"
	ParamStartedAt = "started_at"
	ParamExpected  = "expected"
	ParamFound     = "found"
	ParamSupported = "supported"
)

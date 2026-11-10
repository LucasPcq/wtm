package daemonrestart

import "github.com/LucasPcq/wtm/next/internal/domain"

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of daemon restart: whether to restart a daemon of a newer wtm, which takes this build's place (Force), the daemon being the machine's one.
type Request struct {
	Force bool `json:"force"`
}

// PathForce is the path of the field that restarts a newer wtm's daemon as this build.
const PathForce = "force"

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	if path != PathForce {
		return r, domain.Unknown(path)
	}
	force, err := domain.Yes(path, values)
	if err != nil {
		return r, err
	}
	r.Force = force
	return r, nil
}

// Values is the request's answer to the field at path, nil when it leaves the field to its default.
func (r Request) Values(path string) []string {
	if path == PathForce {
		return domain.SaidYes(r.Force)
	}
	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: force, which only its flag sets.
type Questions struct {
	Request Request
}

// Form is the schema of the request: force, flag only.
func (q Questions) Form() domain.Form {
	return domain.Form{Fields: []domain.Field{
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathForce, Skip: domain.SkipFlagOnly}, Value: q.Request.Force, NoFirst: true},
	}}
}

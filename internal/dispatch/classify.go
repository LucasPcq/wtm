package dispatch

import (
	"errors"
	"strconv"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
)

func classify(err error) kernel.Error {
	var newer *statefile.NewerError
	if errors.As(err, &newer) {
		return kernel.Precondition(kernel.Problem{Code: kernel.CodeStateSchemaNewer, Cause: err, Params: kernel.Params{
			kernel.ParamPath:      newer.Path,
			kernel.ParamFound:     strconv.Itoa(newer.Found),
			kernel.ParamSupported: strconv.Itoa(newer.Supported),
		}})
	}
	return kernel.Classify(kernel.ClassifyParams{Err: err, Code: kernel.CodeInternal})
}

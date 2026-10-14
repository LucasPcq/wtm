package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
)

const (
	causeSeparator = ": "
	fieldSeparator = "; "
	liftedByFmt    = "%s (%s)"
	detailLineFmt  = "%s%s%s\n"
)

type Wording struct {
	Catalog text.Catalog
	Names   Names
}

type FailureParams struct {
	Err     kernel.Error
	Wording Wording
	Next    []NextStepParams
}

func Failure(w io.Writer, params FailureParams) {
	Error(w, params.Wording.Error(params.Err))
	for _, line := range params.Wording.failureDetails(params.Err) {
		fmt.Fprintf(w, detailLineFmt, Indent, Indent, line)
	}
	NextSteps(w, params.Next)
}

func (wd Wording) FailureLine(err kernel.Error) string {
	details := wd.failureDetails(err)
	if len(details) == 0 {
		return wd.Error(err)
	}
	return wd.Error(err) + causeSeparator + strings.Join(details, fieldSeparator)
}

func (wd Wording) failureDetails(err kernel.Error) []string {
	switch typed := err.(type) {
	case *kernel.InvalidError:
		lines := make([]string, 0, len(typed.Fields))
		for _, field := range typed.Fields {
			lines = append(lines, wd.Field(field))
		}
		return lines
	case *kernel.RefusedError:
		lines := make([]string, 0, len(typed.Blockers))
		for _, blocker := range typed.Blockers {
			lines = append(lines, wd.blockerLine(blocker))
		}
		return lines
	case *kernel.Failure:
		return nil
	}
	return nil
}

func (wd Wording) Error(err kernel.Error) string {
	base := err.Base()
	message := wd.Catalog.Message(base.Code, base.Params)
	if base.Cause == nil || !keepsCause(err) {
		return message
	}
	return message + causeSeparator + base.Cause.Error()
}

func keepsCause(err kernel.Error) bool {
	switch typed := err.(type) {
	case *kernel.InvalidError:
		return len(typed.Fields) == 0
	case *kernel.RefusedError:
		return false
	case *kernel.Failure:
		return typed.Kind() == kernel.KindInternal
	}
	return false
}

func (wd Wording) Field(field kernel.FieldError) string {
	spelled := field
	spelled.Path = wd.Names(field.Path)
	spelled.With = make([]string, 0, len(field.With))
	for _, path := range field.With {
		spelled.With = append(spelled.With, wd.Names(path))
	}
	return wd.Catalog.Field(spelled)
}

func (wd Wording) Blocker(blocker kernel.Blocker) string {
	return wd.Catalog.Message(blocker.Code, blocker.Params)
}

func (wd Wording) blockerLine(blocker kernel.Blocker) string {
	if blocker.Field == "" {
		return wd.Blocker(blocker)
	}
	return fmt.Sprintf(liftedByFmt, wd.Blocker(blocker), wd.Names(blocker.Field))
}

func (wd Wording) Warning(warning kernel.Warning) string {
	return wd.Catalog.Message(warning.Code, warning.Params)
}

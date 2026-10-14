package text

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/kernel"
)

const (
	paramAccepted = "accepted"
	paramWith     = "with"
	duplicateFmt  = "text: code %q has two messages"
)

type Catalog struct {
	messages Messages
}

func NewCatalog(sets ...Messages) Catalog {
	merged := Messages{}
	for _, set := range append([]Messages{kernelMessages}, sets...) {
		for code, template := range set {
			if _, taken := merged[code]; taken {
				panic(fmt.Sprintf(duplicateFmt, code))
			}
			merged[code] = template
		}
	}
	return Catalog{messages: merged}
}

func (c Catalog) Message(code kernel.Code, params kernel.Params) string {
	template, known := c.messages[code]
	if !known {
		return string(code)
	}
	pairs := make([]string, 0, 2*len(params))
	for key, value := range params {
		pairs = append(pairs, "{"+key+"}", value)
	}
	return strings.NewReplacer(pairs...).Replace(template)
}

func (c Catalog) Field(problem kernel.FieldError) string {
	params := kernel.Params{
		kernel.ParamPath: problem.Path,
		paramAccepted:    strings.Join(problem.Accepted, ", "),
		paramWith:        strings.Join(problem.With, ", "),
	}
	for key, value := range problem.Params {
		params[key] = value
	}
	message := c.Message(problem.Code, params)
	because := problem.Params[kernel.ParamBecause]
	if because == "" {
		return message
	}
	return message + ": " + c.Message(kernel.Code(because), params)
}

func (c Catalog) Status(status kernel.Status) string {
	code, known := statusCodes[status]
	if !known {
		return string(status)
	}
	return c.Message(code, nil)
}

func (c Catalog) Reason(reason kernel.Reason) string {
	code, known := reasonCodes[reason]
	if !known {
		return string(reason)
	}
	return c.Message(code, nil)
}

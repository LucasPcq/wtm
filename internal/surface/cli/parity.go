package cli

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/dispatch"
)

const (
	annotationEntry  = "wtm.entry"
	annotationDetail = "wtm.entry.detail"

	unservedFmt    = "%s is in the registry, but no command serves it and it is not declared unbound"
	noDetailFmt    = "%s declares neither its own Detail nor GenericDetail"
	staleUnbindFmt = "%s is declared unbound, but %s"
	servedReason   = "a command serves it"
	missingReason  = "the registry does not hold it"
)

var unbound = map[string]string{
	"operations": "a query: queries get their commands at step 7 of LUC-283",
}

type parityParams struct {
	Registry dispatch.Registry
	Root     *cobra.Command
	Unbound  map[string]string
}

func parityGaps(params parityParams) []string {
	served := servedEntries(params.Root)
	var gaps []string
	for _, entry := range params.Registry {
		name := entry.Name()
		detail, isServed := served[name]
		_, excluded := params.Unbound[name]
		switch {
		case isServed && excluded:
			gaps = append(gaps, fmt.Sprintf(staleUnbindFmt, name, servedReason))
		case isServed && detail == "":
			gaps = append(gaps, fmt.Sprintf(noDetailFmt, name))
		case !isServed && !excluded:
			gaps = append(gaps, fmt.Sprintf(unservedFmt, name))
		}
	}
	for name := range params.Unbound {
		if _, held := params.Registry.Lookup(name); !held {
			gaps = append(gaps, fmt.Sprintf(staleUnbindFmt, name, missingReason))
		}
	}
	slices.Sort(gaps)
	return gaps
}

func servedEntries(root *cobra.Command) map[string]string {
	served := map[string]string{}
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if name, bound := cmd.Annotations[annotationEntry]; bound {
			served[name] = cmd.Annotations[annotationDetail]
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
	return served
}

package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

const (
	flagExpectFingerprint  = "expect-fingerprint"
	yesUsage               = "Run unattended: no question, no confirmation; what is not given takes its default"
	dryRunUsage            = "Show the plan and its fingerprint, apply nothing"
	expectFingerprintUsage = "Apply only if the facts still have this fingerprint, the one --dry-run printed"
	repeatableUsage        = " (repeatable)"
	decisionsUsage         = " (KEY=VALUE, repeatable)"
	flagPrefix             = "--"
	positionalFmt          = "<%s>"
	unknownEntryFmt        = "cli.Bind: the registry holds no %q"
	positionalTypeFmt      = "cli.Bind: %s cannot be positional: it is a %s field"
	detailOwn              = "own"
	detailGeneric          = "generic"
)

// Doc is what only a terminal reads about a command; its flags come from the
// registry.
type Doc struct {
	Use     string
	Short   string
	Long    string
	Example string
}

type Option func(*binding)

// Positional takes the field at path from the arguments rather than a flag: a
// list field all of them, any other field one.
func Positional(path string) Option {
	return func(b *binding) { b.positional = path }
}

type FlagParams struct {
	Path  string
	Name  string
	Short string
}

// Flag spells a field's flag other than its path in kebab case, or gives it a
// short form.
func Flag(params FlagParams) Option {
	return func(b *binding) { b.renamed[params.Path] = params }
}

// Detail renders what is the command's own in its plan and its items.
func Detail(detail render.Detail) Option {
	return func(b *binding) { b.detail, b.detailKind = detail, detailOwn }
}

// GenericDetail declares the generic rendering enough for the command.
func GenericDetail() Option {
	return func(b *binding) { b.detail, b.detailKind = render.GenericDetail{}, detailGeneric }
}

// binding is one command of the registry served on the command line.
type binding struct {
	name       string
	engine     *Engine
	specs      []kernel.FieldSpec
	positional string
	renamed    map[string]FlagParams
	detail     render.Detail
	detailKind string
	cmd        *cobra.Command
	exitCodes  map[kernel.Code]int
}

// Bind generates the command named name from the registry.
func Bind(name string, doc Doc, options ...Option) *cobra.Command {
	return production.Bind(name, doc, options...)
}

// Bind panics on a name the registry does not hold, as the tree is built at
// start-up: a wrong name never reaches a user.
func (e *Engine) Bind(name string, doc Doc, options ...Option) *cobra.Command {
	entry, found := e.schema().Lookup(name)
	if !found {
		panic(fmt.Sprintf(unknownEntryFmt, name))
	}
	b := &binding{name: name, engine: e, specs: entry.Schema(), exitCodes: entry.ExitCodes(), renamed: map[string]FlagParams{}, detail: render.GenericDetail{}}
	for _, option := range options {
		option(b)
	}
	cmd := &cobra.Command{
		Use:         doc.Use,
		Short:       doc.Short,
		Long:        doc.Long,
		Example:     doc.Example,
		Args:        cobra.ArbitraryArgs,
		RunE:        b.run,
		Annotations: map[string]string{annotationEntry: name, annotationDetail: b.detailKind},
	}
	for _, spec := range b.specs {
		b.addField(cmd, spec)
	}
	b.addEngineFlags(cmd)
	cmd.SetFlagErrorFunc(b.flagError)
	b.cmd = cmd
	e.bindings[name] = b
	return cmd
}

func (b *binding) addField(cmd *cobra.Command, spec kernel.FieldSpec) {
	if spec.Path == b.positional {
		b.checkPositional(spec)
		return
	}
	flag := b.flagOf(spec.Path)
	usage := fieldUsage(spec)
	switch spec.Type {
	case kernel.FieldBool:
		cmd.Flags().BoolP(flag.Name, flag.Short, false, usage)
	case kernel.FieldTextList, kernel.FieldMultiSelect, kernel.FieldReorder:
		cmd.Flags().StringArrayP(flag.Name, flag.Short, nil, usage+repeatableUsage)
	case kernel.FieldDecisions:
		cmd.Flags().StringArrayP(flag.Name, flag.Short, nil, usage+decisionsUsage)
	default:
		cmd.Flags().StringP(flag.Name, flag.Short, "", usage)
	}
	if enum := spec.Constraints.Enum; len(enum) > 0 {
		_ = cmd.RegisterFlagCompletionFunc(flag.Name, cobra.FixedCompletions(enum, cobra.ShellCompDirectiveNoFileComp))
	}
}

func (b *binding) checkPositional(spec kernel.FieldSpec) {
	if spec.Type == kernel.FieldBool || spec.Type == kernel.FieldDecisions {
		panic(fmt.Sprintf(positionalTypeFmt, spec.Path, spec.Type))
	}
}

func fieldUsage(spec kernel.FieldSpec) string {
	if spec.Description != "" {
		return spec.Description
	}
	return spec.Label
}

// addEngineFlags adds what every command of the registry takes, once.
func (b *binding) addEngineFlags(cmd *cobra.Command) {
	shared.AddYesFlag(cmd, yesUsage)
	shared.AddOutputFlag(cmd)
	cmd.Flags().Bool(domain.FlagDryRun, false, dryRunUsage)
	cmd.Flags().String(flagExpectFingerprint, "", expectFingerprintUsage)
	if slices.ContainsFunc(b.specs, func(spec kernel.FieldSpec) bool { return spec.Rememberable }) {
		shared.AddAskFlag(cmd)
	}
}

func (b *binding) flagOf(path string) FlagParams {
	flag, renamed := b.renamed[path]
	if renamed {
		return flag
	}
	return FlagParams{Path: path, Name: kebab(path)}
}

func kebab(path string) string {
	return strings.NewReplacer(".", "-", "_", "-").Replace(path)
}

func (b *binding) spec(path string) (kernel.FieldSpec, bool) {
	at := slices.IndexFunc(b.specs, func(spec kernel.FieldSpec) bool { return spec.Path == path })
	if at < 0 {
		return kernel.FieldSpec{}, false
	}
	return b.specs[at], true
}

// spell names a field path the way the command line takes it: its flag, or
// its positional; an entry keeps its index ("<branches>[2]").
func (b *binding) spell(path string) string {
	field, index, indexed := strings.Cut(path, "[")
	entry := entrySuffix(index, indexed)
	if field == b.positional {
		return fmt.Sprintf(positionalFmt, field) + entry
	}
	if _, known := b.spec(field); known {
		return flagPrefix + b.flagOf(field).Name + entry
	}
	return path
}

func entrySuffix(index string, indexed bool) string {
	if !indexed {
		return ""
	}
	return "[" + index
}

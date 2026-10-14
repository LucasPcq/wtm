package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// writeSinks are the calls that change something outside the process: a file,
// a repository, a program run. infra/ is classified here rather than read,
// because its readers run git too; everything above it is read.
var writeSinks = []objectRef{
	{Path: "os", Name: "WriteFile"},
	{Path: "os", Name: "Create"},
	{Path: "os", Name: "Rename"},
	{Path: "os", Name: "Remove"},
	{Path: "os", Name: "RemoveAll"},
	{Path: "os", Name: "OpenFile"},
	{Path: "os/exec", Name: "Command"},
	{Path: "os/exec", Name: "CommandContext"},
	{Path: internalPrefix + "infra/fs", Name: "UpdateFile"},
	{Path: internalPrefix + "infra/fs", Name: "RemoveDir"},
	{Path: internalPrefix + "infra/fs", Name: "ReplaceFile"},
	{Path: internalPrefix + "infra/fs", Name: "OpenLog"},
	{Path: internalPrefix + "infra/proc", Name: "Command"},
	{Path: internalPrefix + "infra/proc", Name: "GroupCommand"},
}

// writePorts are the interfaces of infra/ whose every method writes: a call
// through one, or to the method of a type of that package implementing it,
// makes its caller a mutator. git.Reader is not one, so a read stays a read.
var writePorts = []objectRef{
	{Path: internalPrefix + "infra/git", Name: "Writer"},
}

// engineServices are the services the engine reaches a worktree through, each
// function a reader or a mutator; a service joins the list when its command
// moves onto the engine.
var engineServices = []dir{"service/gitwt", "service/identity", "service/runenv", "service/hooks"}

// mutatorCallers may call a mutator of the engine's services: a command's
// Apply, a service composing it into a mutator of its own, and the bridge of
// the commands still on flow/.
var mutatorCallers = []dir{"command", "service", "flow"}

// readOnly are the packages that never write: a query only reads.
var readOnly = []dir{"query"}

// writingField is the one function of the engine's contract that writes; every
// other one (Observe, Choices, Plan, Skip, Run…) is a reader.
const writingField = "Apply"

// mutatorFact marks a function that reaches a write sink, with the chain of
// calls down to it; a port's method is a sink itself.
type mutatorFact struct{ Via string }

func (*mutatorFact) AFact() {}

func (f *mutatorFact) String() string {
	if f.Via == "" {
		return "mutator"
	}
	return "mutator via " + f.Via
}

var writesinkAnalyzer = &analysis.Analyzer{
	Name:      "writesink",
	Doc:       "a function that reaches a write sink is a mutator: never called from query/ or from a reader of the engine's contract, and a mutator of the engine's services only from command/",
	Run:       runWritesink,
	FactTypes: []analysis.Fact{new(mutatorFact)},
}

func runWritesink(pass *analysis.Pass) (any, error) {
	pkg := pass.Pkg.Path()
	if internalPath(pkg) == "" {
		return nil, nil
	}
	if dir("infra").holds(pkg) {
		exportPortFacts(pass)
		return nil, nil
	}
	writes := mutatorsOf(pass)
	for fn, via := range writes {
		pass.ExportObjectFact(fn, &mutatorFact{Via: via})
	}
	exportInterfaceFacts(pass, writes)
	check := writeCheck{pass: pass, writes: writes}
	if slices.ContainsFunc(readOnly, func(d dir) bool { return d.holds(pkg) }) {
		check.readOnlyPackage()
		return nil, nil
	}
	check.readerFields()
	check.engineMutatorCalls()
	return nil, nil
}

// writeOf says how a use of fn writes: fn itself and the chain of calls down
// to the sink, ok false when it reads.
func writeOf(pass *analysis.Pass, writes map[*types.Func]string, fn *types.Func) (string, bool) {
	if via, ok := writes[fn]; ok {
		return chain(fn, via), true
	}
	if slices.ContainsFunc(writeSinks, func(ref objectRef) bool { return refers(fn, ref) }) {
		return qualified(fn), true
	}
	fact := new(mutatorFact)
	if fn.Pkg() != pass.Pkg && pass.ImportObjectFact(fn, fact) {
		return chain(fn, fact.Via), true
	}
	return "", false
}

func chain(fn *types.Func, via string) string {
	if via == "" {
		return qualified(fn)
	}
	return qualified(fn) + " → " + via
}

// written is the sentence a finding opens with: what writes, and how.
func written(path string) string {
	name, via, ok := strings.Cut(path, " → ")
	if !ok {
		return name + " writes"
	}
	return name + " writes (" + via + ")"
}

func qualified(fn *types.Func) string {
	if fn.Pkg() == nil {
		return fn.Name()
	}
	if recv := receiverName(fn); recv != "" {
		return fmt.Sprintf("%s.%s.%s", fn.Pkg().Name(), recv, fn.Name())
	}
	return fn.Pkg().Name() + "." + fn.Name()
}

func receiverName(fn *types.Func) string {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return ""
	}
	recv := sig.Recv().Type()
	if pointer, ok := recv.(*types.Pointer); ok {
		recv = pointer.Elem()
	}
	if named, ok := recv.(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}

// mutatorsOf is every function of the package that reaches a write sink, its
// own or through another function, of this package or of one already marked.
func mutatorsOf(pass *analysis.Pass) map[*types.Func]string {
	calls := map[*types.Func][]*types.Func{}
	writes := map[*types.Func]string{}
	for _, file := range pass.Files {
		for _, d := range file.Decls {
			decl, ok := d.(*ast.FuncDecl)
			if !ok || decl.Body == nil {
				continue
			}
			fn, ok := pass.TypesInfo.Defs[decl.Name].(*types.Func)
			if !ok {
				continue
			}
			ast.Inspect(decl.Body, func(node ast.Node) bool {
				callee, ok := usedFunc(pass, node)
				switch {
				case !ok:
				case callee.Pkg() == pass.Pkg:
					calls[fn] = append(calls[fn], callee)
				default:
					if path, ok := writeOf(pass, nil, callee); ok && !marked(writes, fn) {
						writes[fn] = path
					}
				}
				return true
			})
		}
	}
	for changed := true; changed; {
		changed = false
		for fn, callees := range calls {
			if _, ok := writes[fn]; ok {
				continue
			}
			for _, callee := range callees {
				if via, ok := writes[callee]; ok {
					writes[fn] = chain(callee, via)
					changed = true
					break
				}
			}
		}
	}
	return writes
}

func marked(writes map[*types.Func]string, fn *types.Func) bool {
	_, ok := writes[fn]
	return ok
}

func usedFunc(pass *analysis.Pass, node ast.Node) (*types.Func, bool) {
	ident, ok := node.(*ast.Ident)
	if !ok {
		return nil, false
	}
	fn, ok := pass.TypesInfo.Uses[ident].(*types.Func)
	if !ok {
		return nil, false
	}
	// A generic function is marked once, on its declaration, not per instance.
	return fn.Origin(), true
}

// exportPortFacts marks the methods of infra's write ports, on the interface
// and on every type of the package that implements it.
func exportPortFacts(pass *analysis.Pass) {
	for _, port := range writePorts {
		if port.Path != pass.Pkg.Path() {
			continue
		}
		iface, ok := pass.Pkg.Scope().Lookup(port.Name).Type().Underlying().(*types.Interface)
		if !ok {
			continue
		}
		for method := range iface.Methods() {
			pass.ExportObjectFact(method, &mutatorFact{})
		}
		for _, impl := range implementations(pass.Pkg, iface) {
			for method := range iface.Methods() {
				if own := methodOf(impl, method.Name()); own != nil {
					pass.ExportObjectFact(own, &mutatorFact{})
				}
			}
		}
	}
}

// exportInterfaceFacts marks a method of an interface of the package as a
// mutator when a type of the package implementing it writes in that method:
// a call through store.Meta writes as the call to store.MetaDisk does.
func exportInterfaceFacts(pass *analysis.Pass, writes map[*types.Func]string) {
	for _, name := range pass.Pkg.Scope().Names() {
		obj, ok := pass.Pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		iface, ok := obj.Type().Underlying().(*types.Interface)
		if !ok {
			continue
		}
		for _, impl := range implementations(pass.Pkg, iface) {
			for method := range iface.Methods() {
				if via, ok := writes[methodOf(impl, method.Name())]; ok && method.Pkg() == pass.Pkg {
					pass.ExportObjectFact(method, &mutatorFact{Via: via})
				}
			}
		}
	}
}

func implementations(pkg *types.Package, iface *types.Interface) []types.Type {
	var impls []types.Type
	for _, name := range pkg.Scope().Names() {
		obj, ok := pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok || types.IsInterface(obj.Type()) {
			continue
		}
		if pointer := types.NewPointer(obj.Type()); types.Implements(pointer, iface) {
			impls = append(impls, pointer)
		}
	}
	return impls
}

func methodOf(typ types.Type, name string) *types.Func {
	obj, _, _ := types.LookupFieldOrMethod(typ, true, nil, name)
	fn, _ := obj.(*types.Func)
	return fn
}

type writeCheck struct {
	pass   *analysis.Pass
	writes map[*types.Func]string
}

func (c writeCheck) readOnlyPackage() {
	for _, file := range c.pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			c.refuse(node, "%s, in %s: a query only reads (docs/dev/lint.md, writesink)", internalPath(c.pass.Pkg.Path()))
			return true
		})
	}
}

// readerFields refuses a write from any function of a literal of the engine's
// contract but Apply, whether written in place or named.
func (c writeCheck) readerFields() {
	for _, file := range c.pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			lit, ok := node.(*ast.CompositeLit)
			if !ok || !kernelContract(c.pass.TypesInfo.TypeOf(lit)) {
				return true
			}
			for _, elt := range lit.Elts {
				field, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := field.Key.(*ast.Ident)
				if !ok || key.Name == writingField {
					continue
				}
				c.readerBody(key.Name, field.Value)
			}
			return true
		})
	}
}

func (c writeCheck) readerBody(field string, value ast.Expr) {
	if lit, ok := value.(*ast.FuncLit); ok {
		ast.Inspect(lit.Body, func(node ast.Node) bool {
			c.refuse(node, "%s, in "+field+": only Apply writes, the rest of a command reads (docs/dev/lint.md, writesink)")
			return true
		})
		return
	}
	var name *ast.Ident
	switch v := value.(type) {
	case *ast.Ident:
		name = v
	case *ast.SelectorExpr:
		name = v.Sel
	default:
		return
	}
	c.refuse(name, "%s, as "+field+": only Apply writes, the rest of a command reads (docs/dev/lint.md, writesink)")
}

func kernelContract(typ types.Type) bool {
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	named, ok := typ.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == internalPrefix+"kernel"
}

// engineMutatorCalls refuses a mutator of the engine's services called from a
// package that is not one of its callers: a surface, dispatch, a rule.
func (c writeCheck) engineMutatorCalls() {
	if slices.ContainsFunc(mutatorCallers, func(d dir) bool { return d.holds(c.pass.Pkg.Path()) }) {
		return
	}
	for _, file := range c.pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			callee, ok := usedFunc(c.pass, node)
			if !ok || callee.Pkg() == nil || !slices.ContainsFunc(engineServices, func(d dir) bool { return d.holds(callee.Pkg().Path()) }) {
				return true
			}
			if path, ok := writeOf(c.pass, c.writes, callee); ok {
				c.pass.Reportf(node.Pos(), "%s, called from %s: a mutator of the engine's services is called from command/ only (docs/dev/services.md, writesink)", written(path), internalPath(c.pass.Pkg.Path()))
			}
			return true
		})
	}
}

func (c writeCheck) refuse(node ast.Node, format string, args ...any) {
	callee, ok := usedFunc(c.pass, node)
	if !ok {
		return
	}
	if path, ok := writeOf(c.pass, c.writes, callee); ok {
		c.pass.Reportf(node.Pos(), format, append([]any{written(path)}, args...)...)
	}
}

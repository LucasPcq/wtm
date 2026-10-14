package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var writeSinks = []objectRef{
	{Path: "os", Name: "WriteFile"},
	{Path: "os", Name: "Create"},
	{Path: "os", Name: "CreateTemp"},
	{Path: "os", Name: "Rename"},
	{Path: "os", Name: "Remove"},
	{Path: "os", Name: "RemoveAll"},
	{Path: "os", Name: "OpenFile"},
	{Path: "os", Name: "Mkdir"},
	{Path: "os", Name: "MkdirAll"},
	{Path: "os", Name: "MkdirTemp"},
	{Path: "os", Name: "Symlink"},
	{Path: "os", Name: "Link"},
	{Path: "os", Name: "Chmod"},
	{Path: "os", Name: "Chown"},
	{Path: "os", Name: "Lchown"},
	{Path: "os", Name: "Chtimes"},
	{Path: "os", Name: "Truncate"},
	{Path: "os", Name: "Setenv"},
	{Path: "os", Name: "Unsetenv"},
	{Path: "os", Name: "Clearenv"},
	{Path: "os", Name: "Chdir"},
	{Path: "os/exec", Name: "Command"},
	{Path: "os/exec", Name: "CommandContext"},
	{Path: "syscall", Name: "Kill"},
	{Path: internalPrefix + "infra/fs", Name: "UpdateFile"},
	{Path: internalPrefix + "infra/fs", Name: "RemoveDir"},
	{Path: internalPrefix + "infra/fs", Name: "ReplaceFile"},
	{Path: internalPrefix + "infra/fs", Name: "OpenLog"},
	{Path: internalPrefix + "infra/proc", Name: "Command"},
	{Path: internalPrefix + "infra/proc", Name: "GroupCommand"},
}

var methodSinks = []methodRef{
	{Path: "os", Recv: "File", Names: []string{"Write", "WriteString", "WriteAt", "ReadFrom", "Truncate", "Chmod", "Chown", "Chdir"}},
	{Path: "os", Recv: "Process", Names: []string{"Kill", "Signal"}},
}

type methodRef struct {
	Path  string
	Recv  string
	Names []string
}

func (ref methodRef) holds(fn *types.Func) bool {
	return fn.Pkg() != nil && fn.Pkg().Path() == ref.Path && receiverName(fn) == ref.Recv && slices.Contains(ref.Names, fn.Name())
}

var writePorts = []objectRef{
	{Path: internalPrefix + "infra/git", Name: "Writer"},
}

// Via is the chain of calls down to the sink, empty on a port's method.
type mutatorFact struct{ Via string }

func (*mutatorFact) AFact() {}

func (f *mutatorFact) String() string {
	if f.Via == "" {
		return "mutator"
	}
	return "mutator via " + f.Via
}

func writeOf(pass *analysis.Pass, writes map[*types.Func]string, fn *types.Func) (string, bool) {
	if via, ok := writes[fn]; ok {
		return chain(fn, via), true
	}
	if slices.ContainsFunc(writeSinks, func(ref objectRef) bool { return refers(fn, ref) }) || slices.ContainsFunc(methodSinks, func(ref methodRef) bool { return ref.holds(fn) }) {
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

package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// A code with no message reaches its reader as its bare key. The kernel's
// codes are spoken by kernel/text, which imports kernel and so is analysed
// after it: kernel hands them over as a fact. Every other package of the
// engine speaks its own codes, in a text.Messages beside them.
var (
	catalogScope    = []dir{"kernel", "dispatch", "command", "query"}
	codeRef         = objectRef{Path: internalPrefix + "kernel", Name: "Code"}
	messagesRef     = objectRef{Path: internalPrefix + "kernel/text", Name: "Messages"}
	kernelCatalogue = internalPrefix + "kernel/text"
)

type kernelCodesFact struct{ Codes []declaredCode }

type declaredCode struct {
	Value string
	At    string
	Pos   token.Pos
}

func (*kernelCodesFact) AFact() {}

func (f *kernelCodesFact) String() string { return fmt.Sprintf("%d kernel codes", len(f.Codes)) }

var catalogAnalyzer = &analysis.Analyzer{
	Name:      "catalog",
	Doc:       "every kernel.Code declared in kernel/, dispatch/, command/ and query/ has its message in its package's catalogue",
	Run:       runCatalog,
	FactTypes: []analysis.Fact{new(kernelCodesFact)},
}

func runCatalog(pass *analysis.Pass) (any, error) {
	pkg := pass.Pkg.Path()
	if !slices.ContainsFunc(catalogScope, func(d dir) bool { return d.holds(pkg) }) {
		return nil, nil
	}
	declared := declaredCodes(pass)
	if pkg == codeRef.Path {
		pass.ExportPackageFact(&kernelCodesFact{Codes: declared})
		return nil, nil
	}
	spoken := spokenCodes(pass)
	for _, code := range append(declared, importedKernelCodes(pass)...) {
		if spoken[code.Value] {
			continue
		}
		pass.Reportf(code.Pos, "code %q (%s) has no message in the catalogue of %s: a code reaches its reader as a sentence or not at all (docs/dev/commands.md)", code.Value, code.At, internalPath(pkg))
	}
	return nil, nil
}

func declaredCodes(pass *analysis.Pass) []declaredCode {
	var declared []declaredCode
	seen := map[string]bool{}
	add := func(value constant.Value, pos token.Pos) {
		if value == nil || value.Kind() != constant.String || seen[constant.StringVal(value)] {
			return
		}
		seen[constant.StringVal(value)] = true
		at := pass.Fset.Position(pos)
		declared = append(declared, declaredCode{Value: constant.StringVal(value), At: fmt.Sprintf("%s:%d", filepath.Base(at.Filename), at.Line), Pos: pos})
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.Ident:
				if obj, ok := pass.TypesInfo.Defs[typed].(*types.Const); ok && isCode(obj.Type()) {
					add(obj.Val(), typed.Pos())
				}
			case *ast.CallExpr:
				if fun, ok := pass.TypesInfo.Types[typed.Fun]; ok && fun.IsType() && isCode(fun.Type) {
					add(pass.TypesInfo.Types[typed].Value, typed.Pos())
				}
			}
			return true
		})
	}
	return declared
}

// spokenCodes are the keys of the package's text.Messages; a key written as a
// bare string is reported, since no constant declares it.
func spokenCodes(pass *analysis.Pass) map[string]bool {
	spoken := map[string]bool{}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			lit, ok := node.(*ast.CompositeLit)
			if !ok || !isNamed(pass.TypesInfo.TypeOf(lit), messagesRef) {
				return true
			}
			for _, elt := range lit.Elts {
				pair, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if _, bare := pair.Key.(*ast.BasicLit); bare {
					pass.Reportf(pair.Key.Pos(), "message keyed by a bare string: key it by the kernel.Code constant it speaks")
					continue
				}
				if value := pass.TypesInfo.Types[pair.Key].Value; value != nil && value.Kind() == constant.String {
					spoken[constant.StringVal(value)] = true
				}
			}
			return true
		})
	}
	return spoken
}

func importedKernelCodes(pass *analysis.Pass) []declaredCode {
	if pass.Pkg.Path() != kernelCatalogue {
		return nil
	}
	for _, imported := range pass.Pkg.Imports() {
		fact := new(kernelCodesFact)
		if imported.Path() != codeRef.Path || !pass.ImportPackageFact(imported, fact) {
			continue
		}
		codes := slices.Clone(fact.Codes)
		for i := range codes {
			codes[i].Pos = pass.Files[0].Name.Pos()
		}
		return codes
	}
	return nil
}

func isCode(t types.Type) bool { return isNamed(t, codeRef) }

func isNamed(t types.Type, ref objectRef) bool {
	named, ok := t.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == ref.Path && named.Obj().Name() == ref.Name
}

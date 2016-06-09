// Parse interface declarations.

package parser

import (
	"go/ast"
	"go/token"
	"log"
)

// An Interface is a representation of an interface type.
type Interface struct {
	// The name of the interface type.
	Name string

	// The methods in the interface.
	Methods []Method
}

// A Method is a representation of a member function of an interface
// type.
type Method struct {
	// The name of the method.
	Name string

	// The type.
	Type ast.FuncType
}

// Extract all interface declarations from the AST of a file.
//
// If the supplied AST is nil, nil will be returned; in all other
// cases a slice (possibly 0-sized) will be returned.
func ParseInterfacesFromAST(theAST *ast.File) []Interface {
	if theAST == nil {
		return nil
	}

	interfaces := make([]Interface, 0)

	ast.Inspect(theAST, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		// We're interested in interface type specifications,
		// which we get from GenDecl.Specs -> TypeSpec.Type ->
		// InterfaceType.
		switch x := n.(type) {
		case *ast.GenDecl:
			if x.Tok == token.TYPE && x.Specs != nil {
				for _, spec := range x.Specs {
					switch tyspec := spec.(type) {
					case *ast.TypeSpec:
						name := tyspec.Name.Name
						log.Println("Found tyspec", name)
						switch ifacety := tyspec.Type.(type) {
						case *ast.InterfaceType:
							methods := make([]Method, 0)
							if ifacety.Methods != nil {
								for _, field := range ifacety.Methods.List {
									if field.Names == nil || len(field.Names) < 1 {
										continue
									}

									obj := field.Names[0].Obj

									if obj == nil || obj.Decl == nil {
										continue
									}
									switch funcdecl := obj.Decl.(type) {
									case *ast.Field:
										if funcdecl.Type == nil {
											continue
										}
										switch functype := funcdecl.Type.(type) {
										case *ast.FuncType:
											if functype == nil {
												continue
											}
											method := Method{Name: field.Names[0].Name, Type: *functype}
											methods = append(methods, method)
										}
									}
								}
							}
							interfaceType := Interface{Name: name, Methods: methods}
							interfaces = append(interfaces, interfaceType)
						}
					}
				}
			}
		}

		return true
	})

	return interfaces

}

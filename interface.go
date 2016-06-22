// Parse interface declarations.
//
// For all of the InterfaceFrom* functions, if the parameter is not an
// interface type, the default interface is returned and the error is
// non-nil. Similarly for the FunctionFrom* functions.

package main

import (
	"errors"
	"fmt"
	"go/ast"
)

// A Function is a representation of a function name and type, which
// may be a member of an interface or not. All Functions in an
// Interface will be members.
type Function struct {
	// The name of the function.
	Name string

	// The parameter types
	Parameters []Type

	// The output types
	Returns []Type
}

// Type is a representation of a Go type. The concrete types are
// ArrayType, BasicType, ChanType, MapType, PointerType, and
// QualifiedType.
type Type interface {
	// Return an unambiguous string rendition of the type.
	ToString() string
}

// ArrayType is the type of arrays.
type ArrayType struct {
	// The element type
	ElementType Type
}

// ToString converts an ArrayType into a string of the form
// "[](type)".
func (ty *ArrayType) ToString() string {
	if ty == nil {
		return ""
	}

	tystr := fmt.Sprintf("[](%s)", ty.ElementType.ToString())
	return tystr
}

// BasicType is simple named types with no additional structure.
type BasicType string

// ToString just exposes the underlying type name.
func (ty *BasicType) ToString() string {
	if ty == nil {
		return ""
	}

	return string(*ty)
}

// ChanType is the type of channels.
type ChanType struct {
	// The element type.
	ElementType Type
}

// ToString converts a ChanType into a string of the form "chan
// (type)".
func (ty *ChanType) ToString() string {
	if ty == nil {
		return ""
	}

	tystr := fmt.Sprintf("chan (%s)", ty.ElementType.ToString())
	return tystr
}

// MapType is the type of maps.
type MapType struct {
	// The key type
	KeyType Type
	// The value type.
	ValueType Type
}

// ToString converts a MapType into a string of the form
// "map[type](type)".
func (ty *MapType) ToString() string {
	if ty == nil {
		return ""
	}

	tystr := fmt.Sprintf("map[%s](%s)", ty.KeyType.ToString(), ty.ValueType.ToString())
	return tystr
}

// PointerType is the type of pointers.
type PointerType struct {
	// The target type.
	TargetType Type
}

// QualifiedType is the type of types with a package name qualfiier.
type QualifiedType struct {
	// The package name
	Package string
	// The type
	Type Type
}

// ToString converts a QualifiedType into a string of the form
// "package.type".
func (ty *QualifiedType) ToString() string {
	if ty == nil {
		return ""
	}

	tystr := fmt.Sprintf("%s.%s", ty.Package, ty.Type.ToString())
	return tystr
}

// ToString converts a PointerType into a string of the form
// "*(type)".
func (ty *PointerType) ToString() string {
	if ty == nil {
		return ""
	}

	tystr := fmt.Sprintf("*(%s)", ty.TargetType.ToString())
	return tystr
}

// InterfacesFromAST extracts all interface declarations from the AST
// of a file, as a map from names to interface decls.
func InterfacesFromAST(theAST *ast.File) map[string][]Function {
	if theAST == nil {
		return nil
	}

	interfaces := make(map[string][]Function)

	ast.Inspect(theAST, func(node ast.Node) bool {
		switch tyspec := node.(type) {
		case *ast.TypeSpec:
			name := tyspec.Name.Name
			switch ifacety := tyspec.Type.(type) {
			case *ast.InterfaceType:
				functions, err := FunctionsFromInterfaceType(*ifacety)
				if err == nil {
					interfaces[name] = functions
				}
			}

			// Whether we found one or not, there can be
			// no interfaces at a lower level than this.
			return false
		}

		// Maybe we just haven't recursed deeply
		// enough. Onwards!
		return true
	})

	return interfaces
}

// FunctionsFromInterfaceType tries to extract function declarations
// from an ast.InterfaceType.
func FunctionsFromInterfaceType(ifacety ast.InterfaceType) ([]Function, error) {
	if ifacety.Methods == nil {
		return []Function{}, errors.New("Interface method slice is nil")
	}

	var functions []Function
	for _, field := range ifacety.Methods.List {
		if field.Names == nil || len(field.Names) == 0 {
			continue
		}

		// Can there be more than one name?
		name := field.Names[0].Name
		obj := field.Names[0].Obj

		if obj == nil || obj.Decl == nil {
			continue
		}

		switch fundecl := obj.Decl.(type) {
		case *ast.Field:
			ast.Inspect(fundecl, func(node ast.Node) bool {
				switch funty := node.(type) {
				case *ast.FuncType:
					parameters := TypeListFromFieldList(*funty.Params)
					returns := TypeListFromFieldList(*funty.Results)
					function := Function{Name: name, Parameters: parameters, Returns: returns}
					functions = append(functions, function)
					return false
				}

				return true
			})
		}
	}

	return functions, nil
}

// TypeListFromFieldList gets the list of type names from an
// ast.FieldList. Names are not returned.
func TypeListFromFieldList(fields ast.FieldList) []Type {
	var types []Type

	ast.Inspect(&fields, func(node ast.Node) bool {
		switch tyexpr := node.(type) {
		case ast.Expr:
			ty := TypeFromTypeExpr(tyexpr)
			if ty != nil {
				types = append(types, ty)
			}
			return false
		}

		return true
	})

	return types
}

// TypeFromTypeExpr gets a type from an ast.Expr which is known to
// represent a type.
func TypeFromTypeExpr(ty ast.Expr) Type {
	switch x := ty.(type) {
	case *ast.Ident:
		// Type name
		ty := BasicType(x.Name)
		return &ty
	case *ast.ArrayType:
		ty := ArrayType{ElementType: TypeFromTypeExpr(x.Elt)}
		return &ty
	case *ast.ChanType:
		ty := ChanType{ElementType: TypeFromTypeExpr(x.Value)}
		return &ty
	case *ast.MapType:
		ty := MapType{KeyType: TypeFromTypeExpr(x.Key), ValueType: TypeFromTypeExpr(x.Value)}
		return &ty
	case *ast.StarExpr:
		ty := PointerType{TargetType: TypeFromTypeExpr(x.X)}
		return &ty
	case *ast.SelectorExpr:
		// x.X is an expression which resolves to the package
		// name and x.Sel is the "selector", which is the
		// actual type name.
		pkg := TypeFromTypeExpr(x.X).ToString()
		innerTy := BasicType(x.Sel.Name)
		ty := QualifiedType{Package: pkg, Type: &innerTy}
		return &ty
	}

	return nil
}

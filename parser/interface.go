// Parse interface declarations.
//
// For all of the InterfaceFrom* functions, if the parameter is not an
// interface type, the default interface is returned and the error is
// non-nil. Similarly for the MethodFrom* functions.

package parser

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
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

	// The parameter types
	Parameters []Type

	// The output types
	Returns []Type
}

// Type is a representation of a Go type. The concrete types are
// ArrayType, BasicType, ChanType, MapType, and PointerType.
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
// of a file.
func InterfacesFromAST(theAST *ast.File) []Interface {
	if theAST == nil {
		return nil
	}

	var interfaces []Interface

	ast.Inspect(theAST, func(node ast.Node) bool {
		iface, err := InterfaceFromNode(node)
		if err == nil {
			interfaces = append(interfaces, iface)
		}
		return true
	})

	return interfaces
}

// InterfaceFromNode tries to extract an interface from an ast.Node.
func InterfaceFromNode(node ast.Node) (Interface, error) {
	if node == nil {
		return Interface{}, errors.New("Node is nil.")
	}
	switch gendecl := node.(type) {
	case *ast.GenDecl:
		iface, err := InterfaceFromGenDecl(gendecl)
		return iface, err
	}
	return Interface{}, errors.New("Node is not a GenDecl.")
}

// InterfaceFromGenDecl tries to extract an interface from an
// *ast.GenDecl.
func InterfaceFromGenDecl(gendecl *ast.GenDecl) (Interface, error) {
	if gendecl == nil {
		return Interface{}, errors.New("GenDecl is nil.")
	}
	if gendecl.Tok != token.TYPE {
		return Interface{}, errors.New("GenDecl is not a type declaration.")
	}
	if gendecl.Specs == nil {
		return Interface{}, errors.New("GenDecl contains no specifications.")
	}

	// Will there ever be more than one Spec? I don't know. To be
	// safe, loop over them and return the first interface.
	for _, spec := range gendecl.Specs {
		iface, err := InterfaceFromSpec(spec)
		if err == nil {
			return iface, nil
		}
	}

	return Interface{}, errors.New("GenDecl contains no type specifications.")
}

// InterfaceFromSpec tries to extract an interface from an ast.Spec.
func InterfaceFromSpec(spec ast.Spec) (Interface, error) {
	if spec == nil {
		return Interface{}, errors.New("Spec is nil.")
	}

	switch tyspec := spec.(type) {
	case *ast.TypeSpec:
		iface, err := InterfaceFromTypeSpec(tyspec)
		return iface, err
	}

	return Interface{}, errors.New("Spec is not a TypeSpec.")
}

// InterfaceFromTypeSpec tries to extract an interface from an
// *ast.TypeSpec.
func InterfaceFromTypeSpec(tyspec *ast.TypeSpec) (Interface, error) {
	if tyspec == nil {
		return Interface{}, errors.New("TypeSpec is nil.")
	}

	switch ifacety := tyspec.Type.(type) {
	case *ast.InterfaceType:
		name := tyspec.Name.Name
		var methods []Method
		if ifacety.Methods != nil {
			for _, field := range ifacety.Methods.List {
				if field.Names == nil || len(field.Names) < 1 {
					continue
				}

				// Can there be more than one name?
				obj := field.Names[0].Obj

				if obj == nil || obj.Decl == nil {
					continue
				}

				switch fundecl := obj.Decl.(type) {
				case *ast.Field:
					method, err := MethodFromField(field.Names[0].Name, fundecl)

					if err == nil {
						methods = append(methods, method)
					}
				}
			}
		}
		return Interface{Name: name, Methods: methods}, nil
	}

	return Interface{}, errors.New("TypeSpec is not an interface type.")
}

// MethodFromField tries to extract a method from an *ast.Field.
func MethodFromField(name string, field *ast.Field) (Method, error) {
	if field == nil {
		return Method{}, errors.New("Field is nil.")
	}
	if field.Type == nil {
		return Method{}, errors.New("Field type is nil.")
	}

	switch functype := field.Type.(type) {
	case *ast.FuncType:
		method, err := MethodFromFuncType(name, functype)
		return method, err
	}

	return Method{}, errors.New("Type is not a function type.")
}

// MethodFromFuncType tries to extract a method from an *ast.FuncType.
func MethodFromFuncType(name string, funty *ast.FuncType) (Method, error) {
	if funty == nil {
		return Method{}, errors.New("FuncType is nil.")
	}

	parameters := typeList(funty.Params)
	returns := typeList(funty.Results)

	return Method{Name: name, Parameters: parameters, Returns: returns}, nil
}

// Get the list of type names from an *ast.FieldList. Names are not
// returned.
func typeList(fields *ast.FieldList) []Type {
	var types []Type

	if fields == nil || fields.List == nil {
		return types
	}

	for _, field := range fields.List {
		ty := typeFromField(field)
		if ty != nil {
			types = append(types, ty)
		}
	}

	return types
}

// Get a type from an *ast.Field.
func typeFromField(field *ast.Field) Type {
	if field == nil {
		return nil
	}

	return typeFromTypeExpr(field.Type)
}

// Get a type from an ast.Expr which is known to represent a type.
func typeFromTypeExpr(ty ast.Expr) Type {
	switch x := ty.(type) {
	case *ast.Ident:
		// Type name
		ty := BasicType(x.Name)
		return &ty
	case *ast.ArrayType:
		ty := ArrayType{ElementType: typeFromTypeExpr(x.Elt)}
		return &ty
	case *ast.ChanType:
		ty := ChanType{ElementType: typeFromTypeExpr(x.Value)}
		return &ty
	case *ast.MapType:
		ty := MapType{KeyType: typeFromTypeExpr(x.Key), ValueType: typeFromTypeExpr(x.Value)}
		return &ty
	case *ast.StarExpr:
		ty := PointerType{TargetType: typeFromTypeExpr(x.X)}
		return &ty
	}

	return nil
}

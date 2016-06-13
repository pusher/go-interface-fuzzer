// Parse fuzzer special comments.
//
// For all the WantedFuzzerFrom* functions, if the parameter does not
// contain a fuzzer special comment, the default wanted fuzzer is
// returned and the error is non-nil.
//
// See the README for a comprehensive explanation of the special
// comments.

package parser

import (
	"errors"
	"fmt"
	"go/ast"
	"strings"
	"unicode"
)

// WantedFuzzer is a description of a fuzzer we want to generate.
type WantedFuzzer struct {
	// The name of the interface.
	InterfaceName string

	// The function to produce a reference implementation.
	Reference Function

	// If true, the reference function returns a value rather than a
	// pointer.
	ReturnsValue bool

	// Comparison functions to use. The keys of this map are
	// ToString'd Types.
	Comparison map[string]EitherFunctionOrMethod

	// Generator functions The keys of this map are ToString'd Types.
	Generator map[string]Generator

	// Initial state for custom generator functions.
	GeneratorState string
}

// Generator is the name of a function to generate a value of a given
// type.
type Generator struct {
	// True if this is stateful.
	IsStateful bool

	// The function itself.
	Name string
}

// EitherFunctionOrMethod is either a function or a method. Param and
// receiver types are all the same.
type EitherFunctionOrMethod struct {
	// True if this is function, rather than a method.
	IsFunction bool

	// The function itself.
	Name string

	// The type of the method receiver / function parameters.
	Type Type

	// List of return types. Only meaningful in "@before compare".
	Returns []Type
}

// WantedFuzzersFromAST extracts all wanted fuzzers from comments in
// the AST of a file.
func WantedFuzzersFromAST(theAST *ast.File) (wanteds []WantedFuzzer, errs []error) {
	if theAST == nil {
		return nil, nil
	}

	if theAST.Doc != nil {
		wanted, err, logErr := WantedFuzzerFromCommentGroup(theAST.Doc)
		if err == nil {
			wanteds = append(wanteds, wanted)
		} else if logErr {
			errs = append(errs, err)
		}
	}

	for _, group := range theAST.Comments {
		wanted, err, logErr := WantedFuzzerFromCommentGroup(group)
		if err == nil {
			wanteds = append(wanteds, wanted)
		} else if logErr {
			errs = append(errs, err)
		}
	}

	return wanteds, errs
}

// WantedFuzzerFromCommentGroup tries to extract a wanted fuzzer
// description from a comment group. It is assumed that only one
// wanted fuzzer may occur in a comment group, and the first special
// comment must be the "@fuzz interface:" line; special comments in a
// group before this are ignored.
func WantedFuzzerFromCommentGroup(group *ast.CommentGroup) (WantedFuzzer, error, bool) {
	if group == nil {
		return WantedFuzzer{}, errors.New("CommentGroup is nil."), false
	}

	comments := group.List

	fuzzer := WantedFuzzer{
		InterfaceName:  "",
		Reference:      Function{},
		ReturnsValue:   false,
		Comparison:     make(map[string]EitherFunctionOrMethod),
		Generator:      make(map[string]Generator),
		GeneratorState: "",
	}
	fuzzing := false

	for _, comment := range comments {
		lines := splitLines(comment.Text)
		for _, line := range lines {
			line = strings.TrimSpace(line)

			// Either we have found the start of a fuzzer description
			// or not; and if not all we do is look for it.
			if !fuzzing {
				// "@fuzz interface:"
				suff, ok := matchPrefix(line, "@fuzz interface:")
				if ok {
					iface, err := parseFuzzInterface(suff)
					if err != nil {
						return WantedFuzzer{}, err, true
					}
					fuzzer.InterfaceName = iface
					fuzzing = true
				}
			} else {
				// "@known correct:"
				suff, ok := matchPrefix(line, "@known correct:")
				if ok {
					fundecl, returnsValue, err := parseKnownCorrect(suff)
					if err != nil {
						return WantedFuzzer{}, err, true
					}
					retty := BasicType(fuzzer.InterfaceName)
					fundecl.Returns = []Type{&retty}
					fuzzer.Reference = fundecl
					fuzzer.ReturnsValue = returnsValue
					continue
				}

				// "@comparison:"
				suff, ok = matchPrefix(line, "@comparison:")
				if ok {
					tyname, fundecl, err := parseComparison(suff)
					if err != nil {
						return WantedFuzzer{}, err, true
					}
					fuzzer.Comparison[tyname.ToString()] = fundecl
					continue
				}

				// "@generator:"
				suff, ok = matchPrefix(line, "@generator:")
				if ok {
					tyname, genfunc, stateful, err := parseGenerator(suff)
					if err != nil {
						return WantedFuzzer{}, err, true
					}
					fuzzer.Generator[tyname.ToString()] = Generator{IsStateful: stateful, Name: genfunc}
					continue
				}

				// "@generator state:"
				suff, ok = matchPrefix(line, "@generator state:")
				if ok {
					state, err := parseGeneratorState(suff)
					if err != nil {
						return WantedFuzzer{}, err, true
					}
					fuzzer.GeneratorState = state
					continue
				}
			}
		}
	}

	if fuzzing {
		return fuzzer, nil, false
	}
	return WantedFuzzer{}, errors.New("No fuzzer found in group."), false
}

// Parse a "@fuzz interface:"
//
// SYNTAX: Name
func parseFuzzInterface(line string) (string, error) {
	var (
		name string
		err  error
		rest string
	)

	name, rest = parseName(line)

	if name == "" {
		err = fmt.Errorf("Expected a name in '%s'", line)
	} else if rest != "" {
		err = fmt.Errorf("Unexpected left over input in '%s' (got '%s')", line, rest)
	}

	return name, err
}

// Parse a "@known correct:"
//
// SYNTAX: [&] FunctionName [ArgType1 ... ArgTypeN]
func parseKnownCorrect(line string) (Function, bool, error) {
	var (
		function     Function
		returnsValue bool
		rest         string
		err          error
	)

	if len(line) == 0 {
		return Function{}, false, errors.New("@known correct has empty argument")
	}

	// [&]
	if line[0] == '&' {
		line = strings.TrimLeftFunc(line[1:], unicode.IsSpace)
		returnsValue = true
	}

	// FunctionName
	if len(line) == 0 {
		return Function{}, false, errors.New("@known correct must have a function name")
	}

	function.Name, rest = parseName(line)

	// [ArgType1 ... ArgTypeN]
	var args []Type
	for rest != "" {
		var argty Type
		argty, rest, err = parseType(rest)

		if err != nil {
			return Function{}, false, err
		}

		args = append(args, argty)
	}
	function.Parameters = args

	return function, returnsValue, nil
}

// Parse a "@comparison:"
//
// SYNTAX: (Type:FunctionName | FunctionName Type)
func parseComparison(line string) (Type, EitherFunctionOrMethod, error) {
	var (
		ty         Type
		funcOrMeth EitherFunctionOrMethod
		err        error
		rest       string
	)

	funcOrMeth, rest, err = parseFunctionOrMethod(line)

	if err == nil && rest == "" {
		ty = funcOrMeth.Type
	} else if rest != "" {
		err = fmt.Errorf("Unexpected left over input in '%s' (got '%s')", line, rest)
	}

	return ty, funcOrMeth, err
}

// Parse a "@generator:"
//
// SYNTAX: [!] FunctionName Type
func parseGenerator(line string) (Type, string, bool, error) {
	var (
		ty       Type
		name     string
		stateful bool
		err      error
		rest     string
	)

	// [!]
	if line[0] == '!' {
		line = strings.TrimLeftFunc(line[1:], unicode.IsSpace)
		stateful = true
	}

	name, rest = parseName(line)

	if name == "" {
		err = fmt.Errorf("Expected a name in '%s'", line)
	} else {
		ty, rest, err = parseType(rest)

		if rest != "" {
			err = fmt.Errorf("Unexpected left over input in '%s' (got '%s')", line, rest)
		}
	}

	return ty, name, stateful, err
}

// Parse a "@generator state:"
//
// This does absolutely NO checking whatsoever beyond presence
// checking!
//
// SYNTAX: Expression
func parseGeneratorState(line string) (string, error) {
	if line == "" {
		return "", fmt.Errorf("Expected an initial state")
	}

	return line, nil
}

// Parse a function or a method, returning the remainder of the
// string, which has leading spaces stripped.
//
// SYNTAX: (Type:FunctionName | FunctionName Type)
func parseFunctionOrMethod(line string) (EitherFunctionOrMethod, string, error) {
	var (
		funcOrMeth EitherFunctionOrMethod
		rest       string
		err        error
	)

	// This is a bit tricky, as there is overlap between names and
	// types. Try parsing as both a name and a type: if the type
	// succeeds, assume it's a method and go with that; if not and the
	// name succeeds assume it's a function; and if neither succeed
	// give an error.

	tyType, tyRest, tyErr := parseType(line)
	nName, nRest := parseName(line)

	if tyErr == nil && tyRest[0] == ':' {
		// It's a method.
		funcOrMeth.Type = tyType
		funcOrMeth.Name, rest = parseName(tyRest[1:])
	} else if nName != "" {
		// It's a function
		funcOrMeth.Name = nName
		funcOrMeth.Type, rest, err = parseType(nRest)
		funcOrMeth.IsFunction = true
	} else {
		err = fmt.Errorf("'%s' does not appear to be a method or function", line)
	}

	return funcOrMeth, rest, err
}

// Parse a type. This is very stupid and doesn't make much effort to
// be absolutely correct.
//
// SYNTAX: []Type | chan Type | map[Type]Type | *Type | (Type) | Name.Type | Name
func parseType(s string) (Type, string, error) {
	// Array type
	suff, ok := matchPrefix(s, "[]")
	if ok {
		tycon := func(t Type) Type {
			ty := ArrayType{ElementType: t}
			return &ty
		}
		return parseUnaryType(tycon, suff, s)
	}

	// Chan type
	suff, ok = matchPrefix(s, "chan")
	if ok {
		tycon := func(t Type) Type {
			ty := ChanType{ElementType: t}
			return &ty
		}
		return parseUnaryType(tycon, suff, s)
	}

	// Map type
	suff, ok = matchPrefix(s, "map[")
	if ok {
		keyTy, keyRest, keyErr := parseType(suff)
		suff, ok = matchPrefix(keyRest, "]")
		if ok && keyErr == nil {
			tycon := func(t Type) Type {
				ty := MapType{KeyType: keyTy, ValueType: t}
				return &ty
			}
			return parseUnaryType(tycon, keyRest[1:], s)
		}
	}

	// Pointer type
	suff, ok = matchPrefix(s, "*")
	if ok {
		tycon := func(t Type) Type {
			ty := PointerType{TargetType: t}
			return &ty
		}
		return parseUnaryType(tycon, suff, s)
	}

	// Type in (posibly 0) parentheses
	noParens, parenOk := matchDelims(s, "(", ")")
	if parenOk {
		// Basic type OR qualified type
		if noParens == s {
			pref, suff := parseName(s)
			suff = strings.TrimLeftFunc(suff, unicode.IsSpace)

			if len(suff) > 0 && suff[0] == '.' {
				pkg := pref
				tyname, suff2 := parseName(suff[1:])
				ty := BasicType(tyname)
				rest := strings.TrimLeftFunc(suff2, unicode.IsSpace)
				qty := QualifiedType{Package: pkg, Type: &ty}
				return &qty, rest, nil
			} else {
				basicTy := BasicType(pref)
				rest := suff
				return &basicTy, rest, nil
			}
		}

		return parseType(noParens)
	}

	return nil, s, fmt.Errorf("Mismatched parentheses in '%s'", s)
}

// Helper function for parsing a unary type operator: [], chan, or *.
func parseUnaryType(tycon func(Type) Type, s, orig string) (Type, string, error) {
	var (
		innerTy Type
		rest    string
		err     error
	)

	noSpaces := strings.TrimLeft(s, " ")
	noParens, parenOk := matchDelims(noSpaces, "(", ")")

	if parenOk {
		innerTy, rest, err = parseType(noParens)
	} else {
		err = fmt.Errorf("Mismatched parentheses in '%s'", orig)
	}

	return tycon(innerTy), rest, err
}

// Parse a name.
//
// SYNTAX: [a-zA-Z0-9_-]
func parseName(s string) (string, string) {
	name, suff := takeWhileIn(s, "qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM1234567890_-")
	rest := strings.TrimLeftFunc(suff, unicode.IsSpace)
	return name, rest
}

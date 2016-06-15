package main

import (
	"errors"
	"fmt"
	"go/ast"
	"strconv"
	"strings"
	"unicode"

	goimports "golang.org/x/tools/imports"
)

// Options for the code generator
type CodeGenOptions struct {
	// Generate a complete source file, with package name and imports.
	Complete bool

	// The filename to use when automatically resolving
	// imports. If unset by the command-line arguments, defaults
	// to the filename of the source file.
	Filename string

	// The package to use in the generated output. If unset by the
	// command-line arguments, defaults to the package of the
	// source file.
	PackageName string

	// Avoid generating the FuzzTest...(..., *testing.T) function.
	NoTestCase bool

	// Avoid generating the Fuzz...(..., *rand.Rand, uint)
	// function. This implies NoTestCase.
	NoDefaultFuzz bool
}

// Fuzzer is a pair of an interface declaration and a description of
// how to generate the fuzzer.
type Fuzzer struct {
	Interface Interface
	Wanted    WantedFuzzer
}

/// ENTRY POINT

// Generate code for the fuzzers.
func CodeGen(options CodeGenOptions, imports []*ast.ImportSpec, fuzzers []Fuzzer) (string, []error) {
	var code string
	var errs []error

	if options.Complete {
		code = GeneratePreamble(options.PackageName, imports)
	}

	codeGenErr := func(fuzzer Fuzzer, err error) error {
		return fmt.Errorf("error occurred whilst generating code for '%s': %s.", fuzzer.Interface.Name, err)
	}

	for _, fuzzer := range fuzzers {
		code = code + "// " + fuzzer.Interface.Name + "\n\n"

		// FuzzTest...(... *testing.T)
		if !(options.NoTestCase || options.NoDefaultFuzz) {
			generated, err := CodegenTestCase(fuzzer)
			if err != nil {
				errs = append(errs, codeGenErr(fuzzer, err))
				continue
			}
			code = code + generated + "\n\n"
		}

		// Fuzz...(... *rand.Rand, uint)
		if !options.NoDefaultFuzz {
			generated, err := CodegenWithDefaultReference(fuzzer)
			if err != nil {
				errs = append(errs, codeGenErr(fuzzer, err))
				continue
			}
			code = code + generated + "\n\n"
		}

		generated, err := CodegenWithReference(fuzzer)
		if err != nil {
			errs = append(errs, codeGenErr(fuzzer, err))
			continue
		}
		code = code + generated + "\n\n"
	}

	code, err := FixImports(options, code)
	if err != nil {
		errs = append(errs, err)
	}

	return code, errs
}

// GeneratePreamble generates the header for a complete source file:
// the package name and the imports. These imports may be both
// overzealous (all imports from the source file are copied across)
// and incomplete (imports the generated functions pull in aren't
// added), so the FixImports function must be called after the full
// code has been generated to fix this up.
func GeneratePreamble(packagename string, imports []*ast.ImportSpec) string {
	preamble := "package " + packagename + "\n\n"

	for _, iport := range imports {
		preamble = preamble + GenerateImport(iport) + "\n"
	}

	return preamble + "\n"
}

// GenerateImport generates an import statement from an
// *ast.ImportSpec.
func GenerateImport(iport *ast.ImportSpec) string {
	if iport.Name != nil {
		return "import " + iport.Name.Name + " " + iport.Path.Value
	}
	return "import " + iport.Path.Value
}

// FixImports adds and removes imports with 'goimports'.
func FixImports(options CodeGenOptions, code string) (string, error) {
	if options.Complete {
		cbytes, err := goimports.Process(options.Filename, []byte(code), nil)
		return string(cbytes), err
	}
	return code, nil
}

/// MAIN CODE GENERATORS

// CodegenTestCase generates a function which can be used as a test
// case when given an implementation to test.
//
// For an interface named `Store` with a generating function that
// takes a single `int`, the generated function signature looks like
// this:
//
// ~~~go
// FuzzTestStore(makeTest (func(int) Store), t *testing.T)
// ~~~
//
// This test case will call `FuzzStore` (see
// CodegenWithDefaultReference) with a max number of 100 operations.
func CodegenTestCase(fuzzer Fuzzer) (string, error) {
	// Format parameters:
	//
	// - interface name
	// - generator arguments (comma separated)
	template := `func FuzzTest%[1]s(makeTest (func(%[2]s) %[1]s), t *testing.T) {
	rand := rand.New(rand.NewSource(0))

	err := Fuzz%[1]s(makeTest, rand, 100)

	if err != nil {
		t.Error(err)
	}
}`

	return fmt.Sprintf(template, fuzzer.Interface.Name, GeneratorArgs(fuzzer)), nil
}

// CodegenWithDefaultReference generates a function which will compare
// a supplied implementation of the interface against the reference,
// by performing a sequence of random operations.
//
// For an interface named `Store` with a generating function that
// takes a single `int`, the generated function signature looks like
// this:
//
// ~~~go
// FuzzStore(makeTest (func(int) Store), rand *rand.Rand, maxops uint) error
// ~~~
//
// This function will call `FuzzStoreWith` (see CodegenWithReference)
// with the default reference.
func CodegenWithDefaultReference(fuzzer Fuzzer) (string, error) {
	// Format parameters:
	//
	// - interface name
	// - generator arguments (comma separated)
	// - reference generator name
	// - "&" if the reference generator needs that
	// - code to declare arguments, set random values, and call functions
	template := `func Fuzz%[1]s(makeTest (func (%[2]s) %[1]s), rand *rand.Rand, max uint) error {
%[5]s

	return Fuzz%[1]sWith(%[4]sexpected%[1]s, actual%[1]s, rand, max)
}`

	var ampersand string
	if fuzzer.Wanted.ReturnsValue {
		ampersand = "&"
	}

	funcalls, err := MakeFunctionCalls(fuzzer, fuzzer.Wanted.Reference, fuzzer.Wanted.Reference.Name, "makeTest")

	if err != nil {
		return "", err
	}

	body := fmt.Sprintf(
		template,
		fuzzer.Interface.Name,
		GeneratorArgs(fuzzer),
		fuzzer.Wanted.Reference.Name,
		ampersand,
		indentLines(funcalls, "\t"))

	return body, nil
}

// CodegenWithReference generates a function which will compare two
// arbitrary implementations of the supplied interface, by performing
// a sequence of random operations.
//
// For an interface named `Store`, the generated function signature
// looks like this:
//
// ~~~go
// FuzzStoreWith(reference Store, test Store, rand *rand.Rand, maxops uint) error
// ~~~
//
// In any found discrepancies, the return value from the reference
// `Store` (the first parameter) will be displayed as the "expected"
// output, and the other as the "actual".
func CodegenWithReference(fuzzer Fuzzer) (string, error) {
	// Format parameters:
	//
	// - interface name
	// - number of methods in interface
	// - code to perform methods
	// - code to create initial state
	template := `func Fuzz%[1]sWith(reference %[1]s, test %[1]s, rand *rand.Rand, maxops uint) error {%[4]s

	for i = 0; i < maxops; i++ {
		// Pick a random number between 0 and the number of methods of the interface. Then do that method on
		// both, check for discrepancy, and bail out on error. Simple!

		actionToPerform := rand.Intn(%[2]v)

		switch actionToPerform {
%[3]s
		}
	}

	return nil
}`

	var actions []string
	for i, function := range fuzzer.Interface.Functions {
		caseTemplate := "case %[1]v:\n%s"
		action, err := CodegenFunctionTest(fuzzer, function)
		if err != nil {
			return "", err
		}
		actions = append(actions, fmt.Sprintf(caseTemplate, i, indentLines(action, "\t")))
	}

	var initialState string
	if fuzzer.Wanted.GeneratorState != "" {
		initialState = fmt.Sprintf("\n\n\t// Create initial state\n\tstate := %s", fuzzer.Wanted.GeneratorState)
	}

	body := fmt.Sprintf(
		template,
		fuzzer.Interface.Name,
		len(fuzzer.Interface.Functions),
		indentLines(strings.Join(actions, "\n"), "\t\t"),
		initialState)

	return body, nil
}

// codegenFunctionTest generate the code to declare and initialise
// variables, call the method on both implementations, compare the
// results, and bail out on error.
func CodegenFunctionTest(fuzzer Fuzzer, function Function) (string, error) {
	// Format parameters:
	//
	// - code to declare variables + call functions (etc)
	// - code to check for discrepancies
	template := `// Call the method on both implementations
%[1]s

// And check for discrepancies.
%[2]s`

	funcalls, err := MakeFunctionCalls(fuzzer, function, "reference."+function.Name, "test."+function.Name)
	if err != nil {
		return "", err
	}

	var checks []string
	retsExpected := FuncExpectedNames(function)
	retsActual := FuncActualNames(function)
	for i, ty := range function.Returns {
		expected := retsExpected[i]
		actual := retsActual[i]
		check, err := MakeValueComparison(fuzzer, expected, actual, ty, "inconsistent result in "+function.Name)
		if err != nil {
			return "", err
		}
		checks = append(checks, check)
	}

	return fmt.Sprintf(template, funcalls, strings.Join(checks, "\n")), nil
}

/// FUNCTION CALLS

// Produce the generator arguments as a comma-separated list.
func GeneratorArgs(fuzzer Fuzzer) string {
	var args []string

	for _, ty := range fuzzer.Wanted.Reference.Parameters {
		args = append(args, ty.ToString())
	}

	return strings.Join(args, ", ")
}

// Generate a call to two functions with the same signature, with
// random argument values.
//
// Arguments are stored in variables arg0 ... argN. Return values in
// variables reta0 ... retaN and retb0 ... retbN.
func MakeFunctionCalls(fuzzer Fuzzer, function Function, funcA, funcB string) (string, error) {
	// Format parameters:
	//
	// - first function name
	// - second function name
	// - code to declare random arguments inside a var block.
	// - code to produce random argument values
	// - argument variable names (comma separated)
	// - return variable names for first function (comma separated)
	// - return variable names for second function (comma separated)
	template := `var (
	%[3]s
)

%[4]s

%[6]s := %[1]s(%[5]s)
%[7]s := %[2]s(%[5]s)`

	// Format parameters:
	//
	// - first function name
	// - second function name
	// - return variable names for first function (comma separated)
	// - return variable names for second function (comma separated)
	templateNoArgs := "%[3]s := %[1]s()\n%[4]s := %[2]s()"

	var (
		decls []string
		args  []string
		gens  []string
	)

	argNames := FuncArgNames(function)
	for i, ty := range function.Parameters {
		arg := argNames[i]

		decls = append(decls, arg+" "+ty.ToString())
		args = append(args, arg)
		gen, err := MakeTypeGenerator(fuzzer, arg, ty)
		if err != nil {
			return "", err
		}
		gens = append(gens, gen)
	}

	retsExpected := FuncExpectedNames(function)
	retsActual := FuncActualNames(function)

	body := fmt.Sprintf(
		template,
		funcA,
		funcB,
		strings.Join(decls, "\n\t"),
		strings.Join(gens, "\n"),
		strings.Join(args, ", "),
		strings.Join(retsExpected, ", "),
		strings.Join(retsActual, ", "))

	// Slightly nicer output if there are no arguments
	if len(args) == 0 {
		body = fmt.Sprintf(
			templateNoArgs,
			funcA,
			funcB,
			strings.Join(retsExpected, ", "),
			strings.Join(retsActual, ", "))
	}

	return body, nil
}

/// VALUE INITIALISATION

// Produce some code to populate a given variable with a random value
// of the named type, assuming a PRNG called 'rand' is in scope.
func MakeTypeGenerator(fuzzer Fuzzer, varname string, ty Type) (string, error) {
	tyname := ty.ToString()

	// If there's a provided generator, use that.
	generator, ok := fuzzer.Wanted.Generator[tyname]
	if ok {
		if generator.IsStateful {
			if fuzzer.Wanted.GeneratorState == "" {
				return "", errors.New("stateful generator used when no initial state given")
			}
			return fmt.Sprintf("%s, state = %s(rand, state)", varname, generator.Name), nil
		}
		return fmt.Sprintf("%s = %s(rand)", varname, generator.Name), nil
	}

	// If it's a type we can handle, supply a default generator.
	var tygen string
	tygen, ok = DefaultGenerator(tyname)
	if ok {
		return fmt.Sprintf("%s = %s", varname, tygen), nil
	}

	// Otherwise cry because generic programming in Go is hard :(
	return "", fmt.Errorf("I don't know how to generate a %s", tyname)
}

// Default generators for builtin types.
func DefaultGenerator(tyname string) (string, bool) {
	generators := map[string]string{
		"bool":       "rand.Intn(2) == 0",
		"byte":       "byte(rand.Uint32())",
		"complex64":  "complex(float32(rand.NormFloat64()), float32(rand.NormFloat64()))",
		"complex128": "complex(rand.NormFloat64(), rand.NormFloat64())",
		"float32":    "float32(rand.NormFloat64())",
		"float64":    "rand.NormFloat64()",
		"int":        "rand.Int()",
		"int8":       "int8(rand.Int())",
		"int16":      "int16(rand.Int())",
		"int32":      "rand.Int31()",
		"int64":      "rand.Int63()",
		"rune":       "rune(rand.Int31())",
		"uint":       "uint(rand.Uint32())",
		"uint8":      "uint8(rand.Uint32())",
		"uint16":     "uint16(rand.Uint32())",
		"uint32":     "rand.Uint32()",
		"uint64":     "(uint64(rand.Uint32()) << 32) | uint64(rand.Uint32())",
	}

	tygen, ok := generators[tyname]
	return tygen, ok
}

/// VALUE COMPARISON

// Produce some code to compare two values of the same type, returning
// an error on discrepancy.
func MakeValueComparison(fuzzer Fuzzer, expectedvar string, actualvar string, ty Type, errmsg string) (string, error) {
	// Format parameters:
	//
	// - expected variable name
	// - actual variable name
	// - comparison expression
	// - error message
	template := `if !%[3]s {
	return fmt.Errorf("%[4]s\nexpected: %%v\nactual:   %%v", %[1]s, %[2]s)
}`

	tyname := ty.ToString()
	comparison := fmt.Sprintf(DefaultComparison(tyname), expectedvar, actualvar)

	// If there's a provided comparison, use that.
	tycomp, ok := fuzzer.Wanted.Comparison[tyname]
	if ok {
		comparison = fmt.Sprintf("%s.%s(%s)", expectedvar, tycomp.Name, actualvar)

		if tycomp.IsFunction {
			comparison = fmt.Sprintf("%s(%s, %s)", tycomp.Name, expectedvar, actualvar)
		}
	}

	return fmt.Sprintf(template, expectedvar, actualvar, comparison, errmsg), nil
}

// Default comparisons for builtin types.
func DefaultComparison(tyname string) string {
	if tyname == "error" {
		// Special case for errors: just compare nilness.
		return "((%s == nil) == (%s == nil))"
	}

	// For everything else, use reflect.DeepEqual
	return "reflect.DeepEqual(%s, %s)"
}

/// TYPE-DIRECTED VARIABLE NAMING

// Produce unique variable names for function arguments. These do not
// clash with names produced by funcExpectedNames or funcActualNames.
func FuncArgNames(function Function) []string {
	return TypeListNames("arg", function.Parameters)
}

// Produce unique variable names for actual function returns. These do
// not clash with names produced by funcArgNames or funcExpectedNames.
func FuncActualNames(function Function) []string {
	return TypeListNames("actual", function.Returns)
}

// Produce unique variable names for expected function returns. These
// do not clash with names produced by funcArgNames or
// funcActualNames.
func FuncExpectedNames(function Function) []string {
	return TypeListNames("expected", function.Returns)
}

// Produce names for variables given a list of types.
func TypeListNames(prefix string, tylist []Type) []string {
	var names []string

	for i, ty := range tylist {
		// Generate a name for this variable based on the type.
		name := TypeNameToVarName(prefix, ty)
		for _, prior := range names {
			if name == prior {
				name = name + strconv.Itoa(i)
				break
			}
		}
		names = append(names, name)
	}

	return names
}

// Produce a (possibly not unique) variable name from a type name.
func TypeNameToVarName(pref string, ty Type) string {
	name := filter(ty.ToString(), unicode.IsLetter)

	// More pleasing capitalisation.
	for i, r := range name {
		name = string(unicode.ToUpper(r)) + name[i+1:]
		break
	}

	return pref + name
}

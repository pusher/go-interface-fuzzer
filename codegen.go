package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"strconv"
	"strings"
	"text/template"
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
	Name    string
	Methods []Function
	Wanted  WantedFuzzer
}

var (
	// Default generators for builtin types. If there is no entry
	// for the desired type, an error is signalled.
	defaultGenerators = map[string]string{
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

	// Default comparisons for builtin types. If there is no entry
	// for the desired type, 'fallbackComparison' is used.
	defaultComparisons = map[string]string{
		"error": "((%s == nil) == (%s == nil))",
	}

	// Fallback comparison if there is nothing in 'defaultComparisons'.
	fallbackComparison = "reflect.DeepEqual(%s, %s)"
)

// All of the templates take a Fuzzer as the argument.
const (
	// Template used by CodegenTestCase.
	testCaseTemplate = `
{{$name := .Name}}
{{$args := argV .Wanted.Reference.Parameters}}

func FuzzTest{{$name}}(makeTest func({{$args}}) {{$name}}, t *testing.T) {
	rand := rand.New(rand.NewSource(0))

	err := Fuzz{{$name}}(makeTest, rand, 100)

	if err != nil {
		t.Error(err)
	}
}`

	// Template used by CodegenWithDefaultReference
	withDefaultReferenceTemplate = `
{{$name  := .Name}}
{{$args  := argV .Wanted.Reference.Parameters}}
{{$decls := makeFunCalls . .Wanted.Reference .Wanted.Reference.Name "makeTest"}}
{{$and   := eitherOr .Wanted.ReturnsValue "&" ""}}

func Fuzz{{$name}}(makeTest func ({{$args}}) {{$name}}, rand *rand.Rand, max uint) error {
{{indent $decls "\t"}}

	return Fuzz{{$name}}With({{$and}}expected{{$name}}, actual{{$name}}, rand, max)
}`

	// Template used by CodegenWithReference
	withReferenceTemplate = `
{{$fuzzer := .}}
{{$name   := .Name}}
{{$count  := len .Methods}}
{{$state  := .Wanted.GeneratorState}}

func Fuzz{{$name}}With(reference {{$name}}, test {{$name}}, rand *rand.Rand, maxops uint) error {
{{if $state | eq ""}}{{else}}	// Create initial state
	state := {{$state}}

{{end}}	for i := uint(0); i < maxops; i++ {
		// Pick a random number between 0 and the number of methods of the interface. Then do that method on
		// both, check for discrepancy, and bail out on error. Simple!

		actionToPerform := rand.Intn({{$count}})

		switch actionToPerform { {{range $i, $function := .Methods}}
		case {{$i}}:
			// Call the method on both implementations
{{indent (makeFunCalls $fuzzer $function (printf "reference.%s" $function.Name) (printf "test.%s" $function.Name)) "\t\t\t"}}

			// And check for discrepancies.{{range $j, $ty := $function.Returns}}{{$expected := expected $function $j}}{{$actual   := actual $function $j}}
			if !{{printf (comparison $fuzzer $ty) $expected $actual}} {
				return fmt.Errorf("inconsistent result in {{$function.Name}}\nexpected: %v\nactual:   %v", {{$expected}}, {{$actual}})
			}{{end}}{{end}}
		} {{range $i, $invariant := .Wanted.Invariants}}

		if !({{sed $invariant "%var" "reference"}}) {
			return errors.New("invariant violated: {{$invariant}}")
		}
{{end}}
	}

	return nil
}`

	// Template used by MakeFunctionCalls.
	functionCallTemplate = `
{{$fuzzer       := . }}
{{$function     := function ""}}
{{$expecteds    := expecteds $function}}
{{$actuals      := actuals $function}}
{{$arguments    := arguments $function}}
{{$expectedFunc := expectedFunc ""}}
{{$actualFunc   := actualFunc ""}}

{{if len $arguments | ne 0}}
var ({{range $i, $ty := $function.Parameters}}
	{{argument $function $i}} {{toString $ty}}{{end}}
)
{{range $i, $ty := $function.Parameters}}
{{makeTyGen $fuzzer (argument $function $i) $ty}}{{end}}{{end}}

{{if len $expecteds | eq 0}}
{{$expectedFunc}}({{varV $arguments}})
{{$actualFunc}}({{varV $arguments}})
{{else}}
{{varV $expecteds}} := {{$expectedFunc}}({{varV $arguments}})
{{varV $actuals}} := {{$actualFunc}}({{varV $arguments}})
{{end}}`
)

/// ENTRY POINT

// Generate code for the fuzzers.
func CodeGen(options CodeGenOptions, imports []*ast.ImportSpec, fuzzers []Fuzzer) (string, []error) {
	var code string
	var errs []error

	if options.Complete {
		code = generatePreamble(options.PackageName, imports)
	}

	codeGenErr := func(fuzzer Fuzzer, err error) error {
		return fmt.Errorf("error occurred whilst generating code for '%s': %s", fuzzer.Name, err)
	}

	for _, fuzzer := range fuzzers {
		code = code + "// " + fuzzer.Name + "\n\n"

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

	code, err := fixImports(options, code)
	if err != nil {
		errs = append(errs, err)
	}

	return code, errs
}

// Generates the header for a complete source file: the package name
// and the imports. These imports may be both overzealous (all imports
// from the source file are copied across) and incomplete (imports the
// generated functions pull in aren't added), so the FixImports
// function must be called after the full code has been generated to
// fix this up.
func generatePreamble(packagename string, imports []*ast.ImportSpec) string {
	preamble := "package " + packagename + "\n\n"

	for _, iport := range imports {
		preamble = preamble + generateImport(iport) + "\n"
	}

	return preamble + "\n"
}

// Generates an import statement from an *ast.ImportSpec.
func generateImport(iport *ast.ImportSpec) string {
	if iport.Name != nil {
		return "import " + iport.Name.Name + " " + iport.Path.Value
	}
	return "import " + iport.Path.Value
}

// Adds and removes imports with 'goimports'.
func fixImports(options CodeGenOptions, code string) (string, error) {
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
	return runTemplate("testCase", testCaseTemplate, fuzzer)
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
	return runTemplate("withDefaultReference", withDefaultReferenceTemplate, fuzzer)
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
	return runTemplate("withReference", withReferenceTemplate, fuzzer)
}

/// FUNCTION CALLS

// Generate a call to two functions with the same signature, with
// random argument values.
//
// Arguments are stored in variables arg0 ... argN. Return values in
// variables reta0 ... retaN and retb0 ... retbN.
func makeFunctionCalls(fuzzer Fuzzer, function Function, funcA, funcB string) (string, error) {
	funcs := template.FuncMap{
		"function":     func(s string) Function { return function },
		"expectedFunc": func(s string) string { return funcA },
		"actualFunc":   func(s string) string { return funcB },
	}

	return runTemplateWith("functionCall", functionCallTemplate, fuzzer, funcs)
}

/// VALUE INITIALISATION

// Produce some code to populate a given variable with a random value
// of the named type, assuming a PRNG called 'rand' is in scope.
func makeTypeGenerator(fuzzer Fuzzer, varname string, ty Type) (string, error) {
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
	tygen, ok = defaultGenerators[tyname]
	if ok {
		return fmt.Sprintf("%s = %s", varname, tygen), nil
	}

	// Otherwise cry because generic programming in Go is hard :(
	return "", fmt.Errorf("I don't know how to generate a %s", tyname)
}

/// VALUE COMPARISON

// Produce a format string to compare two values of the same type.
// given the variable names.
func makeValueComparison(fuzzer Fuzzer, ty Type) string {
	tyname := ty.ToString()
	comparison, ok := defaultComparisons[tyname]
	if !ok {
		comparison = fallbackComparison
	}

	// If there's a provided comparison, use that.
	tycomp, ok := fuzzer.Wanted.Comparison[tyname]
	if ok {
		comparison = "%s." + tycomp.Name + "(%s)"

		if tycomp.IsFunction {
			comparison = tycomp.Name + "(%s, %s)"
		}
	}

	return comparison
}

/// TEMPLATES

// Run a template and return the output.
func runTemplate(tplName, tpl string, fuzzer Fuzzer) (string, error) {
	return runTemplateWith(tplName, tpl, fuzzer, nil)
}

// Run a template and return the output, overriding the built-in
// template functions with a custom map (which can also add new
// functions).
func runTemplateWith(tplName, tpl string, fuzzer Fuzzer, overrides template.FuncMap) (string, error) {
	funcMap := template.FuncMap{
		// Render a list of types
		"argV": func(types []Type) string {
			var args []string

			for _, ty := range types {
				args = append(args, ty.ToString())
			}

			return strings.Join(args, ", ")
		},
		// Render a list of variables
		"varV": func(vars []string) string {
			return strings.Join(vars, ", ")
		},
		// Select one of two values based on a flag
		"eitherOr": func(f bool, a, b string) string {
			if f {
				return a
			}
			return b
		},
		// Indent every line of a string
		"indent": indentLines,
		// Argument names
		"arguments": func(function Function) []string {
			return funcArgNames(function)
		},
		"argument": func(function Function, i int) (string, error) {
			return inSlice(funcArgNames(function), i, "argument")
		},
		// Expected value names
		"expecteds": func(function Function) []string { return funcExpectedNames(function) },
		"expected": func(function Function, i int) (string, error) {
			return inSlice(funcExpectedNames(function), i, "result")
		},
		// Actual value names
		"actuals": func(function Function) []string {
			return funcActualNames(function)
		},
		"actual": func(function Function, i int) (string, error) {
			return inSlice(funcActualNames(function), i, "result")
		},
		// Render a type as a string
		"toString": func(ty Type) string {
			return ty.ToString()
		},
		// Make a function call
		"makeFunCalls": makeFunctionCalls,
		// Make a value comparison
		"comparison": makeValueComparison,
		// Make a type generator
		"makeTyGen": makeTypeGenerator,
		// Replace one string with another
		"sed": func(s, old, new string) string {
			return strings.Replace(s, old, new, -1)
		},
	}

	for k, v := range overrides {
		funcMap[k] = v
	}

	var buf bytes.Buffer
	t, err := template.New(tplName).Funcs(funcMap).Parse(tpl)
	if err != nil {
		return "", err
	}
	err = t.Execute(&buf, fuzzer)
	return strings.TrimSpace(string(buf.Bytes())), err
}

// Safe slice lookup
func inSlice(ss []string, i int, name string) (string, error) {
	if i < 0 || i >= len(ss) {
		return "", errors.New(name + " index out of range")
	}

	return ss[i], nil
}

/// TYPE-DIRECTED VARIABLE NAMING

// Produce unique variable names for function arguments. These do not
// clash with names produced by funcExpectedNames or funcActualNames.
func funcArgNames(function Function) []string {
	return typeListNames("arg", function.Parameters)
}

// Produce unique variable names for actual function returns. These do
// not clash with names produced by funcArgNames or funcExpectedNames.
func funcActualNames(function Function) []string {
	return typeListNames("actual", function.Returns)
}

// Produce unique variable names for expected function returns. These
// do not clash with names produced by funcArgNames or
// funcActualNames.
func funcExpectedNames(function Function) []string {
	return typeListNames("expected", function.Returns)
}

// Produce names for variables given a list of types.
func typeListNames(prefix string, tylist []Type) []string {
	var names []string

	for i, ty := range tylist {
		// Generate a name for this variable based on the type.
		name := typeNameToVarName(prefix, ty)
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
func typeNameToVarName(pref string, ty Type) string {
	name := filter(ty.ToString(), unicode.IsLetter)

	// More pleasing capitalisation.
	for i, r := range name {
		name = string(unicode.ToUpper(r)) + name[i+1:]
		break
	}

	return pref + name
}

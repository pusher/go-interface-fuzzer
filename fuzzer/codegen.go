package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	fuzzparser "barrucadu/go-interface-fuzzer/parser"
)

// Fuzzer is a pair of an interface declaration and a description of
// how to generate the fuzzer.
type Fuzzer struct {
	Interface fuzzparser.Interface
	Wanted    fuzzparser.WantedFuzzer
}

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

	return fmt.Sprintf(template, fuzzer.Interface.Name, generatorArgs(fuzzer)), nil
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

	return Fuzz%[1]sWith(%[4]sreta0, retb0, rand, max)
}`

	var ampersand string
	if fuzzer.Wanted.ReturnsValue {
		ampersand = "&"
	}

	funcalls, err := makeFunctionCalls(fuzzer, fuzzer.Wanted.Reference, fuzzer.Wanted.Reference.Name, "makeTest")

	if err != nil {
		return "", err
	}

	body := fmt.Sprintf(
		template,
		fuzzer.Interface.Name,
		generatorArgs(fuzzer),
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
	template := `func Fuzz%[1]sWith(reference %[1]s, test %[1]s, rand *rand.Rand, maxops uint) error {
	actionsToPerform := maxops%[4]s

	for actionsToPerform > 0 {
		// Pick a random number between 0 and the number of methods of the interface. Then do that method on
		// both, check for discrepancy, and bail out on error. Simple!

		actionToPerform := rand.Intn(%[2]v)

		switch actionToPerform {
%[3]s
		}

		actionsToPerform --
	}

	return nil
}`

	var actions []string
	for i, function := range fuzzer.Interface.Functions {
		// Format parameters:
		//
		// - case number
		// - code to declare variables + call functions (etc)
		// - code to check for discrepancies
		template := `case %[1]v:
	// Call the method on both implementations
%[2]s

	// And check for discrepancies.
%[3]s`

		funcalls, err := makeFunctionCalls(fuzzer, function, "reference."+function.Name, "test."+function.Name)
		if err != nil {
			return "", err
		}

		var checks []string
		for i, ty := range function.Returns {
			expected := "reta" + strconv.Itoa(i)
			actual := "retb" + strconv.Itoa(i)
			check, err := makeValueComparison(fuzzer, expected, actual, ty, "Inconsistent result in "+function.Name)
			if err != nil {
				return "", err
			}
			checks = append(checks, check)
		}

		action := fmt.Sprintf(template, i, indentLines(funcalls, "\t"), indentLines(strings.Join(checks, "\n"), "\t"))
		actions = append(actions, action)
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

// Produce the generator arguments as a comma-separated list.
func generatorArgs(fuzzer Fuzzer) string {
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
func makeFunctionCalls(fuzzer Fuzzer, function fuzzparser.Function, funcA, funcB string) (string, error) {
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
		retsa []string
		retsb []string
	)

	for i, ty := range function.Parameters {
		arg := "arg" + strconv.Itoa(i)
		decls = append(decls, arg+" "+ty.ToString())
		args = append(args, arg)
		gen, err := makeTypeGenerator(fuzzer, arg, ty)
		if err != nil {
			return "", err
		}
		gens = append(gens, gen)
	}

	for i := range function.Returns {
		retsa = append(retsa, "reta"+strconv.Itoa(i))
		retsb = append(retsb, "retb"+strconv.Itoa(i))
	}

	body := fmt.Sprintf(
		template,
		funcA,
		funcB,
		strings.Join(decls, "\n\t"),
		strings.Join(gens, "\n"),
		strings.Join(args, ", "),
		strings.Join(retsa, ", "),
		strings.Join(retsb, ", "))

	// Slightly nicer output if there are no arguments
	if len(args) == 0 {
		body = fmt.Sprintf(
			templateNoArgs,
			funcA,
			funcB,
			strings.Join(retsa, ", "),
			strings.Join(retsb, ", "))
	}

	return body, nil
}

// Produce some code to populate a given variable with a random value
// of the named type, assuming a PRNG called 'rand' is in scope.
func makeTypeGenerator(fuzzer Fuzzer, varname string, ty fuzzparser.Type) (string, error) {
	tyname := ty.ToString()

	// If there's a provided generator, use that.
	generator, ok := fuzzer.Wanted.Generator[tyname]
	if ok {
		if generator.IsStateful {
			if fuzzer.Wanted.GeneratorState == "" {
				return "", errors.New("Stateful generator used when no initial state given.")
			}
			return fmt.Sprintf("%s, state = %s(rand, state)", varname, generator.Name), nil
		}
		return fmt.Sprintf("%s = %s(rand)", varname, generator.Name), nil
	}

	// If it's a type we can handle, supply a default generator.
	tygen := ""

	switch tyname {
	case "bool":
		tygen = "rand.Intn(2) == 0"
	case "byte":
		tygen = "byte(rand.Uint32())"
	case "complex64":
		tygen = "complex(float32(rand.NormFloat64()), float32(rand.NormFloat64()))"
	case "complex128":
		tygen = "complex(rand.NormFloat64(), rand.NormFloat64())"
	case "float32":
		tygen = "float32(rand.NormFloat64())"
	case "float64":
		tygen = "rand.NormFloat64()"
	case "int":
		tygen = "rand.Int()"
	case "int8":
		tygen = "int8(rand.Int())"
	case "int16":
		tygen = "int16(rand.Int())"
	case "int32":
		tygen = "rand.Int31()"
	case "int64":
		tygen = "rand.Int63()"
	case "rune":
		tygen = "rune(rand.Int31())"
	case "uint":
		tygen = "uint(rand.Uint32())"
	case "uint8":
		tygen = "uint8(rand.Uint32())"
	case "uint16":
		tygen = "uint16(rand.Uint32())"
	case "uint32":
		tygen = "rand.Uint32()"
	case "uint64":
		tygen = "(uint64(rand.Uint32()) << 32) | uint64(rand.Uint32())"
	}

	if tygen != "" {
		return fmt.Sprintf("%s = %s", varname, tygen), nil
	}

	// Otherwise cry because generic programming in Go is hard :(
	return "", fmt.Errorf("I don't know how to generate a %s", tyname)
}

// Produce some code to compare two values of the same type, returning
// an error on discrepancy.
func makeValueComparison(fuzzer Fuzzer, expectedvar string, actualvar string, ty fuzzparser.Type, errmsg string) (string, error) {
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
	comparison := fmt.Sprintf("reflect.DeepEqual(%s, %s)", expectedvar, actualvar)

	// If there's a provided comparison, use that.
	tycomp, ok := fuzzer.Wanted.Comparison[tyname]
	if ok {
		comparison = fmt.Sprintf("%s.%s(%s)", expectedvar, tycomp.Name, actualvar)

		if tycomp.IsFunction {
			comparison = fmt.Sprintf("%s(%s, %s)", tycomp.Name, expectedvar, actualvar)
		}
	} else if tyname == "error" {
		// Special case for errors: just compare nilness.
		comparison = fmt.Sprintf("((%s == nil) == (%s == nil))", expectedvar, actualvar)
	}

	return fmt.Sprintf(template, expectedvar, actualvar, comparison, errmsg), nil
}

// Indent every line by the given number of tabs.
func indentLines(s string, indent string) string {
	lines := strings.Split(s, "\n")
	return indent + strings.Join(lines, "\n"+indent)
}

package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/urfave/cli"
)

// Turn a collection of errors into a single error message with a list
// of errors.
func errorList(message string, errs []error) string {
	var errstrs []string
	for _, err := range errs {
		errstrs = append(errstrs, err.Error())
	}
	return (message + ":\n\t- " + strings.Join(errstrs, "\n\t- "))
}

// Reconcile the wanted fuzzers with the interfaces. Complain if there
// are any wanted fuzzers for which the interface decl isn't in the
// file.
func reconcileFuzzers(interfaces []Interface, wanteds []WantedFuzzer) ([]Fuzzer, []error) {
	var fuzzers []Fuzzer
	var errs []error

	for _, wanted := range wanteds {
		var found bool

		for _, iface := range interfaces {
			if wanted.InterfaceName != iface.Name {
				continue
			}

			fuzzer := Fuzzer{Interface: iface, Wanted: wanted}

			// Check we don't already have a fuzzer for
			// this interface.
			for _, existingFuzzer := range fuzzers {
				if existingFuzzer.Interface.Name == iface.Name {
					errs = append(errs, fmt.Errorf("Already have a fuzzer for '%s'.", wanted.InterfaceName))
				}
			}

			fuzzers = append(fuzzers, fuzzer)
			found = true
		}

		if !found {
			errs = append(errs, fmt.Errorf("Couldn't find interface '%s' in this file.", wanted.InterfaceName))
		}
	}

	return fuzzers, errs
}

// Generate code for the fuzzers.
func codeGen(fuzzers []Fuzzer) (string, []error) {
	var code string
	var errs []error

	codeGenErr := func(fuzzer Fuzzer, err error) error {
		return fmt.Errorf("error occurred whilst generating code for '%s': %s.", fuzzer.Interface.Name, err)
	}

	for _, fuzzer := range fuzzers {
		testCase, testCaseErr := CodegenTestCase(fuzzer)
		withDefaultReference, withDefaultReferenceErr := CodegenWithDefaultReference(fuzzer)
		withReference, withReferenceErr := CodegenWithReference(fuzzer)

		if testCaseErr != nil {
			errs = append(errs, codeGenErr(fuzzer, testCaseErr))
			continue
		}
		if withDefaultReferenceErr != nil {
			errs = append(errs, codeGenErr(fuzzer, withDefaultReferenceErr))
			continue
		}
		if withReferenceErr != nil {
			errs = append(errs, codeGenErr(fuzzer, withReferenceErr))
			continue
		}

		code = code + fmt.Sprintf("// %s\n\n%s\n\n%s\n\n%s\n", fuzzer.Interface.Name, testCase, withDefaultReference, withReference)
	}

	return code, errs
}

func main() {
	app := cli.NewApp()

	app.Name = "go-interface-fuzzer"
	app.Usage = "Generate fuzz tests for Go interfaces."
	app.Action = func(c *cli.Context) error {
		if len(c.Args()) < 1 {
			return cli.NewExitError("Must specify a file to generate a fuzzer from.", 1)
		}

		filename := c.Args().Get(0)
		fset := token.NewFileSet()
		parsedFile, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)

		if err != nil {
			return cli.NewExitError(fmt.Sprintf("Could not parse file: '%s'", err.Error()), 1)
		}

		if parsedFile == nil {
			// Shouldn't be reachable, as 'err' should be non-nil if
			// the ast is nil.
			return cli.NewExitError("Could not parse file.", 1)
		}

		// Extract all the interfaces
		interfaces := InterfacesFromAST(parsedFile)

		// Extract the wanted fuzzers
		wanteds, werrs := WantedFuzzersFromAST(parsedFile)
		if len(werrs) > 0 {
			return cli.NewExitError(errorList("Found errors while extracting interface definitions", werrs), 1)
		}

		// Reconcile the wanteds with the interfaces.
		fuzzers, ferrs := reconcileFuzzers(interfaces, wanteds)
		if len(ferrs) > 0 {
			return cli.NewExitError(errorList("Found errors while determining wanted fuzz testers", ferrs), 1)
		}

		// Codegen
		code, cerrs := codeGen(fuzzers)
		if len(cerrs) > 0 {
			return cli.NewExitError(errorList("Found some errors while generating code", cerrs), 1)
		}
		fmt.Println(code)

		return nil
	}

	app.Run(os.Args)
}

package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"

	"github.com/urfave/cli"

	fuzzparser "barrucadu/go-interface-fuzzer/parser"
)

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

		interfaces := fuzzparser.InterfacesFromAST(parsedFile)

		wanteds := fuzzparser.WantedFuzzersFromAST(parsedFile)

		// Reconcile the wanteds with the interfaces.
		var fuzzers []Fuzzer

		for _, wanted := range wanteds {
			var found bool

			for _, iface := range interfaces {
				if wanted.InterfaceName == iface.Name {
					fuzzer := Fuzzer{Interface: iface, Wanted: wanted}

					// Check we don't already have a fuzzer for this
					// interface.
					for _, existingFuzzer := range fuzzers {
						if existingFuzzer.Interface.Name == iface.Name {
							return cli.NewExitError(fmt.Sprintf("Already have a fuzzer for '%s'.", wanted.InterfaceName), 1)
						}
					}

					fuzzers = append(fuzzers, fuzzer)
					found = true
				}
			}

			if !found {
				return cli.NewExitError(fmt.Sprintf("Couldn't find interface '%s' in this file.", wanted.InterfaceName), 1)
			}
		}

		// Codegen
		codeGenErr := func(fuzzer Fuzzer, err error) *cli.ExitError {
			msg := fmt.Sprintf("Error occurred whilst generating code for '%s': %s.", fuzzer.Interface.Name, err)
			return cli.NewExitError(msg, 1)
		}

		for _, fuzzer := range fuzzers {
			testCase, testCaseErr := CodegenTestCase(fuzzer)
			withDefaultReference, withDefaultReferenceErr := CodegenWithDefaultReference(fuzzer)
			withReference, withReferenceErr := CodegenWithReference(fuzzer)

			if testCaseErr != nil {
				return codeGenErr(fuzzer, testCaseErr)
			}
			if withDefaultReferenceErr != nil {
				return codeGenErr(fuzzer, withDefaultReferenceErr)
			}
			if withReferenceErr != nil {
				return codeGenErr(fuzzer, withReferenceErr)
			}

			fmt.Printf("// %s\n\n", fuzzer.Interface.Name)
			fmt.Printf("%s\n\n", testCase)
			fmt.Printf("%s\n\n", withDefaultReference)
			fmt.Printf("%s\n", withReference)
		}

		return nil
	}

	app.Run(os.Args)
}

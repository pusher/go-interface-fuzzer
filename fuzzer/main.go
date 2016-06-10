package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"log"
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

		log.Println("I am going to fuzz", filename, "and I have successfully parsed it!")

		interfaces := fuzzparser.InterfacesFromAST(parsedFile)

		log.Println("Found the following interfaces:")
		for _, iface := range interfaces {
			log.Println("\t", iface.Name, "with functions:")
			for _, field := range iface.Functions {
				log.Println("\t\t", field.Name)
				for _, ty := range field.Parameters {
					log.Println("\t\t\ttakes a", ty.ToString())
				}
				for _, ty := range field.Returns {
					log.Println("\t\t\tgives a", ty.ToString())
				}
			}
		}

		wanteds := fuzzparser.WantedFuzzersFromAST(parsedFile)

		log.Println("And I want to generate the following fuzzers:")
		for _, wanted := range wanteds {
			log.Println("\t", wanted.InterfaceName)
			if wanted.ReturnsValue {
				log.Println("\t\tReference implementation: & ", wanted.Reference)
			} else {
				log.Println("\t\tReference implementation:", wanted.Reference)
			}
			log.Println("\t\tComparison Functions:", wanted.Comparison)
			log.Println("\t\tGenerator Functions:", wanted.Generator)
		}

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

			fmt.Printf("%v\n\n%v\n\n%v\n", testCase, withDefaultReference, withReference)
		}

		return nil
	}

	app.Run(os.Args)
}

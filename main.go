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

func main() {
	var opts CodeGenOptions

	app := cli.NewApp()
	app.Name = "go-interface-fuzzer"
	app.Usage = "Generate fuzz tests for Go interfaces."
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "complete, c",
			Usage:       "Generate a complete source file, with package name and imports",
			Destination: &opts.Complete,
		},
		cli.StringFlag{
			Name:        "filename, f",
			Usage:       "Use `FILE` as the file name when automatically resolving imports (defaults to the filename of the source fle)",
			Destination: &opts.Filename,
		},
		cli.StringFlag{
			Name:        "package, p",
			Usage:       "Use `NAME` as the package name (defaults to the package of the source file)",
			Destination: &opts.PackageName,
		},
	}
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
		if opts.Filename == "" {
			opts.Filename = filename
		}
		if opts.PackageName == "" {
			opts.PackageName = parsedFile.Name.Name
		}
		code, cerrs := CodeGen(opts, parsedFile.Imports, fuzzers)
		if len(cerrs) > 0 {
			return cli.NewExitError(errorList("Found some errors while generating code", cerrs), 1)
		}
		fmt.Println(code)

		return nil
	}

	app.Run(os.Args)
}

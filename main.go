package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
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
func reconcileFuzzers(interfaces map[string][]Function, wanteds []WantedFuzzer) ([]Fuzzer, []error) {
	var errs []error

	// Fuzzers are stored as a map from interface name to fuzzer.
	// This allows rapid checking for duplicates.
	fuzzers := make(map[string]Fuzzer)

	for _, wanted := range wanteds {
		_, present := fuzzers[wanted.InterfaceName]
		if present {
			errs = append(errs, fmt.Errorf("already have a fuzzer for '%s'", wanted.InterfaceName))
			continue
		}

		methods, ok := interfaces[wanted.InterfaceName]

		if !ok {
			errs = append(errs, fmt.Errorf("couldn't find interface '%s' in this file", wanted.InterfaceName))
		}

		fuzzer := Fuzzer{Name: wanted.InterfaceName, Methods: methods, Wanted: wanted}
		fuzzers[wanted.InterfaceName] = fuzzer
	}

	// Get a slice out of the 'fuzzers' map
	realfuzzers := make([]Fuzzer, len(fuzzers))
	i := 0
	for _, fuzzer := range fuzzers {
		realfuzzers[i] = fuzzer
		i++
	}
	return realfuzzers, errs
}

func main() {
	var opts CodeGenOptions
	var ifaceonly string
	var writeout bool

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
			Usage:       "Use `FILE` as the file name when automatically resolving imports (defaults to the filename of the source file)",
			Destination: &opts.Filename,
		},
		cli.StringFlag{
			Name:        "package, p",
			Usage:       "Use `NAME` as the package name (defaults to the package of the source file)",
			Destination: &opts.PackageName,
		},
		cli.BoolFlag{
			Name:        "no-test-case, T",
			Usage:       "Do not generate the TestFuzz... function",
			Destination: &opts.NoTestCase,
		},
		cli.BoolFlag{
			Name:        "no-default, D",
			Usage:       "Do not generate the Fuzz... function, implies no-test-case",
			Destination: &opts.NoDefaultFuzz,
		},
		cli.StringFlag{
			Name:        "interface",
			Usage:       "Ignore special comments and just generate a fuzz tester for the named interface, implies no-default",
			Destination: &ifaceonly,
		},
		cli.BoolFlag{
			Name:        "output, o",
			Usage:       "Write the output to the filename given by the -f flag, which must be specified",
			Destination: &writeout,
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

		// Extract all the interfaces
		interfaces := InterfacesFromAST(parsedFile)

		// Extract the wanted fuzzers
		var wanteds []WantedFuzzer
		var werrs []error
		if ifaceonly == "" {
			wanteds, werrs = WantedFuzzersFromAST(parsedFile)
		} else {
			// Default fuzzer for this interface.
			wanteds = append(wanteds, WantedFuzzer{InterfaceName: ifaceonly})
		}
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
			if writeout {
				return cli.NewExitError("When using -o a filename MUST be given to -f", 1)
			}
			opts.Filename = filename
		}
		if opts.PackageName == "" {
			opts.PackageName = parsedFile.Name.Name
		}
		code, cerrs := CodeGen(opts, parsedFile.Imports, fuzzers)
		if len(cerrs) > 0 {
			return cli.NewExitError(errorList("Found some errors while generating code", cerrs), 1)
		}

		if writeout {
			err := ioutil.WriteFile(opts.Filename, []byte(code), 0644)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
		} else {
			fmt.Println(code)
		}

		return nil
	}

	app.Run(os.Args)
}

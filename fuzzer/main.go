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

		interfaces := fuzzparser.ParseInterfacesFromAST(parsedFile)

		log.Println("Found the following interfaces:")
		for _, iface := range interfaces {
			log.Println("\t", iface.Name, "with methods:")
			for _, field := range iface.Methods {
				log.Println("\t\t", field.Name)
			}
		}

		return nil
	}

	app.Run(os.Args)
}

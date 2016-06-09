package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"

	"github.com/urfave/cli"
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
		parsedFile, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ParseComments)

		if err != nil {
			return cli.NewExitError(fmt.Sprintf("Could not parse file: '%s'", err.Error()), 1)
		}

		if parsedFile == nil {
			// Shouldn't be reachable, as 'err' should be non-nil if
			// the ast is nil.
			return cli.NewExitError("Could not parse file.", 1)
		}

		fmt.Println("I am going to fuzz", filename, "and I have successfully parsed it!")
		return nil
	}

	app.Run(os.Args)
}

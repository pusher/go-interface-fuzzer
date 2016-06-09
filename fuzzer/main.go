package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"

	"github.com/urfave/cli"
)

type InterfaceType struct {
	Name    string
	Methods []InterfaceMethod
}

type InterfaceMethod struct {
	MethodName string
	MethodType ast.FuncType
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

		log.Println("I am going to fuzz", filename, "and I have successfully parsed it!")

		interfaces := make([]InterfaceType, 0)

		ast.Inspect(parsedFile, func(n ast.Node) bool {
			if n == nil {
				return true
			}

			// We're interested in interface type specifications,
			// which we get from GenDecl.Specs -> TypeSpec.Type ->
			// InterfaceType.
			switch x := n.(type) {
			case *ast.GenDecl:
				if x.Tok == token.TYPE && x.Specs != nil {
					for _, spec := range x.Specs {
						switch tyspec := spec.(type) {
						case *ast.TypeSpec:
							name := tyspec.Name.Name
							log.Println("Found tyspec", name)
							switch ifacety := tyspec.Type.(type) {
							case *ast.InterfaceType:
								methods := make([]InterfaceMethod, 0)
								if ifacety.Methods != nil {
									for _, field := range ifacety.Methods.List {
										if field.Names == nil || len(field.Names) < 1 {
											continue
										}

										obj := field.Names[0].Obj

										if obj == nil || obj.Decl == nil {
											continue
										}
										switch funcdecl := obj.Decl.(type) {
										case *ast.Field:
											if funcdecl.Type == nil {
												continue
											}
											switch functype := funcdecl.Type.(type) {
											case *ast.FuncType:
												if functype == nil {
													continue
												}
												method := InterfaceMethod{MethodName: field.Names[0].Name, MethodType: *functype}
												methods = append(methods, method)
											}
										}
									}
								}
								interfaceType := InterfaceType{Name: name, Methods: methods}
								interfaces = append(interfaces, interfaceType)
							}
						}
					}
				}
			}

			return true
		})

		log.Println("Found the following interfaces:")
		for _, iface := range interfaces {
			log.Println("\t", iface.Name, "with methods:")
			for _, field := range iface.Methods {
				log.Println("\t\t", field.MethodName)
			}
		}

		return nil
	}

	app.Run(os.Args)
}

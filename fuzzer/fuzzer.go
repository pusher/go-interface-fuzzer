package main

import (
	fuzzparser "barrucadu/go-interface-fuzzer/parser"
)

// A fuzzer is a pair of an interface declaration and a description of
// how to generate the fuzzer.
type Fuzzer struct {
	Interface fuzzparser.Interface
	Wanted    fuzzparser.WantedFuzzer
}

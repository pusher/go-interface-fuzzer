go-interface-fuzzer
===

`go-interface-fuzzer` is a fuzzy testing tool for Go interfaces. The
goal of the project is to make it easier for developers to have
confidence in the correctness of their programs by combining
randomised testing with reference semantics.

Given an interface, a reference implementation, and some hints on how
to generate function parameters and compare function return values,
`go-interface-fuzzer` will generate testing functions which can be
used to check that the behaviour of an arbitrary other type
implementing the interface behaves the same.

See the `_examples` directory for a complete self-contained example.

## Table of Contents

- [Project Status](#project-status)
- [Getting Started](#getting-started)
  - [Installing](#installing)
  - [Usage](#usage)
    - [Incorporating into the build](#incorporating-into-the-build)
  - [Directives](#directives)
    - [`@fuzz interface` (required)](#fuzz-interface-required)
    - [`@known correct` (required)](#known-correct-required)
    - [`@invariant`](#invariant)
    - [`@comparison`](#comparison)
    - [`@generator state`](#generator-state)
  - [Defaults](#defaults)
- [Other Uses](#other-uses)
  - [Regression testing](#regression-testing)
  - [Assertion-only testing](#assertion-only-testing)

## Project Status

The tool is stable and the interface fixed. New functionality,
directives, and command-line arguments may be added, but existing
usages will not suddenly stop working.

## Getting Started

### Installing

To start using `go-interface-fuzzer`, install Go and run `go get`:

```sh
$ go get github.com/pusher/go-interface-fuzzer
```

This will install the `go-interface-fuzzer` command-line tool into
your `$GOBIN` path, which defaults to `$GOPATH/bin`.


### Usage

The generated code in the `_examples` directory is produced by

```bash
go-interface-fuzzer -c -o -f _examples/store.generated.go _examples/store.go
```

- The `-c` flag generates a **c**omplete source file, complete with
  package name and imports.
- The `-o` flag writes the **o**utput to the filename given by the
  `-f` flag.
- The `-f` flag specifies the **f**ilename to use when writing output
  and resolving imports.

The generated code can be customised further, see the full help text
(`go-interface-fuzzer --help`) for a complete flag listing.

The tool generates three functions, named after the interface used.
With the example file, the following functions are be produced:

 - `FuzzStoreWith(reference Store, test Store, rand *rand.Rand, maxops uint) error`

   Create a new reference store and test store, apply a
   randomly-generated list of actions, and bail out on inconsistency.

 - `FuzzStore(makeTest (func(int) Store), rand *rand.Rand, maxops uint) error`

   Call `FuzzStoreWithReference` with the ModelStore as the reference
   one.

- `FuzzTestStore(makeTest (func(int) Store), t *testing.T)`

   A test case parameterised by the store generating function, with a
   default maxops of 100.

By default `go-interface-fuzzer` generates an incomplete fragment: no
package name, no imports, just the three testing functions per
interface.


#### Incorporating into the build

Go makes adding a code generation stage to your build process quite
simple, with the `go generate` tool. To incorporate into your build,
add a comment to your source file:

```go
//go:generate go-interface-fuzzer -c -o -f output_file.go input_file.go
```

Typically this would be added to the same file which defines the
interface and provides the processing directives (see the next
section), but that isn't required.

To then actually generate the file, run `go generate`. It is not done
for you as part of `go build`.

For further information on code generation in Go see
"[Generating code](https://blog.golang.org/generate)", on the Go blog.


### Directives

An interface must be marked-up with some processing directives to
direct the tool. These directives are given inside a single multi-line
comment.

The minimum is just indicating that a fuzz tester should be generated
for the interface, and how to produce a new value of the reference
implementation type. For example:

```go
/*
@fuzz interface: Store
@known correct:  makeReferenceStore int
*/
type Store interface {
    // ...
}
```

The fuzzer definition does not need to be immediately next to the
interface, it can be anywhere in the source file.

See the `_examples` directory for a complete self-contained example
using most of the directives.


#### `@fuzz interface` (required)

This directive begins the definition of a fuzzer, and gives the name
of the interface to test.

**Example:** `@fuzz interface: Store`

**Argument syntax:** `InterfaceName`


#### `@known correct` (required)

This directive gives a function to produce a new value of the
reference implementation. It specifies the parameters of the function,
and whether the return type is a value or a pointer type.

The generated fuzzing function will expect a function argument with
the same parameters to create a new value of the type under test.

**Example:** `@known correct: makeReferenceStore int`

**Argument syntax:** `[&] FunctionName [ArgType1 ... ArgTypeN]`

The presence of a `&` means that this returns a value rather than a
pointer, and so a reference must be made to it.


#### `@invariant`

This directive specifies a property that must always hold. It is only
checked for the test implementation, not the reference implementation.

**Example:** `@invariant: %var.NumEntries() == len(%var.AsSlice())`

**Argument syntax:** `Expression`

The argument is a Go expression that evaluates to a boolean, with
`%var` replaced with the variable name.


#### `@comparison`

This directive specifies a function to use to compare two values. If
not specified the reflection package is used.

**Example:** `@comparison: *MessageIterator:CompareWith`

**Argument syntax:** `(Type:FunctionName | FunctionName Type)`

In the method form, the target of the comparison is passed as the sole
parameter; in the function form both are passed as parameters.


#### `@generator`

This directive specifies a function to generate a value of the
required type. It is passed a PRNG of type `*rand.Rand`. If no
generator for a type is specified, the tool will attempt to produce a
default; and report an error otherwise.

**Example:** `@generator: GenerateChannel model.Channel`

**Argument syntax:** `"[!] FunctionName Type`

The presence of a `!` means that this is a stateful function: it is
also passed a state parameter and is expected to return a new state as
its second result.


#### `@generator state`

This directive supplies an initial state for stateful generators. It
must be given if any generators are stateful. The initial state is any
legal Go expression; it is just copied verbatim into the generated
code.

**Example:** `@generator state: InitialGeneratorState`

**Argument syntax:** `Expression`


### Defaults

The following default **comparison** operations are used if not
overridden:

| Type            | Comparison                                   |
|-----------------|----------------------------------------------|
| `error`         | Equal if both values are `nil` or non-`nil`. |
| Everything else | `reflect.DeepEqual`                          |

The following default **generator** functions are used if not
overridden:

| Type            | Generator                                                           |
|-----------------|---------------------------------------------------------------------|
| `bool`          | `rand.Intn(2) == 0`                                                 |
| `byte`          | `byte(rand.Uint32())`                                               |
| `complex64`     | `complex(float32(rand.NormFloat64()), float32(rand.NormFloat64()))` |
| `complex128`    | `complex(rand.NormFloat64(), rand.NormFloat64())`                   |
| `float32`       | `float32(rand.NormFloat64())`                                       |
| `float64`       | `rand.NormFloat64()`                                                |
| `int`           | `rand.Int()`                                                        |
| `int8`          | `int8(rand.Int())`                                                  |
| `int16`         | `int16(rand.Int())`                                                 |
| `int32`         | `rand.Int31()`                                                      |
| `int64`         | `rand.Int63()`                                                      |
| `rune`          | `rune(rand.Int31())`                                                |
| `uint`          | `uint(rand.Uint32())`                                               |
| `uint8`         | `uint8(rand.Uint32())`                                              |
| `uint16`        | `uint16(rand.Uint32())`                                             |
| `uint32`        | `rand.Uint32()`                                                     |
| `uint64`        | `(uint64(rand.Uint32()) << 32) | uint64(rand.Uint32())`             |
| Everything else | **No default**                                                      |


## Other Uses
### Regression testing

Although the motivating use-case for `go-interface-fuzzer` was an
interface with two implementations that should have identical
behaviour, there is nothing preventing the use of multiple *versions*
of the *same* implementation. This facilitates regression testing,
although at the cost of needing to keep the old implementation around.

Here are the concrete steps you would need to follow to do this:

1. Make a copy of your current implementation, with a new name.
2. Write a new implementation.
3. Use the current implementation as the reference implementation for
   the generated fuzzer.

This isn't quite the same as reference correctness, as any bugs in the
old implementation which the new fixes will be reported as an error in
the new implementation.


### Assertion-only testing

By supplying the same implementation as both the reference and the
test implementation, the fuzz tester will simply check invariants.
Although, it'll be a little slow, because every operation is being
performed twice.

In the future, there will probably be an "invariant-only" mode of
operation.

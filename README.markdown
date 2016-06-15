go-interface-fuzzer: randomised testing for Go interfaces
===

`go-interface-fuzzer` is a fuzzy testing tool for Go interfaces. Given
an interface, a reference implementation, and some hints on how to
generate function parameters or compare function return values,
`go-interface-fuzzer` will generate testing functions which can be
used to check that the behaviour of an arbitrary other type
implementing the interface behaves the same.

Usage
---

### Command-line Flags

```
NAME:
   go-interface-fuzzer - Generate fuzz tests for Go interfaces.

USAGE:
   go-interface-fuzzer [global options] command [command options] [arguments...]

VERSION:
   0.0.0

COMMANDS:
GLOBAL OPTIONS:
   --complete, -c            Generate a complete source file, with package name and imports
   --filename FILE, -f FILE  Use FILE as the file name when automatically resolving imports (defaults to the filename of the source file)
   --package NAME, -p NAME   Use NAME as the package name (defaults to the package of the source file)
   --no-test-case, -T        Do not generate the TestFuzz... function
   --no-default, -D          Do not generate the Fuzz... function, implies no-test-case
   --interface value         Ignore special comments and just generate a fuzz tester for the named interface, implies no-default
   --help, -h                show help
   --version, -v             print the version
```

By default `go-interface-fuzzer` generates an incomplete fragment: no
package name, no imports, just the three testing functions per
interface. The `-c` flag can be given to generate a complete file. If
the generated file will not be in the same package as the source file,
the `-f` and `-p` options can be used to specify where.


### Special Comments

Firstly, you need to mark up the interface you want to generate a fuzz
tester for. The minimum is just indicating that a fuzz tester should
be generated for the interface, and how to produce a new value of the
reference implementation type. All other fields are optional:

```go
/*
`@fuzz interface` indicates that this is a special comment for the
interface fuzzer, and gives the name of the interface to use.

This means that the special comment does not need to be immediately
next to the interface, it can be anywhere in the source file.

@fuzz interface: Store

**Syntax:**

  "InterfaceName"


`@known correct` is the type that is, well, known to be correct; given
as a function to produce a new value of that type.

The generated fuzzing function will expect a function argument with
the same parameters to create a new value of the type under test.

@known correct: & NewModelStore int

**Syntax:**

  "[&] FunctionName [ArgType1 ... ArgTypeN]"

  The presence of a `&` means that this returns a value rather than a
  pointer, and so a reference must be made to it.


`@comparison` specifies a function to use to compare two values. If
not specified the reflection package is used.

@comparison: *MessageIterator:CompareWith

**Syntax:**

  "(Type:FunctionName | FunctionName Type)"

  In the method form, the target of the comparison is passed as the
  sole parameter; in the function form both are passed as parameters.


`@generator` specifies a function to generate a value of the required
type. It is passed a PRNG of type *rand.Rand. If no generator for a
type is specified, the tool will attempt to produce a default; and
report an error otherwise.

@generator:   GenerateChannel   model.Channel
@generator: ! GenerateID        model.EventID
@generator: ! GenerateIDMessage model.IDMessage
@generator:   GeneratePartition model.Partition

**Syntax:**

  "[!] FunctionName Type"

  The presence of a `!` means that this is a stateful function: it is
  also passed a state parameter and is expected to return a new state
  as its second result.


`@generator state` supplies an initial state for stateful
generators. It must be given if any generators are stateful. The
initial state is any legal Go expression; it is just copied verbatim
into the generated code.

@generator state: InitialGeneratorState

**Syntax:**

  "Expression"


This will generate the following functions:

 - `FuzzStoreWith(reference Store, test Store, rand *rand.Rand, maxops uint) error`

   Create a new reference store and test store, apply a
   randomly-generated list of actions, and bail out on inconsistency.

 - `FuzzStore(makeTest (func(int) Store), rand *rand.Rand, maxops uint) error`

   Call `FuzzStoreWithReference` with the ModelStore as the reference one.

- `FuzzTestStore(makeTest (func(int) Store), t *testing.T)`

   A test case parameterised by the store generating function, with a
   default maxops of 100.
*/
type Store interface {
    Put(IDMessage) error
    EntriesSince(ID) (ID, []IDMessage)
    EntriesSinceIter(ID) (ID, *MessageIterator)
    MostRecentID() ID
    AsSlice() []IDMessage
    MessageLimit() int
}
```

Once you have your special comments, in which only `@fuzz interface`
and `@known correct` is necessary, run `go-interface-fuzzer` on the
file and it will spit out the testing functions.

All of the comments relating to one fuzzer must be in the same comment
group, which is a collection of comments with no separating lines
(even blank). Multiple fuzzers can be defined in the same comment
group, the `@fuzz interface` line starts a new fuzzer definition.

Defaults
--

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

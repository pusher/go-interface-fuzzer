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

Firstly, you need to mark up the interface you want to generate a fuzz
tester for. The minimum is just indicating that a fuzz tester should
be generated for the interface, and how to produce a new value of the
reference implementation type. All other fields are optional:

~~~go
/*
`@fuzz interface` indicates that this is a special comment for the
interface fuzzer, and gives the name of the interface to use.

This means that the special comment does not need to be immediately
next to the interface, it can be anywhere in the source file.

@fuzz interface: Store

`@known correct` is the type that is, well, known to be correct; given
as a function to produce a new value of that type.

The generated fuzzing function will expect a function argument with
the same parameters to create a new value of the type under test.

@known correct: NewModelStore(int) ModelStore

`@before compare` specifies a function to apply to a value before
comparing it.

The `type.func()` syntax means that this is a method to call on the
value, otherwise the syntax `func(type)` can be used to denote a
"normal" function. There can be no other parameters.

@before compare: ID.ToUint() uint

`@comparison` specifies a function to use to compare two values. If
not specified the reflection package is used. The syntax `func(type,
type)` can also be used, where both types must be the same.

@comparison: *MessageIterator.CompareWith(*MessageIterator) bool

`@generator` specifies a function to generate a value of the required
type. It is passed the list of all generated values so far, in order
to tune the generation as the system evolves, if desired. If no
generator is specified for a type, quickcheck is used.

@generator: GenerateAnID([]ID, rand *rand.Rand) ID

This will generate the following functions:

 - `FuzzStoreWithReference(makeReferenceStore (func(int) Store), makeTestStore (func(int) Store), rand *rand.Rand, uint min, uint max) error`

   Create a new reference store and test store, apply a
   randomly-generated list of actions between the given length bounds,
   and bail out on inconsistency.

 - `FuzzStore(makeTestStore (func(int) Store), rand *rand.Rand, uint min, uint max) error`

   Call `FuzzStoreWithReference` with the ModelStore as the reference one.

- `FuzzTestStore(makeTestStore (func(int) Store), t *testing.T)`

   A test case parameterised by the store generating function, with a
   default min of 50 and max of 100.
*/
type Store interface {
    Put(IDMessage) error
    EntriesSince(ID) (ID, []IDMessage)
    EntriesSinceIter(ID) (ID, *MessageIterator)
    MostRecentID() ID
    AsSlice() []IDMessage
    MessageLimit() int
}
~~~

Once you have your special comments, in which only `@fuzz interface`
and `@known correct` is necessary, run `go-interface-fuzzer` on the
file and it will spit out the testing functions.

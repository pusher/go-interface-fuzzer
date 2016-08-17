package example

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// Store

func FuzzTestStore(makeTest func(int) Store, t *testing.T) {
	rand := rand.New(rand.NewSource(0))

	err := FuzzStore(makeTest, rand, 100)

	if err != nil {
		t.Error(err)
	}
}

func FuzzStore(makeTest func(int) Store, rand *rand.Rand, max uint) error {
	var (
		argInt int
	)

	argInt = rand.Int()

	expectedStore := makeReferenceStore(argInt)
	actualStore := makeTest(argInt)

	return FuzzStoreWith(&expectedStore, actualStore, rand, max)
}

func FuzzStoreWith(reference Store, test Store, rand *rand.Rand, maxops uint) error {
	// Create initial state
	state := uint(0)

	for i := uint(0); i < maxops; i++ {
		// Pick a random number between 0 and the number of methods of the interface. Then do that method on
		// both, check for discrepancy, and bail out on error. Simple!

		actionToPerform := rand.Intn(6)

		switch actionToPerform {
		case 0:
			// Call the method on both implementations
			var (
				argMessage Message
			)

			argMessage, state = generateMessage(rand, state)

			expectedError := reference.Put(argMessage)
			actualError := test.Put(argMessage)

			// And check for discrepancies.
			if !((expectedError == nil) == (actualError == nil)) {
				return fmt.Errorf("inconsistent result in Put\nexpected: %v\nactual:   %v", expectedError, actualError)
			}
		case 1:
			// Call the method on both implementations
			var (
				argID      ID
				argChannel Channel
			)

			argID, state = generateID(rand, state)
			argChannel = generateChannel(rand)

			expectedID, expectedMessage := reference.EntriesSince(argID, argChannel)
			actualID, actualMessage := test.EntriesSince(argID, argChannel)

			// And check for discrepancies.
			if !reflect.DeepEqual(expectedID, actualID) {
				return fmt.Errorf("inconsistent result in EntriesSince\nexpected: %v\nactual:   %v", expectedID, actualID)
			}
			if !reflect.DeepEqual(expectedMessage, actualMessage) {
				return fmt.Errorf("inconsistent result in EntriesSince\nexpected: %v\nactual:   %v", expectedMessage, actualMessage)
			}
		case 2:
			// Call the method on both implementations
			expectedID := reference.MostRecentID()
			actualID := test.MostRecentID()

			// And check for discrepancies.
			if !reflect.DeepEqual(expectedID, actualID) {
				return fmt.Errorf("inconsistent result in MostRecentID\nexpected: %v\nactual:   %v", expectedID, actualID)
			}
		case 3:
			// Call the method on both implementations
			expectedInt := reference.NumEntries()
			actualInt := test.NumEntries()

			// And check for discrepancies.
			if !reflect.DeepEqual(expectedInt, actualInt) {
				return fmt.Errorf("inconsistent result in NumEntries\nexpected: %v\nactual:   %v", expectedInt, actualInt)
			}
		case 4:
			// Call the method on both implementations
			expectedMessage := reference.AsSlice()
			actualMessage := test.AsSlice()

			// And check for discrepancies.
			if !reflect.DeepEqual(expectedMessage, actualMessage) {
				return fmt.Errorf("inconsistent result in AsSlice\nexpected: %v\nactual:   %v", expectedMessage, actualMessage)
			}
		case 5:
			// Call the method on both implementations
			expectedInt := reference.MessageLimit()
			actualInt := test.MessageLimit()

			// And check for discrepancies.
			if !reflect.DeepEqual(expectedInt, actualInt) {
				return fmt.Errorf("inconsistent result in MessageLimit\nexpected: %v\nactual:   %v", expectedInt, actualInt)
			}
		}

		if !(reference.NumEntries() == len(reference.AsSlice())) {
			return errors.New("invariant violated: %var.NumEntries() == len(%var.AsSlice())")
		}

		if !(reference.NumEntries() <= reference.MessageLimit()) {
			return errors.New("invariant violated: %var.NumEntries() <= %var.MessageLimit()")
		}

	}

	return nil
}

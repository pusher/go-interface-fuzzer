func FuzzTestStore(makeTest (func(int) Store), t *testing.T) {
	rand := rand.New(rand.NewSource(0))

	err := FuzzStore(makeTest, rand, 100)

	if err != nil {
		t.Error(err)
	}
}

func FuzzStore(makeTest (func (int) Store), rand *rand.Rand, max uint) error {
	var (
		argInt int
	)
	
	argInt = rand.Int()
	
	retStoreA := makeReferenceStore(argInt)
	retStoreB := makeTest(argInt)

	return FuzzStoreWith(&reta0, retb0, rand, max)
}

func FuzzStoreWith(reference Store, test Store, rand *rand.Rand, maxops uint) error {
	actionsToPerform := maxops

	// Create initial state
	state := uint(0)

	for actionsToPerform > 0 {
		// Pick a random number between 0 and the number of methods of the interface. Then do that method on
		// both, check for discrepancy, and bail out on error. Simple!

		actionToPerform := rand.Intn(6)

		switch actionToPerform {
		case 0:
			// Call the method on both implementations
			var (
				argModelIDMessage model.IDMessage
			)
			
			argModelIDMessage, state = generateIDMessage(rand, state)
			
			retErrorA := reference.Put(argModelIDMessage)
			retErrorB := test.Put(argModelIDMessage)
		
			// And check for discrepancies.
			if !((reta0 == nil) == (retb0 == nil)) {
				return fmt.Errorf("Inconsistent result in Put\nexpected: %v\nactual:   %v", reta0, retb0)
			}
		case 1:
			// Call the method on both implementations
			var (
				argModelID model.ID
				argModelChannel model.Channel
			)
			
			argModelID, state = generateID(rand, state)
			argModelChannel = generateChannel(rand)
			
			retModelIDA, retModelIDMessageA := reference.EntriesSince(argModelID, argModelChannel)
			retModelIDB, retModelIDMessageB := test.EntriesSince(argModelID, argModelChannel)
		
			// And check for discrepancies.
			if !reflect.DeepEqual(reta0, retb0) {
				return fmt.Errorf("Inconsistent result in EntriesSince\nexpected: %v\nactual:   %v", reta0, retb0)
			}
			if !reflect.DeepEqual(reta1, retb1) {
				return fmt.Errorf("Inconsistent result in EntriesSince\nexpected: %v\nactual:   %v", reta1, retb1)
			}
		case 2:
			// Call the method on both implementations
			var (
				argModelID model.ID
				argModelChannel model.Channel
			)
			
			argModelID, state = generateID(rand, state)
			argModelChannel = generateChannel(rand)
			
			retModelIDA, retMessageIteratorA := reference.EntriesSinceIter(argModelID, argModelChannel)
			retModelIDB, retMessageIteratorB := test.EntriesSinceIter(argModelID, argModelChannel)
		
			// And check for discrepancies.
			if !reflect.DeepEqual(reta0, retb0) {
				return fmt.Errorf("Inconsistent result in EntriesSinceIter\nexpected: %v\nactual:   %v", reta0, retb0)
			}
			if !compareMessageIterators(reta1, retb1) {
				return fmt.Errorf("Inconsistent result in EntriesSinceIter\nexpected: %v\nactual:   %v", reta1, retb1)
			}
		case 3:
			// Call the method on both implementations
			retModelIDA := reference.MostRecentID()
			retModelIDB := test.MostRecentID()
		
			// And check for discrepancies.
			if !reflect.DeepEqual(reta0, retb0) {
				return fmt.Errorf("Inconsistent result in MostRecentID\nexpected: %v\nactual:   %v", reta0, retb0)
			}
		case 4:
			// Call the method on both implementations
			retModelIDMessageA := reference.AsSlice()
			retModelIDMessageB := test.AsSlice()
		
			// And check for discrepancies.
			if !reflect.DeepEqual(reta0, retb0) {
				return fmt.Errorf("Inconsistent result in AsSlice\nexpected: %v\nactual:   %v", reta0, retb0)
			}
		case 5:
			// Call the method on both implementations
			retIntA := reference.MessageLimit()
			retIntB := test.MessageLimit()
		
			// And check for discrepancies.
			if !reflect.DeepEqual(reta0, retb0) {
				return fmt.Errorf("Inconsistent result in MessageLimit\nexpected: %v\nactual:   %v", reta0, retb0)
			}
		}

		actionsToPerform --
	}

	return nil
}

func FuzzTestStore(makeTest (func(int) Store), t *testing.T) {
	rand := rand.New(rand.NewSource(0))

	err := FuzzStore(makeTest, rand, 100)

	if err != nil {
		t.Error(err)
	}
}

func FuzzStore(makeTest (func (int) Store), rand *rand.Rand, max uint) error {
	var (
		arg0 int
	)
	
	arg0 = rand.Int()
	
	reta0 := makeReferenceStore(arg0)
	retb0 := makeTest(arg0)

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
				arg0 model.IDMessage
			)
			
			arg0, state = generateIDMessage(rand, state)
			
			reta0 := reference.Put(arg0)
			retb0 := test.Put(arg0)
		
			// And check for discrepancies.
			if !((reta0 == nil) == (retb0 == nil)) {
				return fmt.Errorf("Inconsistent result in Put\nexpected: %v\nactual:   %v", reta0, retb0)
			}
		case 1:
			// Call the method on both implementations
			var (
				arg0 model.ID
				arg1 model.Channel
			)
			
			arg0, state = generateID(rand, state)
			arg1 = generateChannel(rand)
			
			reta0, reta1 := reference.EntriesSince(arg0, arg1)
			retb0, retb1 := test.EntriesSince(arg0, arg1)
		
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
				arg0 model.ID
				arg1 model.Channel
			)
			
			arg0, state = generateID(rand, state)
			arg1 = generateChannel(rand)
			
			reta0, reta1 := reference.EntriesSinceIter(arg0, arg1)
			retb0, retb1 := test.EntriesSinceIter(arg0, arg1)
		
			// And check for discrepancies.
			if !reflect.DeepEqual(reta0, retb0) {
				return fmt.Errorf("Inconsistent result in EntriesSinceIter\nexpected: %v\nactual:   %v", reta0, retb0)
			}
			if !compareMessageIterators(reta1, retb1) {
				return fmt.Errorf("Inconsistent result in EntriesSinceIter\nexpected: %v\nactual:   %v", reta1, retb1)
			}
		case 3:
			// Call the method on both implementations
			reta0 := reference.MostRecentID()
			retb0 := test.MostRecentID()
		
			// And check for discrepancies.
			if !reflect.DeepEqual(reta0, retb0) {
				return fmt.Errorf("Inconsistent result in MostRecentID\nexpected: %v\nactual:   %v", reta0, retb0)
			}
		case 4:
			// Call the method on both implementations
			reta0 := reference.AsSlice()
			retb0 := test.AsSlice()
		
			// And check for discrepancies.
			if !reflect.DeepEqual(reta0, retb0) {
				return fmt.Errorf("Inconsistent result in AsSlice\nexpected: %v\nactual:   %v", reta0, retb0)
			}
		case 5:
			// Call the method on both implementations
			reta0 := reference.MessageLimit()
			retb0 := test.MessageLimit()
		
			// And check for discrepancies.
			if !reflect.DeepEqual(reta0, retb0) {
				return fmt.Errorf("Inconsistent result in MessageLimit\nexpected: %v\nactual:   %v", reta0, retb0)
			}
		}

		actionsToPerform --
	}

	return nil
}

/*

A very simple message store example: messages are sorted into
"channels", and have an associated "ID". A message cannot directly be
retrieved, but all messages since an ID can be retrieved: either as an
iterator or as a slice.

*/

package example

/*
@fuzz interface: Store

@known correct: & makeReferenceStore int

@comparison: compareMessageIterators *MessageIterator

@generator state: uint(0)

@generator:   generateChannel   model.Channel
@generator: ! generateID        model.ID
@generator: ! generateIDMessage model.IDMessage
*/
type Store interface {
	// Inserts an entry in the store. Returns an error if an entry with greater or
	// equal ID was already inserted.
	Put(model.IDMessage) error

	// Returns a slice of all messaegs in the specified channels, from the
	// specified ID to the message with most recent ID. All messages will have IDs
	// such that `sinceID < ID <= mostRecentID`.
	EntriesSince(model.ID, model.Channel) (model.ID, []model.IDMessage)

	// Same as EntriesSince, but returns a MessageIterator rather
	// than a slice.
	EntriesSinceIter(model.ID, model.Channel) (model.ID, *MessageIterator)

	// Returns the ID of the most recently inserted message.
	MostRecentID() model.ID

	// Returns all messages across all partitions and channels as
	// a single slice, sorted by ID.
	AsSlice() []model.IDMessage

	// Returns the maximum number of messages in the store.
	MessageLimit() int
}

// Compare two message iterators
func compareMessageIterators(expected, actual *MessageIterator) bool {
	return reflect.DeepEqual(expected.ToSlice(), actual.ToSlice())
}

// Create a new clean ModelStore for testing purposes.
func makeModelStore(capacity int) ModelStore {
	return NewModelStore(capacity, []model.IDMessage{})
}

// Generate a channel name. Use a short string.
func generateChannel(rand *rand.Rand) model.Channel {
	return model.Channel(randomString(rand, 1+rand.Intn(4)))
}

// Generate an ID. Randomly generate some across the full range of
// rand.Int. Sometimes use only those that have been seen before. This
// is so that EntriesSince and EntriesSinceIter have a good chance of
// actually returning something (whilst not disregarding the "error"
// case where the ID is invalid).
func generateID(rand *rand.Rand, maxSoFar uint) (model.ID, uint) {
	newid := uint(rand.Uint32()) % 64

	if rand.Intn(2) == 0 {
		newid = uint(rand.Uint32()) % maxSoFar
	}

	return model.ID(newid), maxSoFar
}

// Generate an ID message. Randomly generate some. Use a
// monotonically-increasing ID for others. This is because Put
// requires that, so if IDs were totally random, this wouldn't be
// likely to produce particularly good results.
func generateIDMessage(rand *rand.Rand, maxSoFar uint) (model.IDMessage, uint) {
	newid := uint(rand.Uint32()) % 64

	msg := model.Message{
		Channel: generateChannel(rand),
		Data:    model.MessageData(randomString(rand, 1+rand.Intn(4))),
	}

	if rand.Intn(2) == 0 {
		newid = maxSoFar + 1
	}

	if newid > maxSoFar {
		maxSoFar = newid
	}

	return model.IDMessage{ID: model.ID(newid), Message: msg}, maxSoFar
}

// Generate a random string
func randomString(rand *rand.Rand, n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

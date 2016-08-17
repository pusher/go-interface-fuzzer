/*

A very simple message store example: messages are sorted into
"channels", and have an associated "ID". A message cannot directly be
retrieved, but all messages since an ID can be retrieved: either as an
iterator or as a slice.

This assumes a type called ReferenceStore and a function
NewReferenceStore are in scope.

*/

package example

import (
	"math/rand"
	"reflect"
)

/*
@fuzz interface: Store

@known correct: & makeReferenceStore int

@invariant: %var.NumEntries() == len(%var.AsSlice())
@invariant: %var.NumEntries() <= %var.MessageLimit()

@generator state: uint(0)

@generator:   generateChannel Channel
@generator: ! generateID      ID
@generator: ! generateMessage Message
*/
type Store interface {
	// Inserts an entry in the store. Returns an error if an entry with greater or
	// equal ID was already inserted.
	Put(msg Message) error

	// Returns a slice of all messages in the specified channels, from the
	// specified ID to the message with most recent ID. All messages will have IDs
	// such that `sinceID < ID <= mostRecentID`.
	EntriesSince(sinceID ID, channel Channel) (ID, []Message)

	// Returns the ID of the most recently inserted message.
	MostRecentID() ID

	// Returns the number of messages in the store.
	NumEntries() int

	// Returns all messages across all channels as a single slice,
	// sorted by ID.
	AsSlice() []Message

	// Returns the maximum number of messages in the store.
	MessageLimit() int
}

type Message struct {
	// Each message has a unique ID.
	ID ID

	// A message belongs to a specific channel.
	Channel Channel

	// And has a body
	Body string
}

type Channel string

type ID uint64

// Create a new clean ReferenceStore for testing purposes.
func makeReferenceStore(capacity int) ReferenceStore {
	return NewReferenceStore(capacity, []Message{})
}

// Generate a channel name. Use a short string.
func generateChannel(rand *rand.Rand) Channel {
	return Channel(randomString(rand, 1+rand.Intn(4)))
}

// Generate an ID. Randomly generate some across the full range of
// rand.Int. Sometimes use only those that have been seen before. This
// is so that EntriesSince and EntriesSinceIter have a good chance of
// actually returning something (whilst not disregarding the "error"
// case where the ID is invalid).
func generateID(rand *rand.Rand, maxSoFar uint) (ID, uint) {
	newid := uint(rand.Uint32()) % 64

	if rand.Intn(2) == 0 {
		newid = uint(rand.Uint32()) % maxSoFar
	}

	return ID(newid), maxSoFar
}

// Generate an ID message. Randomly generate some. Use a
// monotonically-increasing ID for others. This is because Put
// requires that, so if IDs were totally random, this wouldn't be
// likely to produce particularly good results.
func generateMessage(rand *rand.Rand, maxSoFar uint) (Message, uint) {
	newid := uint(rand.Uint32()) % 64

	msg := Message{
		Channel: generateChannel(rand),
		Data:    MessageData(randomString(rand, 1+rand.Intn(4))),
	}

	if rand.Intn(2) == 0 {
		newid = maxSoFar + 1
	}

	if newid > maxSoFar {
		maxSoFar = newid
	}

	return Message{ID: ID(newid), Message: msg}, maxSoFar
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

package main

import (
	"fmt"
	"strings"
	"testing"
	"testing/quick"
	"unicode"
)

// Check that matchDelims can remove delimiters and get back the
// original string.
func TestMatchDelims(t *testing.T) {
	f := func(str string, numL uint, numR uint) bool {
		numL = numL % 10
		numR = numR % 10

		pref := makeFixedString("(", numL)
		suff := makeFixedString(")", numR)

		teststr := fmt.Sprintf("%s|%s|%s", pref, str, suff)

		innerstr, balanced := matchDelims(teststr, "(", ")")

		if innerstr != "|"+str+"|" {
			expectedActual("Failed at matching delimiters.", "|"+str+"|", innerstr, t)
		}

		if (balanced && numL != numR) || (!balanced && numL == numR) {
			t.Fatal("Reported delimiters balanced when they are not.")
		}

		return true
	}

	quickcheck(f, t)
}

// Check that the result of splitLines can be converted back to the
// original string.
func TestSplitLinesInvertible(t *testing.T) {
	f := func(str string) bool {
		splitted := splitLines(str)
		merged := strings.Join(splitted, "\n")
		if str != merged {
			expectedActual("Splitting a string by lines is not invertible.", str, merged, t)
		}

		return true
	}

	quickcheck(f, t)
}

// Check that the result of splitLines doesn't contain newlines.
func TestSplitLinesNoNewlines(t *testing.T) {
	f := func(str string) bool {
		splitted := splitLines(str)
		for _, line := range splitted {
			if strings.Contains(line, "\n") {
				chunks := strings.Split(line, "\n")
				expectedActual("Newline found in output of string split by lines.", chunks[0], line, t)
			}
		}

		return true
	}

	quickcheck(f, t)
}

// Check matchPrefix correctly removes the prefix.
func TestMatchPrefix(t *testing.T) {
	f := func(pref, suff string) bool {
		str := pref + suff

		suff2, ok := matchPrefix(str, pref)

		if !ok || suff2 != suff {
			expectedActual("Failed to match prefix.", suff, suff2, t)
		}

		return true
	}

	quickcheck(f, t)
}

// Check that takeWhileIn returns a prefix of the original string.
func TestTakeWhileInGivesPrefix(t *testing.T) {
	f := func(str, allowed string) bool {
		pref, suff := takeWhileIn(str, allowed)

		suff2, ok := matchPrefix(str, pref)

		if !ok || suff != suff2 {
			expectedActual("Failed to match prefix given by takeWhileIn.", suff2, suff, t)
		}

		return true
	}

	quickcheck(f, t)
}

// Check that takeWhileIn returns a prefix which only contains allowed characters.
func TestTakeWhileInGivesGoodPrefix(t *testing.T) {
	f := func(str, allowed string) bool {
		pref, _ := takeWhileIn(str, allowed)

		for _, chr := range pref {
			if !strings.ContainsRune(allowed, chr) {
				t.Fatal("Found invalid character in prefix.")
			}
		}

		return true
	}

	quickcheck(f, t)
}

// Check that filter doesn't increase the length.
func TestFilterLen(t *testing.T) {
	f := func(str string) bool {
		return len(str) >= len(filter(str, unicode.IsLetter))
	}

	quickcheck(f, t)
}

// Check that the result of filter only contains characters matching
// the predicate.
func TestFilterPredicate(t *testing.T) {
	f := func(str string) bool {
		predicate := unicode.IsLetter
		filtered := filter(str, predicate)

		for _, r := range filtered {
			if !predicate(r) {
				t.Fatal("Found invalid character in filter output.")
			}
		}

		return true
	}

	quickcheck(f, t)
}

// Check that filtering is idempotent.
func TestFilterIdempotent(t *testing.T) {
	f := func(str string) bool {
		predicate := unicode.IsLetter
		filtered1 := filter(str, predicate)
		filtered2 := filter(filtered1, predicate)

		if filtered1 != filtered2 {
			expectedActual("Filtering is not idempotent.", filtered1, filtered2, t)
		}

		return true
	}

	quickcheck(f, t)
}

// Helper for "expected"/"actual" output.
func expectedActual(msg string, expected interface{}, actual interface{}, t *testing.T) {
	t.Fatal(fmt.Sprintf("%s\nGot: %v\nExpected: %v", msg, actual, expected))
}

// Helper for quickcheck
func quickcheck(f interface{}, t *testing.T) {
	var c quick.Config
	c.MaxCount = 10
	if err := quick.Check(f, &c); err != nil {
		t.Fatal(err)
	}
}

// Helper for making strings of a single character.
func makeFixedString(of string, len uint) (s string) {
	var i uint
	for i = 0; i < len; i++ {
		s = s + of
	}
	return
}

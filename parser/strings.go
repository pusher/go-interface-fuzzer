// String operations used elsewhere in the parser.

package parser

import (
	"strings"
	"unicode"
)

// Drop a prefix and suffix. Complain if the prefix is present but not
// the suffix and vice versa.
func matchDelims(s, pref, suff string) (string, bool) {
	s1, prefCount := removePrefix(s, pref)
	s2, suffCount := removeSuffix(s1, suff)

	return s2, prefCount == suffCount
}

// Remove a prefix from a string as many times as possible.
func removePrefix(s, pref string) (string, uint) {
	var count uint
	for strings.HasPrefix(s, pref) {
		s = strings.TrimPrefix(s, pref)
		count++
	}
	return s, count
}

// Remove a suffix from a string as many times as possible.
func removeSuffix(s, suff string) (string, uint) {
	var count uint
	for strings.HasSuffix(s, suff) {
		s = strings.TrimSuffix(s, suff)
		count++
	}
	return s, count
}

// Split a string into lines.
func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

// Check if a string has a prefix and, if so, return the bit following
// the prefix with whitespace betwen the prefix and the rest trimmed.
func matchPrefix(s, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		after := strings.TrimPrefix(s, prefix)
		return strings.TrimLeftFunc(after, unicode.IsSpace), true
	}
	return s, false
}

// Return a prefix and a suffix where the prefix contains only allowed
// characters.
func takeWhileIn(s, allowed string) (string, string) {
	for i, chr := range s {
		if !strings.ContainsRune(allowed, chr) {
			return s[:i], s[i:]
		}
	}

	return s, ""
}

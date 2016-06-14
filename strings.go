// String operations used elsewhere in the parser.

package main

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

// Indent every non-blank line by the given number of tabs.
func indentLines(s string, indent string) string {
	lines := strings.Split(s, "\n")
	indented := indent + strings.Join(lines, "\n"+indent)
	return strings.Replace(indented, indent+"\n", "\n", -1)
}

// Filter runes in a string. Unlike takeWhileIn this processes the
// entire string, not just a prefix.
func filter(s string, allowed func(rune) bool) string {
	f := func(r rune) rune {
		if allowed(r) {
			return r
		}
		return -1
	}

	return strings.Map(f, s)
}

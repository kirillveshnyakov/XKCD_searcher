package words

import (
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/kljensen/snowball/english"
)

func Norm(phrase string) []string {
	words := strings.FieldsFunc(phrase, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	uniqueNormWords := make(map[string]bool, len(words))

	for _, word := range words {
		word = english.Stem(word, true)
		if !english.IsStopWord(word) {
			uniqueNormWords[word] = true
		}
	}

	return slices.Collect(maps.Keys(uniqueNormWords))
}

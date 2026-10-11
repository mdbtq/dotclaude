package lint

import (
	"strings"
	"unicode"
)

type wordSet map[string]bool

// stopwords are left out of similarity scores: they occur in nearly every
// description and say nothing about what a skill or memory is for.
var stopwords = wordSet{}

func init() {
	for _, w := range strings.Fields(`a an and or the of to in on for with by from as at is are be
		it its this that these those when use used using user users you your can will should
		not no do does via into any all also than then so such e g eg i e ie etc
		de het een en van te in op voor met bij niet geen is zijn wordt dat die dit`) {
		stopwords[w] = true
	}
}

// words returns the distinct content words of s, lowercased.
func words(s string) wordSet {
	set := wordSet{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(w) > 2 && !stopwords[w] {
			set[w] = true
		}
	}
	return set
}

// similarity is the overlap coefficient of two word sets: shared words over
// the size of the smaller set. Descriptions under five content words score 0,
// because a couple of shared words means nothing there.
func similarity(a, b wordSet) float64 {
	if len(a) < 5 || len(b) < 5 {
		return 0
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	shared := 0
	for w := range a {
		if b[w] {
			shared++
		}
	}
	return float64(shared) / float64(len(a))
}

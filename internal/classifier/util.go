package classifier

import (
	"github.com/agnivade/levenshtein"
	"github.com/ghobs91/lodestone/internal/regex"
	"github.com/mozillazg/go-unidecode"
)

// levenshteinMaxThreshold scales the acceptable edit distance with the target
// title length: 15% of the normalized title length, clamped to [2, 8].
// Short titles (e.g. "Her") get a tight threshold of 2, while long titles
// (e.g. "The Lord of the Rings: The Return of the King") get up to 8.
func levenshteinMaxThreshold(target string) int {
	n := len(levenshteinNormalizeString(target))
	t := n * 15 / 100
	if t < 2 {
		return 2
	}
	if t > 8 {
		return 8
	}
	return t
}

func levenshteinFindBestMatch[T any](target string, items []T, getCandidates func(T) []string) (t T, ok bool) {
	threshold := levenshteinMaxThreshold(target)
	minDistance := threshold + 1
	bestMatch := -1

	for i, item := range items {
		candidates := getCandidates(item)

		distance := levenshteinFindMinDistance(target, candidates)
		if distance >= 0 && distance < minDistance {
			minDistance = distance
			bestMatch = i

			if distance == 0 {
				break
			}
		}
	}

	if bestMatch == -1 {
		return t, false
	}

	return items[bestMatch], true
}

func levenshteinFindMinDistance(target string, candidates []string) int {
	normTarget := levenshteinNormalizeString(target)
	triedCandidates := make(map[string]struct{}, len(candidates))
	minDistance := -1

	for _, candidate := range candidates {
		normCandidate := levenshteinNormalizeString(candidate)
		if _, ok := triedCandidates[normCandidate]; ok {
			continue
		}

		distance := levenshtein.ComputeDistance(normTarget, normCandidate)
		if minDistance == -1 || distance < minDistance {
			minDistance = distance
		}

		triedCandidates[normCandidate] = struct{}{}
	}

	return minDistance
}

func levenshteinNormalizeString(str string) string {
	return regex.NormalizeString(unidecode.Unidecode(str))
}

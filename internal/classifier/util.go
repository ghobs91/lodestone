package classifier

import (
	"sync"

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

	normTarget := levenshteinNormalizeString(target)

	for i, item := range items {
		candidates := getCandidates(item)

		distance := levenshteinFindMinDistanceNorm(normTarget, candidates)
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

// levenshteinFindMinDistance is the public entry point; it normalizes the target
// and delegates to the normalized variant.
func levenshteinFindMinDistance(target string, candidates []string) int {
	normTarget := levenshteinNormalizeString(target)
	return levenshteinFindMinDistanceNorm(normTarget, candidates)
}

func levenshteinFindMinDistanceNorm(normTarget string, candidates []string) int {
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

// levenshteinCache is a small LRU-ish cache for normalized strings.  Since
// the same titles appear repeatedly during classification batches, caching
// the (expensive) unidecode + regex normalization avoids redundant work.
var levenshteinCache = &levenshteinNormCache{
	entries: make(map[string]string),
}

type levenshteinNormCache struct {
	mu      sync.RWMutex
	entries map[string]string
}

func (c *levenshteinNormCache) get(raw string) string {
	c.mu.RLock()
	norm, ok := c.entries[raw]
	c.mu.RUnlock()
	if ok {
		return norm
	}

	norm = regex.NormalizeString(unidecode.Unidecode(raw))

	c.mu.Lock()
	// Cap the cache at a reasonable size so it doesn't grow unbounded
	// during long-running classification sweeps.
	if len(c.entries) < 10_000 {
		c.entries[raw] = norm
	}
	c.mu.Unlock()

	return norm
}

func levenshteinNormalizeString(str string) string {
	return levenshteinCache.get(str)
}

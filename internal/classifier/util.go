package classifier

import (
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/agnivade/levenshtein"
	"github.com/ghobs91/lodestone/internal/regex"
	"github.com/mozillazg/go-unidecode"
)

// levenshteinMaxThreshold scales the acceptable edit distance with the target
// title length: 15% of the normalized title length, clamped to [2, 8].
// Short titles (e.g. "Her") get a tight threshold of 2, while long titles
// (e.g. "The Lord of the Rings: The Return of the King") get up to 8.
func levenshteinMaxThreshold(target string) int {
	return levenshteinThresholdForNormalized(levenshteinNormalizeString(target))
}

// levenshteinThresholdForNormalized computes the threshold from an already
// normalized target, avoiding a second normalization/cache lookup.
func levenshteinThresholdForNormalized(normTarget string) int {
	n := len(normTarget)
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
	normTarget := levenshteinNormalizeString(target)
	threshold := levenshteinThresholdForNormalized(normTarget)
	minDistance := threshold + 1
	bestMatch := -1

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
		if distance == 0 {
			return 0
		}
		if minDistance == -1 || distance < minDistance {
			minDistance = distance
		}

		triedCandidates[normCandidate] = struct{}{}
	}

	return minDistance
}

// levenshteinCacheSize bounds the normalization cache. It holds both the
// (rare) target titles and the (many) candidate titles seen during a batch.
const levenshteinCacheSize = 10_000

// levenshteinCache caches normalized strings. The same titles appear repeatedly
// during classification batches, so caching the (expensive) unidecode + regex
// normalization avoids redundant work. An LRU with O(1) eviction is used so a
// full cache never triggers an expensive scan under the lock (which previously
// thrashed the cache on nearly every miss once it filled up).
var levenshteinCache = newLevenshteinNormCache(levenshteinCacheSize)

type levenshteinNormCache struct {
	lru *lru.Cache[string, string]
}

func newLevenshteinNormCache(size int) *levenshteinNormCache {
	c, err := lru.New[string, string](size)
	if err != nil {
		// Only fails for a non-positive size, which is a programming error.
		panic(err)
	}

	return &levenshteinNormCache{lru: c}
}

func (c *levenshteinNormCache) get(raw string) string {
	if norm, ok := c.lru.Get(raw); ok {
		return norm
	}

	norm := regex.NormalizeString(unidecode.Unidecode(raw))
	c.lru.Add(raw, norm)

	return norm
}

func levenshteinNormalizeString(str string) string {
	return levenshteinCache.get(str)
}

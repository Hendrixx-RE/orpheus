package pm

import (
	"sort"
	"strings"
	"unicode"
)

// FuzzyScore calculates a relevance score for matching query against a package name & description.
// Higher scores indicate better matches.
func FuzzyScore(query, name, description string) int {
	q := strings.ToLower(strings.TrimSpace(query))
	n := strings.ToLower(strings.TrimSpace(name))
	d := strings.ToLower(strings.TrimSpace(description))

	if q == "" || n == "" {
		return 0
	}

	// 1. Exact name match -> highest rank
	if n == q {
		return 10000000
	}

	// 2. Exact word match in package name (words delimited by -, _, /, ., space)
	nameWords := strings.FieldsFunc(n, func(r rune) bool {
		return r == '-' || r == '_' || r == '/' || r == '.' || r == ' '
	})
	for _, w := range nameWords {
		if w == q {
			// Shorter package names rank higher among word matches
			return 5000000 - (len(n)-len(q))*100
		}
	}

	// 3. Exact prefix match in package name
	if strings.HasPrefix(n, q) {
		return 2000000 - (len(n)-len(q))*100
	}

	// 4. Multi-word token match (e.g. "visual studio" matching "visual-studio-code")
	qTokens := strings.Fields(q)
	if len(qTokens) > 1 {
		allTokensMatch := true
		tokenScore := 0
		for _, tok := range qTokens {
			if strings.Contains(n, tok) {
				tokenScore += 100000
			} else if strings.Contains(d, tok) {
				tokenScore += 10000
			} else {
				allTokensMatch = false
				break
			}
		}
		if allTokensMatch {
			return 3000000 + tokenScore - len(n)*10
		}
	}

	// 5. Substring match in package name
	if idx := strings.Index(n, q); idx >= 0 {
		score := 1000000 - (len(n)-len(q))*100
		// Bonus if match occurs at start of a word boundary
		if idx == 0 || n[idx-1] == '-' || n[idx-1] == '_' || n[idx-1] == '/' || n[idx-1] == '.' {
			score += 200000
		}
		return score
	}

	// 6. Subsequence fuzzy match in package name
	if fScore, matched := scoreSubsequence(q, n); matched {
		return 500000 + fScore
	}

	// 7. Match in description
	if strings.Contains(d, q) {
		return 50000 - len(d)*5
	}
	if len(qTokens) > 1 {
		matchedCount := 0
		for _, tok := range qTokens {
			if strings.Contains(d, tok) {
				matchedCount++
			}
		}
		if matchedCount > 0 {
			return 10000 * matchedCount
		}
	}
	if fScore, matched := scoreSubsequence(q, d); matched {
		return 1000 + fScore
	}

	return 0
}

// scoreSubsequence checks if query is a subsequence of target and returns a score based on match quality.
func scoreSubsequence(query, target string) (int, bool) {
	qRunes := []rune(query)
	tRunes := []rune(target)

	if len(qRunes) > len(tRunes) {
		return 0, false
	}

	qIdx := 0
	firstMatchIdx := -1
	lastMatchIdx := -1
	consecutiveMatches := 0
	maxConsecutive := 0
	boundaryBonus := 0

	for tIdx, tr := range tRunes {
		if qIdx < len(qRunes) && tr == qRunes[qIdx] {
			if firstMatchIdx == -1 {
				firstMatchIdx = tIdx
			}
			lastMatchIdx = tIdx

			// Check word boundary
			if tIdx == 0 || isBoundaryRune(tRunes[tIdx-1]) {
				boundaryBonus += 5000
			}

			if qIdx > 0 && tIdx > 0 && tRunes[tIdx-1] == qRunes[qIdx-1] {
				consecutiveMatches++
				if consecutiveMatches > maxConsecutive {
					maxConsecutive = consecutiveMatches
				}
			} else {
				consecutiveMatches = 1
			}

			qIdx++
		}
	}

	if qIdx < len(qRunes) {
		return 0, false // Not all query characters were found
	}

	span := lastMatchIdx - firstMatchIdx + 1
	gapPenalty := (span - len(qRunes)) * 500
	lengthPenalty := len(tRunes) * 50

	score := 10000 + (maxConsecutive * 3000) + boundaryBonus - gapPenalty - lengthPenalty
	if score < 1 {
		score = 1
	}
	return score, true
}

func isBoundaryRune(r rune) bool {
	return r == '-' || r == '_' || r == '/' || r == '.' || r == ' ' || unicode.IsUpper(r)
}

// RankSearchResults sorts packages by relevance to query (best matches first).
func RankSearchResults(query string, pkgs []Package) []Package {
	if len(pkgs) <= 1 || strings.TrimSpace(query) == "" {
		return pkgs
	}

	type scoredPkg struct {
		pkg      Package
		score    int
		repoRank int
	}

	scored := make([]scoredPkg, len(pkgs))
	for i, p := range pkgs {
		score := FuzzyScore(query, p.Name, p.Description)

		// Determine repository priority (core > extra > multilib > aur/other)
		repoRank := 4
		descLower := strings.ToLower(p.Description)
		if strings.HasPrefix(descLower, "[core]") {
			repoRank = 1
		} else if strings.HasPrefix(descLower, "[extra]") {
			repoRank = 2
		} else if strings.HasPrefix(descLower, "[multilib]") {
			repoRank = 3
		}

		scored[i] = scoredPkg{pkg: p, score: score, repoRank: repoRank}
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score // higher score first
		}
		if scored[i].repoRank != scored[j].repoRank {
			return scored[i].repoRank < scored[j].repoRank // core/extra before aur
		}
		return len(scored[i].pkg.Name) < len(scored[j].pkg.Name) // shorter name first
	})

	result := make([]Package, len(pkgs))
	for i, s := range scored {
		result[i] = s.pkg
	}
	return result
}

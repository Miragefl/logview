package buffer

import (
	"sort"
	"strings"
)

type SearchIndex struct {
	index map[string]map[int]struct{}
}

func NewSearchIndex() *SearchIndex {
	return &SearchIndex{
		index: make(map[string]map[int]struct{}),
	}
}

func (si *SearchIndex) Add(pos int, text string) {
	tokens := tokenize(text)
	for _, tok := range tokens {
		if _, ok := si.index[tok]; !ok {
			si.index[tok] = make(map[int]struct{})
		}
		si.index[tok][pos] = struct{}{}
	}
}

func (si *SearchIndex) Search(query string) []int {
	words := tokenize(query)
	if len(words) == 0 {
		return nil
	}

	var result map[int]struct{}
	for _, w := range words {
		positions := make(map[int]struct{})
		for tok, posSet := range si.index {
			if strings.Contains(tok, w) {
				for p := range posSet {
					positions[p] = struct{}{}
				}
			}
		}
		if result == nil {
			result = positions
		} else {
			for p := range result {
				if _, ok := positions[p]; !ok {
					delete(result, p)
				}
			}
		}
		if len(result) == 0 {
			return nil
		}
	}

	hits := make([]int, 0, len(result))
	for p := range result {
		hits = append(hits, p)
	}
	sort.Ints(hits)
	return hits
}

func (si *SearchIndex) Clear() {
	si.index = make(map[string]map[int]struct{})
}

func tokenize(text string) []string {
	return strings.Fields(strings.ToLower(text))
}
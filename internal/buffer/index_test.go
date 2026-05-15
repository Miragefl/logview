package buffer

import "testing"

func TestSearchIndexBasicMatch(t *testing.T) {
	idx := NewSearchIndex()
	idx.Add(0, "hello world")
	idx.Add(1, "foo bar")
	idx.Add(2, "say hello again")

	hits := idx.Search("hello")
	if len(hits) != 2 {
		t.Fatalf("Search('hello') returned %d hits, want 2", len(hits))
	}
	if hits[0] != 0 || hits[1] != 2 {
		t.Errorf("hits = %v, want [0, 2]", hits)
	}
}

func TestSearchIndexNoMatch(t *testing.T) {
	idx := NewSearchIndex()
	idx.Add(0, "hello world")
	hits := idx.Search("xyz")
	if len(hits) != 0 {
		t.Errorf("Search('xyz') returned %d hits, want 0", len(hits))
	}
}

func TestSearchIndexCaseInsensitive(t *testing.T) {
	idx := NewSearchIndex()
	idx.Add(0, "Hello World")
	hits := idx.Search("hello")
	if len(hits) != 1 {
		t.Fatalf("case-insensitive search returned %d hits, want 1", len(hits))
	}
}

func TestSearchIndexClear(t *testing.T) {
	idx := NewSearchIndex()
	idx.Add(0, "hello")
	idx.Clear()
	hits := idx.Search("hello")
	if len(hits) != 0 {
		t.Errorf("after Clear, Search returned %d hits, want 0", len(hits))
	}
}
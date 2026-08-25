package book

import (
	"testing"
)

// mark returns a Mark with back-pointers wired to the given shelf and collection.
func testMark(s *Shelf, c *Collection, id, name, url string, tags []string) *Mark {
	return &Mark{
		Shelf:      s,
		Collection: c,
		ID:         id,
		Name:       name,
		URL:        url,
		Tags:       tags,
	}
}

// fixture builds a BookShelves with a single shelf and collection holding marks.
func fixture(t *testing.T, marks ...*Mark) BookShelves {
	t.Helper()
	shelf := &Shelf{
		ID:   "s1",
		Name: "work",
		Collections: map[string]*Collection{
			"dev": {ID: "c1", Name: "dev"},
		},
	}
	col := shelf.Collections["dev"]
	for _, m := range marks {
		m.Shelf = shelf
		m.Collection = col
		col.Marks = append(col.Marks, m)
	}
	return BookShelves{*shelf}
}

func TestDetectDuplicates(t *testing.T) {
	t.Run("no duplicates", func(t *testing.T) {
		bs := fixture(t,
			testMark(nil, nil, "a", "one", "https://a", []string{"x"}),
			testMark(nil, nil, "b", "two", "https://b", []string{"y"}),
		)
		if got := bs.DetectDuplicates(); len(got) != 0 {
			t.Fatalf("DetectDuplicates() = %v, want none", got)
		}
	})

	t.Run("identical duplicate is not a conflict", func(t *testing.T) {
		bs := fixture(t,
			testMark(nil, nil, "a", "one", "https://a", []string{"x"}),
			testMark(nil, nil, "a", "one", "https://a", []string{"x"}),
		)
		got := bs.DetectDuplicates()
		if len(got) != 1 {
			t.Fatalf("got %d conflicts, want 1", len(got))
		}
		if got[0].TrueConflict {
			t.Error("identical marks flagged as conflict")
		}
		if len(got[0].Marks) != 2 {
			t.Errorf("group has %d marks, want 2", len(got[0].Marks))
		}
	})

	t.Run("differing tags is a conflict", func(t *testing.T) {
		bs := fixture(t,
			testMark(nil, nil, "a", "one", "https://a", []string{"x"}),
			testMark(nil, nil, "a", "one", "https://a", []string{"y"}),
		)
		got := bs.DetectDuplicates()
		if len(got) != 1 || !got[0].TrueConflict {
			t.Fatalf("DetectDuplicates() = %v, want 1 true conflict", got)
		}
	})

	t.Run("differing title is a conflict", func(t *testing.T) {
		bs := fixture(t,
			testMark(nil, nil, "a", "one", "https://a", []string{"x"}),
			testMark(nil, nil, "a", "two", "https://a", []string{"x"}),
		)
		got := bs.DetectDuplicates()
		if len(got) != 1 || !got[0].TrueConflict {
			t.Fatalf("DetectDuplicates() = %v, want 1 true conflict", got)
		}
	})

	t.Run("differing deleted state is a conflict", func(t *testing.T) {
		active := testMark(nil, nil, "a", "one", "https://a", []string{"x"})
		deleted := testMark(nil, nil, "a", "one", "https://a", []string{"x"})
		deleted.DeletedAt = "2026-08-01T00:00:00Z"
		bs := fixture(t, active, deleted)
		got := bs.DetectDuplicates()
		if len(got) != 1 || !got[0].TrueConflict {
			t.Fatalf("DetectDuplicates() = %v, want 1 true conflict", got)
		}
	})

	t.Run("derives ID from URL for v1 marks", func(t *testing.T) {
		bs := fixture(t,
			testMark(nil, nil, "", "one", "https://a", nil),
			testMark(nil, nil, "", "one", "https://a", nil),
		)
		got := bs.DetectDuplicates()
		if len(got) != 1 {
			t.Fatalf("got %d conflicts, want 1", len(got))
		}
		if got[0].ID != GenerateID("https://a") {
			t.Errorf("ID = %q, want %q", got[0].ID, GenerateID("https://a"))
		}
	})

	t.Run("tags compared as sets", func(t *testing.T) {
		bs := fixture(t,
			testMark(nil, nil, "a", "one", "https://a", []string{"x", "y"}),
			testMark(nil, nil, "a", "one", "https://a", []string{"y", "x"}),
		)
		got := bs.DetectDuplicates()
		if len(got) != 1 || got[0].TrueConflict {
			t.Fatalf("DetectDuplicates() = %v, want 1 non-conflict", got)
		}
	})
}

func TestResolveDuplicates(t *testing.T) {
	t.Run("removes identical duplicates and keeps first", func(t *testing.T) {
		bs := fixture(t,
			testMark(nil, nil, "a", "one", "https://a", []string{"x"}),
			testMark(nil, nil, "a", "one", "https://a", []string{"x"}),
			testMark(nil, nil, "b", "two", "https://b", []string{"y"}),
		)
		removed, changed := bs.ResolveDuplicates()
		if removed != 1 {
			t.Errorf("removed = %d, want 1", removed)
		}
		if len(changed) != 1 {
			t.Fatalf("changed shelves = %d, want 1", len(changed))
		}
		col := bs[0].Collections["dev"]
		if len(col.Marks) != 2 {
			t.Fatalf("marks after resolve = %d, want 2", len(col.Marks))
		}
		if col.Marks[0].ID != "a" || col.Marks[1].ID != "b" {
			t.Errorf("marks = %q, %q; want a then b", col.Marks[0].ID, col.Marks[1].ID)
		}
	})

	t.Run("leaves true conflicts untouched", func(t *testing.T) {
		bs := fixture(t,
			testMark(nil, nil, "a", "one", "https://a", []string{"x"}),
			testMark(nil, nil, "a", "two", "https://a", []string{"x"}),
		)
		removed, changed := bs.ResolveDuplicates()
		if removed != 0 || len(changed) != 0 {
			t.Fatalf("removed=%d changed=%d, want 0/0", removed, len(changed))
		}
		col := bs[0].Collections["dev"]
		if len(col.Marks) != 2 {
			t.Fatalf("marks after resolve = %d, want 2", len(col.Marks))
		}
	})

	t.Run("duplicate across collections dedupes second occurrence", func(t *testing.T) {
		shelf := &Shelf{
			ID:   "s1",
			Name: "work",
			Collections: map[string]*Collection{
				"dev":  {ID: "c1", Name: "dev"},
				"docs": {ID: "c2", Name: "docs"},
			},
		}
		dev := shelf.Collections["dev"]
		docs := shelf.Collections["docs"]
		m1 := testMark(shelf, dev, "a", "one", "https://a", []string{"x"})
		m2 := testMark(shelf, docs, "a", "one", "https://a", []string{"x"})
		dev.Marks = []*Mark{m1}
		docs.Marks = []*Mark{m2}
		bs := BookShelves{*shelf}

		removed, _ := bs.ResolveDuplicates()
		if removed != 1 {
			t.Fatalf("removed = %d, want 1", removed)
		}
		if len(docs.Marks) != 0 {
			t.Errorf("docs marks = %d, want 0", len(docs.Marks))
		}
		if len(dev.Marks) != 1 {
			t.Errorf("dev marks = %d, want 1", len(dev.Marks))
		}
	})
}

func TestEqualTags(t *testing.T) {
	if !equalTags([]string{"a", "b"}, []string{"b", "a"}) {
		t.Error("equalTags should treat order-insensitive sets as equal")
	}
	if equalTags([]string{"a"}, []string{"a", "b"}) {
		t.Error("equalTags should reject different lengths")
	}
	if equalTags(nil, []string{"a"}) {
		t.Error("equalTags should reject nil vs non-nil")
	}
}

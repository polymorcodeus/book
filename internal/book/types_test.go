package book

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDedupUnique(t *testing.T) {
	tests := []struct {
		name string
		in   [][]string
		want []string
	}{
		{
			name: "preserves first-seen order",
			in:   [][]string{{"a", "b", "c"}, {"b", "a", "d"}},
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "merges multiple slices",
			in:   [][]string{{"x"}, {"y"}, {"z"}, {"x"}},
			want: []string{"x", "y", "z"},
		},
		{
			name: "empty input",
			in:   [][]string{},
			want: []string{},
		},
		{
			name: "empty slices are ignored",
			in:   [][]string{{}, {"a"}, {}, {"a", "b"}},
			want: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DedupUnique(tt.in...)
			if !slices.Equal(got, tt.want) {
				t.Errorf("DedupUnique() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBookShelvesShelf(t *testing.T) {
	bs := BookShelves{
		{Name: "shelf-a"},
	}

	if shelf, ok := bs.Shelf("shelf-a"); !ok || shelf == nil || shelf.Name != "shelf-a" {
		t.Errorf("expected shelf-a, got %v, ok=%v", shelf, ok)
	}

	if shelf, ok := bs.Shelf("missing"); ok || shelf != nil {
		t.Errorf("expected missing shelf to return nil/false, got %v, ok=%v", shelf, ok)
	}
}

func TestVerifyUniqueURL(t *testing.T) {
	bs := BookShelves{
		{
			Name: "shelf-a",
			Collections: map[string]*Collection{
				"col-1": {
					Name: "col-1",
					Marks: []*Mark{
						{ID: "abc12345", Name: "first", URL: "https://example.com/first"},
						{ID: "aabbccdd", Name: "trashed", URL: "https://example.com/trashed", DeletedAt: "2026-08-01T00:00:00Z"},
					},
				},
			},
		},
		{
			Name: "shelf-b",
			Collections: map[string]*Collection{
				"col-2": {
					Name: "col-2",
					Marks: []*Mark{
						{ID: "def67890", Name: "second", URL: "https://example.com/second"},
					},
				},
			},
		},
	}
	bs.LoadParents()

	tests := []struct {
		name        string
		id          string
		wantErr     bool
		wantRestore bool
	}{
		{
			name:    "unique id passes",
			id:      "00000000",
			wantErr: false,
		},
		{
			name:    "duplicate in first shelf",
			id:      "abc12345",
			wantErr: true,
		},
		{
			name:    "duplicate in second shelf",
			id:      "def67890",
			wantErr: true,
		},
		{
			name:        "trashed collision suggests restore",
			id:          "aabbccdd",
			wantErr:     true,
			wantRestore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bs.VerifyUniqueURL(tt.id, nil)
			if tt.wantErr && err == nil {
				t.Errorf("VerifyUniqueURL(%q) expected error, got nil", tt.id)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("VerifyUniqueURL(%q) unexpected error: %v", tt.id, err)
			}
			if tt.wantRestore && (err == nil || !strings.Contains(err.Error(), "restore")) {
				t.Errorf("VerifyUniqueURL(%q) error = %v, want restore hint", tt.id, err)
			}
		})
	}
}

func TestAllTags(t *testing.T) {
	tests := []struct {
		name string
		col  *Collection
		want []string
	}{
		{
			name: "sorts and merges across marks",
			col: &Collection{
				Marks: []*Mark{
					{Tags: []string{"z", "a"}},
					{Tags: []string{"b", "a"}},
				},
			},
			want: []string{"a", "a", "b", "z"},
		},
		{
			name: "empty collection",
			col:  &Collection{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.col.AllTags()
			if !slices.Equal(got, tt.want) {
				t.Errorf("AllTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteMark(t *testing.T) {
	markA := &Mark{Name: "a"}
	markB := &Mark{Name: "b"}

	col := &Collection{Marks: []*Mark{markA, markB}}
	col.DeleteMark(markB)

	if len(col.Marks) != 2 {
		t.Fatalf("len(Marks) = %d, want 2 (soft delete retains the mark)", len(col.Marks))
	}
	if markB.DeletedAt == "" {
		t.Errorf("DeleteMark did not stamp DeletedAt")
	}
	if _, err := time.Parse(time.RFC3339, markB.DeletedAt); err != nil {
		t.Errorf("DeleteMark DeletedAt = %q, not RFC3339: %v", markB.DeletedAt, err)
	}
	if markA.DeletedAt != "" {
		t.Errorf("DeleteMark stamped the wrong mark")
	}

	// Deleting again is a no-op that preserves the original timestamp.
	first := markB.DeletedAt
	col.DeleteMark(markB)
	if markB.DeletedAt != first {
		t.Errorf("DeleteMark not idempotent: %q -> %q", first, markB.DeletedAt)
	}

	// Deleting a nil mark is a no-op.
	col.DeleteMark(nil)
	if len(col.Marks) != 2 {
		t.Errorf("len(Marks) = %d, want 2 after nil delete", len(col.Marks))
	}
}

func TestIsDeleted(t *testing.T) {
	if (&Mark{}).IsDeleted() {
		t.Errorf("empty mark reported deleted")
	}
	if !(&Mark{DeletedAt: "x"}).IsDeleted() {
		t.Errorf("mark with DeletedAt reported not deleted")
	}
}

func TestMarksNamesExcludesDeleted(t *testing.T) {
	col := &Collection{Marks: []*Mark{
		{Name: "a"},
		{Name: "b", DeletedAt: "x"},
		{Name: "c"},
	}}
	want := []string{"a", "c"}
	got := col.MarksNames()
	if !slices.Equal(got, want) {
		t.Errorf("MarksNames() = %v, want %v", got, want)
	}
}

func TestMarkSkipsDeleted(t *testing.T) {
	col := &Collection{Marks: []*Mark{
		{Name: "a", DeletedAt: "x"},
		{Name: "a"},
	}}
	got := col.Mark("a")
	if got == nil || got.DeletedAt != "" {
		t.Errorf("Mark() returned a soft-deleted mark or nil: %+v", got)
	}
	if col.Mark("missing") != nil {
		t.Errorf("Mark() returned a mark for an unknown name")
	}
}

func TestAllTagsExcludesDeleted(t *testing.T) {
	col := &Collection{Marks: []*Mark{
		{Tags: []string{"z", "a"}},
		{Tags: []string{"deleted-only"}, DeletedAt: "x"},
		{Tags: []string{"b", "a"}},
	}}
	want := []string{"a", "a", "b", "z"}
	got := col.AllTags()
	if !slices.Equal(got, want) {
		t.Errorf("AllTags() = %v, want %v", got, want)
	}
}

func TestPurgeDeletedMarks(t *testing.T) {
	cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)
	old := cutoff.Add(-24 * time.Hour).Format(time.RFC3339)
	recent := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)

	tests := []struct {
		name string
		col  *Collection
		want int
	}{
		{
			name: "purges only old soft-deleted marks",
			col: &Collection{Marks: []*Mark{
				{Name: "active"},
				{Name: "old", DeletedAt: old},
				{Name: "recent", DeletedAt: recent},
				{Name: "bad", DeletedAt: "not-a-timestamp"},
			}},
			want: 1,
		},
		{
			name: "no soft-deleted marks",
			col: &Collection{Marks: []*Mark{
				{Name: "active"},
			}},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.col.PurgeDeletedMarks(cutoff)
			if got != tt.want {
				t.Fatalf("PurgeDeletedMarks() = %d, want %d", got, tt.want)
			}
		})
	}

	// Verify the first case retains the right marks.
	col := &Collection{Marks: []*Mark{
		{Name: "active"},
		{Name: "old", DeletedAt: old},
		{Name: "recent", DeletedAt: recent},
		{Name: "bad", DeletedAt: "not-a-timestamp"},
	}}
	col.PurgeDeletedMarks(cutoff)
	var remaining []string
	for _, m := range col.Marks {
		remaining = append(remaining, m.Name)
	}
	want := []string{"active", "recent", "bad"}
	if !slices.Equal(remaining, want) {
		t.Errorf("remaining marks = %v, want %v", remaining, want)
	}
}

func TestSoftDeletedByID(t *testing.T) {
	bs := BookShelves{
		{
			Name: "shelf-a",
			Collections: map[string]*Collection{
				"col-1": {
					Name: "col-1",
					Marks: []*Mark{
						{ID: "abc12345", Name: "active", URL: "https://example.com"},
						{ID: "aabbccdd", Name: "trashed", URL: "https://example.com/trashed", DeletedAt: "2026-08-01T00:00:00Z"},
					},
				},
			},
		},
	}

	if got := bs.SoftDeletedByID("aabbccdd"); got == nil || got.Name != "trashed" {
		t.Errorf("SoftDeletedByID(trashed) = %+v, want trashed mark", got)
	}
	if got := bs.SoftDeletedByID("abc12345"); got != nil {
		t.Errorf("SoftDeletedByID(active) = %+v, want nil", got)
	}
	if got := bs.SoftDeletedByID("missing"); got != nil {
		t.Errorf("SoftDeletedByID(missing) = %+v, want nil", got)
	}
}

func TestGenerateID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "known URL golden value",
			url:  "https://example.com",
			want: "100680ad",
		},
		{
			name: "different URL different id",
			url:  "https://example.org",
			want: "50d7a905",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateID(tt.url)
			if len(got) != 8 {
				t.Errorf("GenerateID() length = %d, want 8", len(got))
			}
			if got != tt.want {
				t.Errorf("GenerateID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateShelfID(t *testing.T) {
	name := "my shelf"
	want := GenerateID(name)
	got := GenerateShelfID(name)
	if got != want {
		t.Errorf("GenerateShelfID(%q) = %q, want %q", name, got, want)
	}
}

func TestGenerateCollectionID(t *testing.T) {
	shelfName := "my shelf"
	collectionName := "my collection"
	want := GenerateID(shelfName + "/" + collectionName)
	got := GenerateCollectionID(shelfName, collectionName)
	if got != want {
		t.Errorf("GenerateCollectionID(%q, %q) = %q, want %q", shelfName, collectionName, got, want)
	}
}

func TestNowTimestamp(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	got := NowTimestamp()
	after := time.Now().UTC().Add(time.Second)

	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("NowTimestamp() returned unparseable value %q: %v", got, err)
	}
	if parsed.Before(before) || parsed.After(after) {
		t.Errorf("NowTimestamp() = %q, not within [%s, %s]", got, before.Format(time.RFC3339), after.Format(time.RFC3339))
	}
	if !strings.HasSuffix(got, "Z") {
		t.Errorf("NowTimestamp() = %q, expected UTC suffix 'Z'", got)
	}
}

func TestShelfIsV2(t *testing.T) {
	cases := []struct {
		name   string
		shelf  Shelf
		wantV2 bool
	}{
		{"nil schema version", Shelf{}, false},
		{"v1 schema version", Shelf{SchemaVersion: IntPtr(1)}, false},
		{"v2 schema version", Shelf{SchemaVersion: IntPtr(2)}, true},
		{"v3 schema version", Shelf{SchemaVersion: IntPtr(3)}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.shelf.IsV2(); got != tc.wantV2 {
				t.Errorf("IsV2() = %t, want %t", got, tc.wantV2)
			}
		})
	}
}

func TestMergeTags(t *testing.T) {
	tests := []struct {
		name string
		in   [][]string
		want []string
	}{
		{
			name: "dedups and removes empty strings",
			in:   [][]string{{"a", "", "b"}, {"", "b", "c"}},
			want: []string{"a", "b", "c"},
		},
		{
			name: "earlier arguments have priority",
			in:   [][]string{{"z", "a"}, {"a", "b"}},
			want: []string{"z", "a", "b"},
		},
		{
			name: "empty input",
			in:   [][]string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeTags(tt.in...)
			if !slices.Equal(got, tt.want) {
				t.Errorf("MergeTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseTagFilter(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    [][]string
		wantErr bool
	}{
		{name: "blank", in: "", want: nil},
		{name: "whitespace", in: "   ", want: nil},
		{name: "single tag", in: "a", want: [][]string{{"a"}}},
		{name: "or", in: "a,b", want: [][]string{{"a", "b"}}},
		{name: "and", in: "a+b", want: [][]string{{"a"}, {"b"}}},
		{name: "and of ors", in: "a,b+c", want: [][]string{{"a", "b"}, {"c"}}},
		{name: "trims spaces", in: "a, b + c", want: [][]string{{"a", "b"}, {"c"}}},
		{name: "trailing comma", in: "a,", wantErr: true},
		{name: "leading comma", in: ",a", wantErr: true},
		{name: "trailing plus", in: "a+", wantErr: true},
		{name: "leading plus", in: "+a", wantErr: true},
		{name: "double plus", in: "a++b", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTagFilter(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseTagFilter(%q) = %v, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTagFilter(%q) error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTagFilter(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSplitTags(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: nil},
		{name: "whitespace", in: "   ", want: nil},
		{name: "single", in: "go", want: []string{"go"}},
		{name: "comma separated", in: "go,cli,tui", want: []string{"go", "cli", "tui"}},
		{name: "trimmed", in: " go , cli ,", want: []string{"go", "cli"}},
		{name: "empty parts skipped", in: "go,,cli", want: []string{"go", "cli"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitTags(tt.in)
			if !slices.Equal(got, tt.want) {
				t.Errorf("SplitTags(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSplitTagLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: nil},
		{name: "single", in: "go", want: []string{"go"}},
		{name: "newlines", in: "go\ncli\ntui", want: []string{"go", "cli", "tui"}},
		{name: "extra spaces", in: "  go   cli  ", want: []string{"go", "cli"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitTagLines(tt.in)
			if !slices.Equal(got, tt.want) {
				t.Errorf("SplitTagLines(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "http", in: "http://example.com", wantErr: false},
		{name: "https", in: "https://example.com/path", wantErr: false},
		{name: "empty", in: "", wantErr: true},
		{name: "plain text", in: "not-a-url", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %t", tt.in, err, tt.wantErr)
			}
		})
	}
}

func TestNewMarkFromInput(t *testing.T) {
	m, err := NewMarkFromInput("https://example.com", []string{"go", "cli"})
	if err != nil {
		t.Fatalf("NewMarkFromInput error: %v", err)
	}
	if m.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", m.URL, "https://example.com")
	}
	if m.ID != GenerateID("https://example.com") {
		t.Errorf("ID = %q, want %q", m.ID, GenerateID("https://example.com"))
	}
	if !slices.Equal(m.Tags, []string{"go", "cli"}) {
		t.Errorf("Tags = %v, want %v", m.Tags, []string{"go", "cli"})
	}

	if _, err := NewMarkFromInput("not-a-url", nil); err == nil {
		t.Error("NewMarkFromInput with invalid URL expected error")
	}
}

func TestResolveMarkTitle(t *testing.T) {
	tests := []struct {
		name        string
		provided    string
		fetched     TitleFetchResult
		interactive bool
		want        string
		wantErr     bool
	}{
		{name: "provided wins", provided: "My Title", fetched: TitleFetchResult{Title: "Fetched"}, interactive: false, want: "My Title"},
		{name: "fetched used", provided: "", fetched: TitleFetchResult{Title: "Fetched"}, interactive: false, want: "Fetched"},
		{name: "unavailable interactive", provided: "", fetched: TitleFetchResult{Unavailable: true}, interactive: true, want: ""},
		{name: "unavailable non-interactive", provided: "", fetched: TitleFetchResult{Unavailable: true}, interactive: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveMarkTitle(tt.provided, "https://example.com", tt.fetched, tt.interactive)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveMarkTitle() error = %v, wantErr %t", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ResolveMarkTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateNewShelfName(t *testing.T) {
	bs := BookShelves{{Name: "existing"}}

	if err := bs.ValidateNewShelfName("new"); err != nil {
		t.Errorf("ValidateNewShelfName(\"new\") error = %v", err)
	}
	if err := bs.ValidateNewShelfName(""); err == nil {
		t.Error("ValidateNewShelfName(\"\") expected error")
	}
	if err := bs.ValidateNewShelfName("existing"); err == nil {
		t.Error("ValidateNewShelfName(\"existing\") expected error")
	}
}

func TestNewShelf(t *testing.T) {
	if _, err := NewShelf("", ""); err == nil {
		t.Error("NewShelf with empty name expected error")
	}

	s, err := NewShelf("test-shelf", "a description")
	if err != nil {
		t.Fatalf("NewShelf error: %v", err)
	}
	if s.Name != "test-shelf" {
		t.Errorf("Name = %q, want %q", s.Name, "test-shelf")
	}
	if s.Description != "a description" {
		t.Errorf("Description = %q, want %q", s.Description, "a description")
	}
	if s.ID != GenerateShelfID("test-shelf") {
		t.Errorf("ID = %q, want %q", s.ID, GenerateShelfID("test-shelf"))
	}
	if s.SchemaVersion == nil || *s.SchemaVersion != 2 {
		t.Errorf("SchemaVersion = %v, want 2", s.SchemaVersion)
	}
	if s.Collections == nil {
		t.Error("Collections map not initialized")
	}
	if s.CreatedAt == "" || s.UpdatedAt == "" {
		t.Error("timestamps not set")
	}
}

func TestBookShelvesRemoveShelf(t *testing.T) {
	bs := BookShelves{
		{Name: "keep"},
		{Name: "remove"},
	}

	removed, err := bs.RemoveShelf("remove")
	if err != nil {
		t.Fatalf("RemoveShelf error: %v", err)
	}
	if removed.Name != "remove" {
		t.Errorf("removed.Name = %q, want %q", removed.Name, "remove")
	}
	if len(bs) != 1 || bs[0].Name != "keep" {
		t.Errorf("after remove bs = %v, want one shelf named keep", bs.ShelfNames())
	}

	if _, err := bs.RemoveShelf("missing"); err == nil {
		t.Error("RemoveShelf missing expected error")
	}
}

func TestNewCollection(t *testing.T) {
	if _, err := NewCollection(nil, "name", ""); err == nil {
		t.Error("NewCollection with nil shelf expected error")
	}
	if _, err := NewCollection(&Shelf{Name: "s"}, "", ""); err == nil {
		t.Error("NewCollection with empty name expected error")
	}

	shelf := &Shelf{Name: "shelf-a"}
	c, err := NewCollection(shelf, "col-a", "desc")
	if err != nil {
		t.Fatalf("NewCollection error: %v", err)
	}
	if c.Name != "col-a" {
		t.Errorf("Name = %q, want %q", c.Name, "col-a")
	}
	if c.Shelf != shelf {
		t.Error("Shelf back-pointer not set")
	}
	if c.ID != GenerateCollectionID("shelf-a", "col-a") {
		t.Errorf("ID = %q, want %q", c.ID, GenerateCollectionID("shelf-a", "col-a"))
	}
}

func TestShelfRemoveCollection(t *testing.T) {
	s := &Shelf{
		Collections: map[string]*Collection{
			"keep":    {Name: "keep"},
			"discard": {Name: "discard"},
		},
	}

	if err := s.RemoveCollection("discard"); err != nil {
		t.Fatalf("RemoveCollection error: %v", err)
	}
	if _, ok := s.Collections["discard"]; ok {
		t.Error("discard collection still present")
	}

	if err := s.RemoveCollection("missing"); err == nil {
		t.Error("RemoveCollection missing expected error")
	}
}

func TestBookShelvesFindMarkByID(t *testing.T) {
	bs := BookShelves{
		{
			Name: "shelf-a",
			Collections: map[string]*Collection{
				"col-1": {
					Name: "col-1",
					Marks: []*Mark{
						{ID: "abc12345", Name: "first", URL: "https://example.com/first"},
					},
				},
			},
		},
	}

	got := bs.FindMarkByID("abc12345")
	if got == nil {
		t.Fatal("FindMarkByID expected match")
	}
	if got.Name != "first" {
		t.Errorf("FindMarkByID Name = %q, want %q", got.Name, "first")
	}
	if got.Shelf == nil || got.Shelf.Name != "shelf-a" {
		t.Error("FindMarkByID did not set Shelf back-pointer")
	}
	if got.Collection == nil || got.Collection.Name != "col-1" {
		t.Error("FindMarkByID did not set Collection back-pointer")
	}

	if bs.FindMarkByID("nope") != nil {
		t.Error("FindMarkByID unexpected match")
	}
}

func TestMarkUpdateMark(t *testing.T) {
	m := &Mark{
		ID:   GenerateID("https://example.com/old"),
		Name: "Old",
		URL:  "https://example.com/old",
		Tags: []string{"a"},
	}

	if err := m.UpdateMark("New", "", []string{"b", "c"}); err != nil {
		t.Fatalf("UpdateMark error: %v", err)
	}
	if m.Name != "New" {
		t.Errorf("Name = %q, want %q", m.Name, "New")
	}
	if !slices.Equal(m.Tags, []string{"b", "c"}) {
		t.Errorf("Tags = %v, want %v", m.Tags, []string{"b", "c"})
	}
	if m.URL != "https://example.com/old" {
		t.Errorf("URL = %q, want unchanged", m.URL)
	}

	if err := m.UpdateMark("", "not-a-url", nil); err == nil {
		t.Error("UpdateMark with invalid URL expected error")
	}

	if err := m.UpdateMark("", "https://example.com/new", nil); err != nil {
		t.Fatalf("UpdateMark URL change error: %v", err)
	}
	if m.URL != "https://example.com/new" {
		t.Errorf("URL = %q, want %q", m.URL, "https://example.com/new")
	}
	if m.ID != GenerateID("https://example.com/new") {
		t.Errorf("ID = %q, want regenerated ID", m.ID)
	}
}

func TestBookShelvesFindMarkByURL(t *testing.T) {
	bs := BookShelves{
		{
			Name: "shelf-a",
			Collections: map[string]*Collection{
				"col-1": {
					Name: "col-1",
					Marks: []*Mark{
						{ID: "abc12345", Name: "first", URL: "https://example.com/first"},
						{ID: "deadbeef", Name: "trashed", URL: "https://example.com/trashed", DeletedAt: "2026-08-01T00:00:00Z"},
					},
				},
			},
		},
	}
	bs.LoadParents()

	got := bs.FindMarkByURL("https://example.com/first")
	if got == nil {
		t.Fatal("FindMarkByURL expected match")
	}
	if got.Name != "first" {
		t.Errorf("Name = %q, want %q", got.Name, "first")
	}
	if got.Shelf == nil || got.Shelf.Name != "shelf-a" {
		t.Error("FindMarkByURL did not set Shelf back-pointer")
	}

	if bs.FindMarkByURL("https://example.com/trashed") != nil {
		t.Error("FindMarkByURL should skip deleted marks")
	}
	if bs.FindMarkByURL("https://example.com/missing") != nil {
		t.Error("FindMarkByURL unexpected match")
	}
}

func TestMarkTouch(t *testing.T) {
	shelf := &Shelf{Name: "shelf-a", SchemaVersion: IntPtr(2), UpdatedAt: "old"}
	collection := &Collection{Name: "col-1", Shelf: shelf, UpdatedAt: "old"}
	mark := &Mark{Name: "mark", Shelf: shelf, Collection: collection}

	mark.Touch()

	if mark.UpdatedAt == "" || mark.UpdatedAt == "old" {
		t.Error("Touch did not update mark UpdatedAt")
	}
	if collection.UpdatedAt == "old" {
		t.Error("Touch did not update collection UpdatedAt")
	}
	if shelf.UpdatedAt == "old" {
		t.Error("Touch did not update shelf UpdatedAt")
	}
}

func TestMarkRecordAdd(t *testing.T) {
	shelf := &Shelf{Name: "shelf-a", SchemaVersion: IntPtr(2)}
	collection := &Collection{Name: "col-1", Shelf: shelf}
	mark := &Mark{Name: "mark", Shelf: shelf, Collection: collection}

	mark.RecordAdd()

	if mark.CreatedAt == "" {
		t.Error("RecordAdd did not set CreatedAt")
	}
	if mark.UpdatedAt != mark.CreatedAt {
		t.Error("RecordAdd did not set UpdatedAt equal to CreatedAt")
	}
	if shelf.UpdatedAt == "" {
		t.Error("RecordAdd did not cascade to shelf")
	}
}

func TestMarkRecordDelete(t *testing.T) {
	shelf := &Shelf{Name: "shelf-a", SchemaVersion: IntPtr(2)}
	collection := &Collection{Name: "col-1", Shelf: shelf, Marks: make([]*Mark, 0)}
	mark := &Mark{Name: "mark", Shelf: shelf, Collection: collection}
	collection.AddMark(mark)

	mark.RecordDelete()

	if !mark.IsDeleted() {
		t.Error("RecordDelete did not soft-delete mark")
	}
	if mark.UpdatedAt == "" {
		t.Error("RecordDelete did not update mark UpdatedAt")
	}
	if shelf.UpdatedAt == "" {
		t.Error("RecordDelete did not cascade to shelf")
	}
}

func TestUpdateMarkClearsTags(t *testing.T) {
	m := &Mark{
		ID:   GenerateID("https://example.com"),
		Name: "Old",
		URL:  "https://example.com",
		Tags: []string{"a", "b"},
	}

	if err := m.UpdateMark("", "", []string{}); err != nil {
		t.Fatalf("UpdateMark error: %v", err)
	}
	if len(m.Tags) != 0 {
		t.Errorf("Tags = %v, want empty", m.Tags)
	}

	if err := m.UpdateMark("", "", nil); err != nil {
		t.Fatalf("UpdateMark nil error: %v", err)
	}
	if len(m.Tags) != 0 {
		t.Errorf("Tags = %v, want still empty after nil", m.Tags)
	}
}

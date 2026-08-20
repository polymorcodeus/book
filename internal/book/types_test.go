package book

import (
	"slices"
	"testing"
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

func TestStructIsEmpty(t *testing.T) {
	tests := []struct {
		name string
		ptr  *Mark
		want bool
	}{
		{
			name: "nil pointer",
			ptr:  nil,
			want: true,
		},
		{
			name: "zero struct",
			ptr:  &Mark{},
			want: true,
		},
		{
			name: "non-zero struct",
			ptr:  &Mark{Name: "example"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StructIsEmpty(tt.ptr); got != tt.want {
				t.Errorf("StructIsEmpty() = %v, want %v", got, tt.want)
			}
		})
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
		name    string
		id      string
		wantErr bool
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bs.VerifyUniqueURL(tt.id)
			if tt.wantErr && err == nil {
				t.Errorf("VerifyUniqueURL(%q) expected error, got nil", tt.id)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("VerifyUniqueURL(%q) unexpected error: %v", tt.id, err)
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
	markC := &Mark{Name: "c"}

	tests := []struct {
		name       string
		start      []*Mark
		remove     *Mark
		wantNames  []string
		wantLength int
	}{
		{
			name:       "removes by pointer identity",
			start:      []*Mark{markA, markB, markC},
			remove:     markB,
			wantNames:  []string{"a", "c"},
			wantLength: 2,
		},
		{
			name:       "removing absent mark is no-op",
			start:      []*Mark{markA, markC},
			remove:     markB,
			wantNames:  []string{"a", "c"},
			wantLength: 2,
		},
		{
			name:       "removes only exact pointer match",
			start:      []*Mark{markA, {Name: "a"}},
			remove:     markA,
			wantNames:  []string{"a"},
			wantLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := &Collection{Marks: tt.start}
			col.DeleteMark(tt.remove)

			if len(col.Marks) != tt.wantLength {
				t.Errorf("len(Marks) = %d, want %d", len(col.Marks), tt.wantLength)
			}

			gotNames := make([]string, len(col.Marks))
			for i, m := range col.Marks {
				gotNames[i] = m.Name
			}
			if !slices.Equal(gotNames, tt.wantNames) {
				t.Errorf("remaining marks = %v, want %v", gotNames, tt.wantNames)
			}
		})
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

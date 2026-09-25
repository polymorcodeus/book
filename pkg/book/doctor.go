package book

import "slices"

// MarkConflict groups marks that share a catalog_id. When catalog_id is empty
// (an unmigrated v1 mark) the ID is derived from the URL, matching the identity
// `book migrate` and the index assign.
type MarkConflict struct {
	ID    string  // the shared catalog_id (or derived ID)
	URL   string  // the mark URL (identical across the group unless IDs collide)
	Marks []*Mark // every mark sharing the ID, in deterministic catalog order

	// TrueConflict reports whether the group holds the same URL with differing
	// content (title, tags, or deleted state). A true conflict cannot be
	// auto-merged; a false one is a harmless duplicate from a git merge.
	TrueConflict bool
}

// effectiveID returns the mark's catalog_id, deriving it from the URL when the
// field is empty. This keeps v1 (unmigrated) marks comparable to v2 marks.
func effectiveID(m *Mark) string {
	if m.ID != "" {
		return m.ID
	}
	return GenerateID(m.URL)
}

// DetectDuplicates scans every shelf and collection for marks that share a
// catalog_id. Results are returned in stable order (by ID). Collection order is
// resolved by name so the outcome is deterministic across runs despite the
// map-keyed collections.
func (bs *BookShelves) DetectDuplicates() []MarkConflict {
	groups := make(map[string]*MarkConflict)
	var order []string

	for i := range *bs {
		shelf := &(*bs)[i]
		for _, name := range shelf.CollectionsNames() {
			c := shelf.Collections[name]
			for _, m := range c.Marks {
				id := effectiveID(m)
				g, ok := groups[id]
				if !ok {
					g = &MarkConflict{ID: id, URL: m.URL}
					groups[id] = g
					order = append(order, id)
				}
				g.Marks = append(g.Marks, m)
			}
		}
	}

	slices.Sort(order)
	result := make([]MarkConflict, 0, len(order))
	for _, id := range order {
		g := groups[id]
		if len(g.Marks) < 2 {
			continue
		}
		g.TrueConflict = !marksIdentical(g.Marks)
		result = append(result, *g)
	}
	return result
}

// ResolveDuplicates removes marks that are exact duplicates (same catalog_id
// and identical content) of an earlier mark, keeping the first occurrence. True
// conflicts are left untouched for manual resolution. It returns the number of
// marks removed and the shelves whose collections were modified.
func (bs *BookShelves) ResolveDuplicates() (removed int, changed []*Shelf) {
	changedSet := make(map[*Shelf]struct{})
	for _, conflict := range bs.DetectDuplicates() {
		if conflict.TrueConflict {
			continue
		}
		for _, dup := range conflict.Marks[1:] {
			if dup.Collection == nil {
				continue
			}
			dup.Collection.RemoveMark(dup)
			removed++
			if dup.Shelf != nil {
				changedSet[dup.Shelf] = struct{}{}
			}
		}
	}

	changed = make([]*Shelf, 0, len(changedSet))
	for shelf := range changedSet {
		changed = append(changed, shelf)
	}
	// Deterministic ordering for the caller's report.
	slices.SortFunc(changed, func(a, b *Shelf) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return removed, changed
}

// marksIdentical reports whether every mark in a non-empty group has the same
// title, URL, tags, and deleted state.
func marksIdentical(marks []*Mark) bool {
	first := marks[0]
	for _, m := range marks[1:] {
		if m.Name != first.Name || m.URL != first.URL || m.DeletedAt != first.DeletedAt {
			return false
		}
		if !equalTags(m.Tags, first.Tags) {
			return false
		}
	}
	return true
}

// equalTags compares two tag slices as sets, ignoring order.
func equalTags(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := slices.Clone(a)
	sb := slices.Clone(b)
	slices.Sort(sa)
	slices.Sort(sb)
	return slices.Equal(sa, sb)
}

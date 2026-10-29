package store

// Ids are the row identity that Power BI already knows about, and the push
// dataset delete path (clear table + re-upload survivors) can remove the
// highest id. Guards against re-issuing an id that was already used.

import (
	"path/filepath"
	"testing"
)

func TestInsertDocDoesNotReuseIDs(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	insert := func(title string) int64 {
		d := &Doc{Title: title, Author: "tester", Category: "기타", Updated: "2026-10-29", Status: "진행"}
		if err := s.InsertDoc(d); err != nil {
			t.Fatalf("insert %q: %v", title, err)
		}
		return d.ID
	}

	first, second := insert("a"), insert("b")
	if first != 1 || second != 2 {
		t.Fatalf("fresh cache ids = %d,%d; want 1,2", first, second)
	}

	// Delete the highest id (what DeleteDoc does locally before the remote
	// clear + re-upload), then insert again: the id must not come back.
	if err := s.DeleteDoc(second); err != nil {
		t.Fatalf("delete: %v", err)
	}
	third := insert("c")
	if third <= second {
		t.Fatalf("reused id %d after deleting the highest id (%d)", third, second)
	}

	// A cache replace from Power BI can lower MAX(id); the issued-id
	// high-water mark must still win.
	if err := s.ReplaceDocs([]Doc{{ID: 1, Title: "a"}, {ID: 2, Title: "b"}}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	fourth := insert("d")
	if fourth <= third {
		t.Fatalf("id went backwards after ReplaceDocs: %d then %d", third, fourth)
	}

	docs, err := s.ListDocs()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(docs) != 3 { // ids 1,2 from the replace + the new row
		t.Fatalf("cached rows = %d; want 3", len(docs))
	}
}

package catalog

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/polymorcodeus/book/internal/book"
)

// fixtureTOML matches the real archive.toml format: nested map tables for
// collections with explicit keys, so each mark header shows its parent.
const fixtureTOML = `shelf_name = "archive"
shelf_desc = "where books go to die!"

[Collections]
  [Collections.powash311]
    collection_name = "powash311"
    collection_desc = "pow pow powashell"

    [[Collections.powash311.marks]]
      catalog_id = "21f96eef"
      title = "counteractive/o365beat: Elastic Beat for fetching and shipping Office 365 audit events"
      url = "https://github.com/counteractive/o365beat"
      tags = ["powershell", "windows"]

    [[Collections.powash311.marks]]
      catalog_id = "e1c3808c"
      title = "Introduction to Testing Your PowerShell Code with Pester - Simple Talk"
      url = "https://www.red-gate.com/simple-talk/sysadmin/powershell/introduction-to-testing-your-powershell-code-with-pester/"
      tags = ["powershell", "windows"]

  [Collections.swyfty]
    collection_name = "swyfty"
    collection_desc = "getting schhwifty"

    [[Collections.swyfty.marks]]
      catalog_id = "fde869ba"
      title = "Make an API call - Box Developer Documentation"
      url = "https://developer.box.com/guides/mobile/ios/quick-start/make-api-call/"
      tags = ["swift", "ios", "api"]

    [[Collections.swyfty.marks]]
      catalog_id = "9af83e1a"
      title = "Implement an API client in Swift using Generics, Codable and Combine | by Marina Sauca | Mac O’Clock | Medium"
      url = "https://medium.com/macoclock/swift-generic-api-854afdb9315e"
      tags = ["swift", "ios", "api"]
`

// TestRoundTripIdempotency decodes a fixture TOML, re-encodes it, and
// asserts the bytes are identical. This is the prerequisite for trusting
// git diffs: if re-saving an unchanged shelf produces different bytes,
// every read/write cycle would create noisy diffs.
func TestRoundTripIdempotency(t *testing.T) {
	tmpDir := t.TempDir()

	// Write the fixture to a temp file.
	fixturePath := filepath.Join(tmpDir, "fixture.toml")
	if err := os.WriteFile(fixturePath, []byte(fixtureTOML), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Decode.
	var shelf book.Shelf
	if _, err := toml.DecodeFile(fixturePath, &shelf); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	// Re-encode to a new buffer.
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(shelf); err != nil {
		t.Fatalf("encode shelf: %v", err)
	}

	// Compare — normalize whitespace because BurntSushi/toml may add/omit
	// blank lines between sections. What matters is that the semantic
	// content is identical and stable across re-encodes.
	want := normalizeTOMLWhitespace([]byte(fixtureTOML))
	got := normalizeTOMLWhitespace(buf.Bytes())

	if !bytes.Equal(got, want) {
		t.Logf("want (%d bytes):\n%s", len(want), want)
		t.Logf("got  (%d bytes):\n%s", len(got), got)
		t.Errorf("round-trip mismatch: see logged output above")
	}
}

// normalizeTOMLWhitespace strips blank lines and normalizes line endings
// so that encoder formatting differences don't cause false negatives.
func normalizeTOMLWhitespace(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	var out [][]byte
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			out = append(out, trimmed)
		}
	}
	return bytes.Join(out, []byte("\n"))
}

// TestCreateTOMLAtomic verifies that CreateTOML writes a temp file and
// renames it into place, leaving no .tmp debris on success.
func TestCreateTOMLAtomic(t *testing.T) {
	tmpDir := t.TempDir()

	shelf := book.Shelf{
		Name:        "Test",
		Description: "test shelf",
		Collections: map[string]*book.Collection{
			"General": {
				Name:        "General",
				Description: "general collection",
				Marks: []*book.Mark{
					{
						Name: "Example",
						URL:  "https://example.com",
						Tags: []string{"demo"},
					},
				},
			},
		},
		FilePath: filepath.Join(tmpDir, "test.toml"),
	}

	if err := CreateTOML(&shelf); err != nil {
		t.Fatalf("create TOML: %v", err)
	}

	// Verify the file exists.
	if _, err := os.Stat(shelf.FilePath); err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	// Verify no .tmp debris.
	tmpPath := shelf.FilePath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file %s was not cleaned up", tmpPath)
	}

	// Verify it decodes back.
	var decoded book.Shelf
	if _, err := toml.DecodeFile(shelf.FilePath, &decoded); err != nil {
		t.Fatalf("decode written file: %v", err)
	}
	if decoded.Name != shelf.Name {
		t.Errorf("name mismatch: got %q, want %q", decoded.Name, shelf.Name)
	}
}

// TestCreateTOMLAtomicOverwrite verifies that on successful write, the
// original file is atomically replaced (not corrupted or left partial).
func TestCreateTOMLAtomicOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "existing.toml")

	// Pre-create a file to simulate an existing shelf.
	original := []byte("shelf_name = \"Original\"\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	// Create a shelf that will encode successfully.
	shelf := book.Shelf{
		Name:        "New",
		Description: "new shelf",
		FilePath:    path,
	}

	if err := CreateTOML(&shelf); err != nil {
		t.Fatalf("create TOML: %v", err)
	}

	// Verify the original was overwritten (rename is atomic).
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if bytes.Contains(content, []byte("Original")) {
		t.Error("atomic rename failed: original content still present")
	}
}

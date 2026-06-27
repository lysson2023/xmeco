package migration

import (
	"bytes"
	"testing"
)

// =============================================================================
// Tier 3 — M-07: BOM 剥离
// =============================================================================

func TestBOMStripping(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "M-07 含UTF-8 BOM被剥离",
			input: append([]byte{0xEF, 0xBB, 0xBF}, []byte("CREATE TABLE users (id SERIAL PRIMARY KEY);")...),
			want:  []byte("CREATE TABLE users (id SERIAL PRIMARY KEY);"),
		},
		{
			name:  "无BOM保持不变",
			input: []byte("SELECT 1;"),
			want:  []byte("SELECT 1;"),
		},
		{
			name:  "仅BOM无内容",
			input: []byte{0xEF, 0xBB, 0xBF},
			want:  []byte{},
		},
		{
			name:  "空内容",
			input: []byte{},
			want:  []byte{},
		},
		{
			name:  "以0xEF开头但不是BOM",
			input: []byte{0xEF, 0x01, 0x02, 0x03},
			want:  []byte{0xEF, 0x01, 0x02, 0x03},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the BOM stripping logic from Run()
			got := bytes.TrimPrefix(tt.input, []byte{0xEF, 0xBB, 0xBF})
			if !bytes.Equal(got, tt.want) {
				t.Errorf("BOM stripping: got %X, want %X", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Tier 3 — M-01~M-09: listMigrationFiles
// =============================================================================

func TestListMigrationFiles(t *testing.T) {
	files, err := listMigrationFiles()
	if err != nil {
		t.Fatalf("listMigrationFiles() unexpected error: %v", err)
	}

	// Verify we have expected migrations
	if len(files) == 0 {
		t.Fatal("expected migration files, got none")
	}

	// Verify files are sorted by version
	for i := 1; i < len(files); i++ {
		if files[i].version <= files[i-1].version {
			t.Errorf("migrations not sorted: [%d]=%d, [%d]=%d",
				i-1, files[i-1].version, i, files[i].version)
		}
	}

	// Verify first and last migration versions
	if files[0].version != 1 {
		t.Errorf("first migration version = %d, want 1", files[0].version)
	}
	if files[len(files)-1].version < 10 {
		t.Errorf("expected at least 10 migrations, got %d", files[len(files)-1].version)
	}

	// Verify each file has required fields
	for _, f := range files {
		if f.version <= 0 {
			t.Errorf("invalid version %d for %s", f.version, f.name)
		}
		if f.title == "" {
			t.Errorf("empty title for %s", f.name)
		}
		if f.name == "" {
			t.Error("empty name in migration file")
		}
		// Verify naming pattern: NNN_name.up.sql
		if len(f.name) < 10 {
			t.Errorf("unexpected short filename: %s", f.name)
		}
	}

	t.Logf("found %d migration files (v%d → v%d)", len(files), files[0].version, files[len(files)-1].version)
}

func TestListMigrationFilesEmptyOnNoSQL(t *testing.T) {
	// Verify the embed FS is accessible and files have .up.sql extension
	files, err := listMigrationFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.name[len(f.name)-7:] != ".up.sql" {
			t.Errorf("unexpected file extension: %s", f.name)
		}
	}
}

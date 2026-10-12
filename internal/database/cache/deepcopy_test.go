package cache

import (
	"testing"
	"time"

	caches "github.com/mgdigital/gorm-cache/v2"

	"github.com/ghobs91/lodestone/internal/database/fts"
	"github.com/ghobs91/lodestone/internal/model"
	"github.com/ghobs91/lodestone/internal/protocol"
)

// TestDeepCopyRealModel exercises the reflection deep copy against a type
// similar to what the search cache stores, containing nested structs, slices,
// maps and null wrappers. It must not panic and must fully detach the copy.
func TestDeepCopyRealModel(t *testing.T) {
	items := []model.TorrentContent{
		{
			ID:          "1",
			InfoHash:    protocol.ID{1, 2, 3},
			Languages:   model.Languages{model.Language("en"): struct{}{}},
			Episodes:    model.Episodes{1: {1: struct{}{}}},
			PublishedAt: time.Now(),
			Tsv:         fts.Tsvector{"a": {1: fts.TsvectorWeight('A')}},
			Torrent: model.Torrent{
				InfoHash: protocol.ID{1, 2, 3},
				Name:     "Example",
				Files: []model.TorrentFile{
					{InfoHash: protocol.ID{1, 2, 3}, Index: 0, Path: "a.mkv"},
				},
			},
		},
	}

	original := caches.Query{Dest: &items, RowsAffected: 1}
	copied := deepCopyQuery(&original)

	if copied == nil {
		t.Fatal("deepCopyQuery returned nil")
	}

	copiedItems := copied.Dest.(*[]model.TorrentContent)
	if len(*copiedItems) != 1 {
		t.Fatalf("expected 1 item, got %d", len(*copiedItems))
	}

	// Mutating the copy must not affect the original.
	(*copiedItems)[0].Torrent.Name = "mutated"
	(*copiedItems)[0].Languages[model.Language("fr")] = struct{}{}
	(*copiedItems)[0].Torrent.Files[0].Path = "mutated.mkv"
	(*copiedItems)[0].Tsv["b"] = map[int]fts.TsvectorWeight{1: fts.TsvectorWeight('B')}

	if items[0].Torrent.Name == "mutated" {
		t.Error("copy shares Torrent.Name with original")
	}
	if _, ok := items[0].Languages[model.Language("fr")]; ok {
		t.Error("copy shares Languages map with original")
	}
	if items[0].Torrent.Files[0].Path == "mutated.mkv" {
		t.Error("copy shares Files slice with original")
	}
	if _, ok := items[0].Tsv["b"]; ok {
		t.Error("copy shares Tsv map with original")
	}
}

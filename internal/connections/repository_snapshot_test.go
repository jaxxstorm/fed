package connections

import (
	"encoding/json"
	"errors"
	"github.com/jaxxstorm/fed/internal/storage"
	"testing"
	"time"
)

func TestRepositorySnapshotRetainsFilesAndFailsClosed(t *testing.T) {
	now := time.Now().UTC()
	contents := map[string][]byte{".github/CODEOWNERS": []byte("* @owners\n"), "binary": {0xff, 0, 1}}
	read := func(path string) (Snapshot, error) {
		return Snapshot{Content: contents[path], MediaType: "text/plain", ETag: path, ObservedAt: now}, nil
	}
	snapshot, err := repositorySnapshot("commit", []string{".github/CODEOWNERS", "binary"}, read)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Commit string
		Files  []struct {
			Path    string
			Content []byte `json:"content_base64"`
			Hash    string `json:"content_hash"`
		}
	}
	if err = json.Unmarshal(snapshot.Content, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Commit != "commit" || len(decoded.Files) != 2 {
		t.Fatal("missing provenance or files")
	}
	for _, f := range decoded.Files {
		if string(f.Content) != string(contents[f.Path]) || f.Hash != storage.Hash(f.Content) {
			t.Fatal("content or hash changed")
		}
	}
	failure := errors.New("denied")
	partial, err := repositorySnapshot("commit", []string{".github/CODEOWNERS", "binary"}, func(path string) (Snapshot, error) {
		if path == "binary" {
			return Snapshot{}, failure
		}
		return read(path)
	})
	if !errors.Is(err, failure) || len(partial.Content) != 0 {
		t.Fatal("partial snapshot accepted")
	}
	single, err := repositorySnapshot("commit", []string{".github/CODEOWNERS"}, read)
	if err != nil || string(single.Content) != string(contents[".github/CODEOWNERS"]) {
		t.Fatal("single-file compatibility changed")
	}
}

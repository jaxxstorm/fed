package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactIntegrity(t *testing.T) {
	f := Files{Directory: t.TempDir()}
	ctx := context.Background()
	key, err := f.Put(ctx, []byte("evidence"))
	if err != nil {
		t.Fatal(err)
	}
	if b, err := f.Get(ctx, key); err != nil || string(b) != "evidence" {
		t.Fatalf("round trip: %s %v", b, err)
	}
	if _, err = f.Get(ctx, "../../outside"); err == nil {
		t.Fatal("path accepted")
	}
	if err = os.WriteFile(filepath.Join(f.Directory, key), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Get(ctx, key); err == nil {
		t.Fatal("corruption accepted")
	}
}

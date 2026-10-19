package guidancecontent

import (
	"context"
	"fmt"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/storage"
)

func draftInventory(snapshot catalog.Snapshot) (catalog.GuidanceInventory, error) {
	if snapshot.ID != snapshot.Commit+":"+storage.Hash(snapshot.Payload) {
		return catalog.GuidanceInventory{}, fmt.Errorf("authored catalog identity mismatch")
	}
	return catalog.CachedInventory(context.Background(), snapshot.Payload, snapshot.Schema, snapshot.Commit)
}

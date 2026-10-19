package guidancecontent

import (
	"fmt"
	"sync"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/storage"
)

// Domain builders share only immutable catalog inventory, never service state,
// source-file verification or review authority. Bound retention to two payload/
// schema identities so source-mutation tests and new catalogs cannot reuse stale
// applicability results or grow an unbounded process cache.
var authoredInventoryCache = struct {
	sync.Mutex
	items map[string]catalog.GuidanceInventory
	order []string
}{items: map[string]catalog.GuidanceInventory{}}

func draftInventory(snapshot catalog.Snapshot) (catalog.GuidanceInventory, error) {
	if snapshot.ID != snapshot.Commit+":"+storage.Hash(snapshot.Payload) {
		return catalog.GuidanceInventory{}, fmt.Errorf("authored catalog identity mismatch")
	}
	key := snapshot.ID + ":" + storage.Hash(snapshot.Schema)
	authoredInventoryCache.Lock()
	defer authoredInventoryCache.Unlock()
	if inventory, ok := authoredInventoryCache.items[key]; ok {
		return inventory, nil
	}
	inventory, err := catalog.Inventory(snapshot)
	if err != nil {
		return inventory, err
	}
	if len(authoredInventoryCache.order) == 2 {
		delete(authoredInventoryCache.items, authoredInventoryCache.order[0])
		authoredInventoryCache.order = authoredInventoryCache.order[1:]
	}
	authoredInventoryCache.items[key] = inventory
	authoredInventoryCache.order = append(authoredInventoryCache.order, key)
	return inventory, nil
}

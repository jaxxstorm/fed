package catalog

import (
	"context"
	"sync"

	"github.com/jaxxstorm/fed/internal/storage"
)

// Share only immutable catalog inventory across authoring and publication.
// Tenant data, authenticated reviews, source freshness and coverage decisions
// never enter this cache. Retain at most two payload/schema identities.
var inventoryCache = struct {
	sync.Mutex
	items map[string]GuidanceInventory
	order []string
}{items: map[string]GuidanceInventory{}}

func CachedInventory(ctx context.Context, payload, schema []byte, commit string) (GuidanceInventory, error) {
	key := commit + ":" + storage.Hash(payload) + ":" + storage.Hash(schema)
	inventoryCache.Lock()
	defer inventoryCache.Unlock()
	if err := ctx.Err(); err != nil {
		return GuidanceInventory{}, err
	}
	if cached, ok := inventoryCache.items[key]; ok {
		return cached, nil
	}
	snapshot, err := Parse(payload, schema, commit)
	if err != nil {
		return GuidanceInventory{}, err
	}
	inventory, err := Inventory(snapshot)
	if err != nil {
		return inventory, err
	}
	if err := ctx.Err(); err != nil {
		return GuidanceInventory{}, err
	}
	if len(inventoryCache.order) == 2 {
		delete(inventoryCache.items, inventoryCache.order[0])
		inventoryCache.order = inventoryCache.order[1:]
	}
	inventoryCache.items[key] = inventory
	inventoryCache.order = append(inventoryCache.order, key)
	return inventory, nil
}

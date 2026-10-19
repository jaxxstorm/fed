package catalog

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestCachedInventoryIdentityAndBound(t *testing.T) {
	ctx := context.Background()
	commit := strings.Repeat("a", 40)
	schema := []byte(`{"type":"object"}`)
	var firstKey string
	for i := range 3 {
		payload := []byte(fmt.Sprintf(`{"KSI":{"TEST":{"status":"stable","indicators":{"RULE":{"name":"Cache fixture %d","statement":"Retain records."}}}}}`, i))
		snapshot, err := Parse(payload, schema, commit)
		if err != nil {
			t.Fatal(err)
		}
		want, err := Inventory(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			got, err := CachedInventory(ctx, payload, schema, commit)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatal("cached inventory differs from fresh resolution", err)
			}
		}
		inventoryCache.Lock()
		if i == 0 {
			firstKey = inventoryCache.order[len(inventoryCache.order)-1]
		}
		count, order := len(inventoryCache.items), len(inventoryCache.order)
		_, firstRetained := inventoryCache.items[firstKey]
		inventoryCache.Unlock()
		if count > 2 || order > 2 || (i == 2 && firstRetained) {
			t.Fatal("immutable inventory cache exceeded retention bound", count, order, firstRetained)
		}
		if _, err := CachedInventory(ctx, payload, []byte(`{"type":"array"}`), commit); err == nil {
			t.Fatal("changed schema reused prior validation")
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := CachedInventory(cancelled, payload, schema, commit); !errors.Is(err, context.Canceled) {
			t.Fatal("cache hit ignored cancellation", err)
		}
	}
}

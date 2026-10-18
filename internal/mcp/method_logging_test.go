package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/jaxxstorm/fed/internal/logging"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConcurrentInProcessToolCorrelation(t *testing.T) {
	var logs bytes.Buffer
	l, _ := logging.New(&logs, nil, "debug", "json", "server")
	ctx := logging.WithLogger(context.Background(), l)
	server := sdk.NewServer(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	server.AddReceivingMiddleware(methodLogging)
	add(server, "test_tool", "test", func(context.Context, struct{}) (any, error) { return "secret-result", nil })
	st, ct := sdk.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: "test_tool", Arguments: map[string]any{}}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	cs.Close()
	ss.Close()
	ids := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var e map[string]any
		_ = json.Unmarshal([]byte(line), &e)
		if e["event"] == "mcp.tool.completed" {
			id, ok := e["operation_id"].(string)
			if !ok || ids[id] {
				t.Fatal("missing/duplicate operation id", e)
			}
			ids[id] = true
			if e["request_id"] != nil {
				t.Fatal("fabricated HTTP correlation")
			}
		}
	}
	if len(ids) != 8 || strings.Contains(logs.String(), "secret-result") {
		t.Fatal("missing tool logs or leaked result", logs.String())
	}
}

func BenchmarkMCPMethodLogging(b *testing.B) {
	l, _ := logging.New(nil, nil, "info", "json", "server")
	ctx := logging.WithLogger(context.Background(), l)
	handler := methodLogging(func(context.Context, string, sdk.Request) (sdk.Result, error) { return &sdk.ListToolsResult{}, nil })
	b.ReportAllocs()
	for b.Loop() {
		_, _ = handler(ctx, "tools/list", nil)
	}
}

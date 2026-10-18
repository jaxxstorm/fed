package mcp_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/checks"
	"github.com/jaxxstorm/fed/internal/logging"
	adapter "github.com/jaxxstorm/fed/internal/mcp"
	"github.com/jaxxstorm/fed/internal/testutil"
	"github.com/jaxxstorm/fed/internal/web"
	"github.com/labstack/echo/v4"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type logCapture struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (c *logCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.b.Write(p)
}
func (c *logCapture) String() string { c.mu.Lock(); defer c.mu.Unlock(); return c.b.String() }

func TestMCPLogOutcomesAndSecretExclusion(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "logging")
	if err != nil {
		t.Fatal(err)
	}
	store := &assessment.Store{DB: db}
	service, err := store.CreateService(ctx, a, "Logging")
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.IssueToken(ctx, db, a, service.ID, []string{"read"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	var capture logCapture
	l, _ := logging.New(&capture, nil, "debug", "json", "server")
	e := web.Bootstrap(db.Ping)
	e.Any("/mcp", echo.WrapHandler(adapter.Handler(store, &checks.Runner{DB: db})))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.ServeHTTP(w, r.WithContext(logging.WithLogger(r.Context(), l)))
	}))
	defer server.Close()
	client := sdk.NewClient(&sdk.Implementation{Name: "logging-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{token}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if _, err = session.CallTool(ctx, &sdk.CallToolParams{Name: "list_services", Arguments: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: "get_assessment_scope", Arguments: map[string]any{"service_id": "secret-unauthorized-service"}})
	if err != nil || !result.IsError {
		t.Fatal("expected forbidden tool", err)
	}
	_, _ = session.CallTool(ctx, &sdk.CallToolParams{Name: "get_assessment_scope", Arguments: map[string]any{"service_id": 123, "secret": "secret-argument"}})
	request, err := http.NewRequest("POST", server.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":"secret-rpc-id","method":"tools/call","params":{"name":123,"arguments":{"secret":"secret-protocol-payload"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", "2025-03-26")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	logs := capture.String()
	for _, s := range []string{token, "secret-unauthorized-service", "secret-argument", "secret-rpc-id", "secret-protocol-payload"} {
		if strings.Contains(logs, s) {
			t.Fatal("sensitive input logged")
		}
	}
	for _, s := range []string{`"event":"mcp.tool.started"`, `"tool":"list_services"`, `"error_code":"forbidden"`, `"request_id":`, `"operation_id":`, `"actor_id":`, `"event":"mcp.method.completed"`, `"event":"mcp.protocol.failed"`} {
		if !strings.Contains(logs, s) {
			t.Fatal("missing MCP field", s, logs)
		}
	}
}

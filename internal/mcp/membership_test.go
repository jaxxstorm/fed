package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/checks"
	adapter "github.com/jaxxstorm/fed/internal/mcp"
	"github.com/jaxxstorm/fed/internal/testutil"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPendingIdentityCannotUseProductTools(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	if _, err := auth.BootstrapAdmin(ctx, db, "test", "admin"); err != nil {
		t.Fatal(err)
	}
	m, err := auth.Admit(ctx, db, auth.AdmissionConfig{}, auth.VerifiedIdentity{Issuer: "test", Subject: "pending", Email: "pending@example.com", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{ID: m.IdentityID, OrganizationID: m.OrganizationID, Kind: "human"}
	server := adapter.NewServer(&assessment.Store{DB: db}, &checks.Runner{DB: db}, actor)
	st, ct := sdk.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := sdk.NewClient(&sdk.Implementation{Name: "membership-test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	result, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: "list_services", Arguments: map[string]any{}})
	if err != nil || !result.IsError {
		t.Fatal("pending tool succeeded", result, err)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	if !strings.Contains(string(raw), "membership_pending") {
		t.Fatal(string(raw))
	}
}

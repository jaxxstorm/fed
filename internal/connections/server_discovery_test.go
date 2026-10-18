package connections

import (
	"bytes"
	"context"
	"github.com/jaxxstorm/fed/internal/logging"
	"strings"
	"testing"
	"time"
)

func TestServerDiscoveryReceiptAndProvenance(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: AWS, Name: "Account", Mode: ServerEnvironment, Configuration: Configuration{AccountID: "123456789012", Region: "us-east-1", Profile: "prod"}})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.Registry, _ = NewRegistry(map[Provider]Adapter{AWS: testAdapter{discover: func(context.Context, Connection, Credentials, string, int) (DiscoveryPage, error) {
		calls++
		return DiscoveryPage{Resources: []DiscoveredResource{{Resource: ResourceKey{Provider: AWS, Tenant: "123456789012", Kind: "account_region", ID: "us-east-1"}, Name: "Account", Locator: "123456789012/us-east-1"}}, Coverage: []KindCoverage{{Kind: "account_region", Complete: true, Authoritative: true}, {Kind: "eks_cluster", Complete: true, Authoritative: true}}}, nil
	}}})
	session, err := s.StartServerDiscovery(ctx, a, OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "start"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SubmitAgentDiscovery(ctx, a, DiscoverySubmission{ServiceID: "svc", SessionID: session.ID, IdempotencyKey: "fake", ObservedAt: time.Now().UTC(), Method: "agent"}); err == nil {
		t.Fatal("agent forged server discovery")
	}
	in := ServerDiscoveryInput{ServiceID: "svc", SessionID: session.ID, IdempotencyKey: "page"}
	receipt, err := s.DiscoverServerPage(ctx, a, in)
	if err != nil || receipt.State != Complete || receipt.Mode != ServerEnvironment {
		t.Fatalf("server discovery: %+v %v", receipt, err)
	}
	if _, err = s.DiscoverServerPage(ctx, a, in); err != nil || calls != 1 {
		t.Fatal("replay repeated provider read")
	}
	page, err := s.ListInventory(ctx, a, InventoryQuery{ServiceID: "svc", ConnectionID: d.Connection.ID})
	if err != nil || len(page.Resources) != 1 || page.Resources[0].Mode != ServerEnvironment {
		t.Fatal("inventory lost server attribution")
	}
}

func TestPartialDiscoveryOperationalLogs(t *testing.T) {
	var logs bytes.Buffer
	l, _ := logging.New(&logs, nil, "debug", "json", "server")
	ctx := logging.WithLogger(context.Background(), l)
	s, a := setupService(t)
	d, err := s.Configure(ctx, a, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: AWS, Name: "secret-name", Mode: ServerEnvironment, Configuration: Configuration{AccountID: "123456789012", Region: "us-east-1", Profile: "secret-profile"}})
	if err != nil {
		t.Fatal(err)
	}
	s.Registry, _ = NewRegistry(map[Provider]Adapter{AWS: testAdapter{discover: func(context.Context, Connection, Credentials, string, int) (DiscoveryPage, error) {
		return DiscoveryPage{Coverage: []KindCoverage{{Kind: "eks_cluster", ErrorCode: "insufficient_permissions"}}}, nil
	}}})
	session, err := s.StartServerDiscovery(ctx, a, OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "start"})
	if err != nil {
		t.Fatal(err)
	}
	in := ServerDiscoveryInput{ServiceID: "svc", SessionID: session.ID, IdempotencyKey: "page"}
	if _, err = s.DiscoverServerPage(ctx, a, in); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DiscoverServerPage(ctx, a, in); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"error_code":"insufficient_permissions"`, `"outcome":"partial"`, `"outcome":"replayed"`, d.Connection.ID} {
		if !strings.Contains(logs.String(), want) {
			t.Fatal("missing discovery context", want)
		}
	}
	if strings.Contains(logs.String(), "secret-name") || strings.Contains(logs.String(), "secret-profile") {
		t.Fatal("provider configuration leaked")
	}
}

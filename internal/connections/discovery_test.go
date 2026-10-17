package connections

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestDiscoveryPaginationReplayAndPartialScans(t *testing.T) {
	ctx := context.Background()
	s, actor := setupService(t)
	d, err := s.Configure(ctx, actor, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: GitHub, Name: "Repositories", Mode: AgentManaged, Configuration: Configuration{AccountID: "example"}})
	if err != nil {
		t.Fatal(err)
	}
	start := func(key string) DiscoverySession {
		t.Helper()
		v, err := s.StartAgentDiscovery(ctx, actor, OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: key})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	resource := func(id int) DiscoveredResource {
		return DiscoveredResource{Resource: ResourceKey{Provider: GitHub, Tenant: "example", Kind: "repository", ID: fmt.Sprint(id)}, Name: fmt.Sprintf("example/repo-%02d", id), Locator: fmt.Sprintf("https://github.com/example/repo-%02d", id)}
	}
	session := start("first")
	page := DiscoverySubmission{ServiceID: "svc", SessionID: session.ID, IdempotencyKey: "page", ObservedAt: time.Now().UTC(), Method: "gh api --paginate", Complete: true, Coverage: []KindCoverage{{Kind: "repository", Complete: true, Authoritative: true}}}
	for i := 1; i <= 30; i++ {
		page.Resources = append(page.Resources, resource(i))
	}
	receipt, err := s.SubmitAgentDiscovery(ctx, actor, page)
	if err != nil || receipt.State != Complete || receipt.Mode != AgentManaged {
		t.Fatalf("submission: %+v %v", receipt, err)
	}
	if _, err = s.SubmitAgentDiscovery(ctx, actor, page); err != nil {
		t.Fatal("identical replay failed", err)
	}
	changed := page
	changed.Method = "different method"
	if _, err = s.SubmitAgentDiscovery(ctx, actor, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("different replay accepted")
	}
	query := InventoryQuery{ServiceID: "svc", ConnectionID: d.Connection.ID}
	first, err := s.ListInventory(ctx, actor, query)
	if err != nil || len(first.Resources) != 25 || first.NextCursor == "" {
		t.Fatalf("first page: %+v %v", first, err)
	}
	query.Cursor = first.NextCursor
	second, err := s.ListInventory(ctx, actor, query)
	if err != nil || len(second.Resources) != 5 || second.NextCursor != "" {
		t.Fatalf("second page: %+v %v", second, err)
	}
	query.Search = "repo-01"
	if _, err = s.ListInventory(ctx, actor, query); !errors.Is(err, ErrConflict) {
		t.Fatal("cursor reused with different filter")
	}
	partial := start("partial")
	partialPage := DiscoverySubmission{ServiceID: "svc", SessionID: partial.ID, IdempotencyKey: "one", ObservedAt: time.Now().UTC(), Method: "gh api", Resources: []DiscoveredResource{resource(1)}, Limitations: []string{"rate limited"}}
	if _, err = s.SubmitAgentDiscovery(ctx, actor, partialPage); err != nil {
		t.Fatal(err)
	}
	partialPage.IdempotencyKey, partialPage.Complete, partialPage.Resources, partialPage.Limitations = "finish", true, nil, nil
	partialPage.Coverage = page.Coverage
	if receipt, err = s.SubmitAgentDiscovery(ctx, actor, partialPage); err != nil || receipt.State != PartialDiscovery {
		t.Fatal("earlier limitations were forgotten")
	}
	var available int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM connection_inventory WHERE available`).Scan(&available); err != nil || available != 30 {
		t.Fatal("partial scan removed inventory", available, err)
	}
	complete := start("complete")
	final := DiscoverySubmission{ServiceID: "svc", SessionID: complete.ID, IdempotencyKey: "finish", ObservedAt: time.Now().UTC(), Method: "gh api --paginate", Complete: true, Resources: []DiscoveredResource{resource(1)}, Coverage: page.Coverage}
	if _, err = s.SubmitAgentDiscovery(ctx, actor, final); err != nil {
		t.Fatal(err)
	}
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM connection_inventory WHERE available`).Scan(&available); err != nil || available != 1 {
		t.Fatal("complete scan did not reconcile absence", available, err)
	}
	var records int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM records WHERE kind='evidence_source'`).Scan(&records); err != nil || records != 0 {
		t.Fatal("discovery selected sources automatically")
	}
}

func TestDiscoveryRejectsWrongTenantRegionAndLatePages(t *testing.T) {
	ctx := context.Background()
	s, actor := setupService(t)
	d, err := s.Configure(ctx, actor, ConfigureInput{ServiceID: "svc", IdempotencyKey: "create", Provider: AWS, Name: "Account", Mode: AgentManaged, Configuration: Configuration{AccountID: "123456789012", Region: "us-east-1"}})
	if err != nil {
		t.Fatal(err)
	}
	in := OperationInput{ServiceID: "svc", ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "discover"}
	session, err := s.StartAgentDiscovery(ctx, actor, in)
	if err != nil {
		t.Fatal(err)
	}
	resource := DiscoveredResource{Resource: ResourceKey{Provider: AWS, Tenant: "123456789012", Kind: "account_region", ID: "us-west-2"}, Name: "Account", Locator: "123456789012/us-west-2"}
	page := DiscoverySubmission{ServiceID: "svc", SessionID: session.ID, IdempotencyKey: "page", ObservedAt: time.Now().UTC(), Method: "aws", Resources: []DiscoveredResource{resource}}
	if _, err = s.SubmitAgentDiscovery(ctx, actor, page); !errors.Is(err, ErrInvalid) {
		t.Fatal("unsupported region accepted")
	}
	resource.Resource.ID, resource.Locator, resource.Resource.Tenant = "us-east-1", "123456789012/us-east-1", "999999999999"
	page.Resources[0] = resource
	if _, err = s.SubmitAgentDiscovery(ctx, actor, page); !errors.Is(err, ErrInvalid) {
		t.Fatal("wrong tenant accepted")
	}
	resource.Resource.Tenant = "123456789012"
	page.Resources[0] = resource
	in.IdempotencyKey = "disconnect"
	if _, err = s.ChangeLifecycle(ctx, actor, in, "disconnect"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SubmitAgentDiscovery(ctx, actor, page); !errors.Is(err, ErrInactive) {
		t.Fatal("late page accepted", err)
	}
}

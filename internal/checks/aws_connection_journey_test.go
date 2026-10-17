package checks

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/providers"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestAWSConnectionSelectionCollectionAndAgentParity(t *testing.T) {
	t.Run("legacy", func(t *testing.T) { testAWSConnectionJourney(t, false) })
	t.Run("delegated", func(t *testing.T) { testAWSConnectionJourney(t, true) })
}

func testAWSConnectionJourney(t *testing.T, delegated bool) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_ = r.ParseForm()
			w.Header().Set("Content-Type", "text/xml")
			if delegated && r.Form.Get("Action") == "GetRole" {
				fmt.Fprint(w, `<GetRoleResponse><GetRoleResult><Role><Arn>arn:aws:iam::123456789012:role/reader</Arn><RoleId>reader</RoleId><RoleName>reader</RoleName><Path>/</Path></Role></GetRoleResult></GetRoleResponse>`)
				return
			}
			if delegated && r.Form.Get("Action") == "AssumeRole" {
				if len(r.Form.Get("ExternalId")) != 64 {
					t.Error("generated external ID not used")
				}
				fmt.Fprint(w, `<AssumeRoleResponse><AssumeRoleResult><Credentials><AccessKeyId>ASSUMED</AccessKeyId><SecretAccessKey>secret</SecretAccessKey><SessionToken>session</SessionToken><Expiration>2099-01-01T00:00:00Z</Expiration></Credentials><AssumedRoleUser><Arn>arn:aws:sts::123456789012:assumed-role/Customer/fed</Arn><AssumedRoleId>TARGET:fed</AssumedRoleId></AssumedRoleUser></AssumeRoleResult></AssumeRoleResponse>`)
				return
			}
			if r.Form.Get("Action") != "GetCallerIdentity" {
				t.Error("unexpected STS operation")
			}
			w.Header().Set("Content-Type", "text/xml")
			fmt.Fprint(w, `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Account>123456789012</Account><Arn>arn:aws:iam::123456789012:role/reader</Arn><UserId>reader</UserId></GetCallerIdentityResult></GetCallerIdentityResponse>`)
			return
		}
		if r.Method != "GET" {
			t.Error("infrastructure write attempted")
		}
		if delegated && !strings.Contains(r.Header.Get("Authorization"), "ASSUMED") {
			t.Error("collector used ambient credentials")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/clusters":
			fmt.Fprint(w, `{"clusters":[]}`)
		case "/detector":
			fmt.Fprint(w, `{"detectorIds":["allowed","denied"]}`)
		case "/detector/allowed":
			fmt.Fprint(w, `{"status":"ENABLED","features":[{"name":"EKS_AUDIT_LOGS","status":"ENABLED"}]}`)
		case "/detector/denied":
			w.WriteHeader(403)
			fmt.Fprint(w, `{"message":"denied"}`)
		default:
			t.Errorf("unexpected AWS endpoint %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	provider := providers.AWS{FedRoleARN: "arn:aws:iam::123456789012:role/reader", Load: func(ctx context.Context, options ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
		var configured awsconfig.LoadOptions
		for _, option := range options {
			if err := option(&configured); err != nil {
				t.Fatal(err)
			}
		}
		expectedProfile := "read-only"
		if delegated {
			expectedProfile = ""
		}
		if configured.SharedConfigProfile != expectedProfile || configured.Region != "us-east-1" {
			t.Fatal("connection credentials or region not applied")
		}
		return aws.Config{Region: "us-east-1", BaseEndpoint: aws.String(server.URL), HTTPClient: server.Client(), Credentials: credentials.NewStaticCredentialsProvider("test", "secret", "")}, nil
	}}
	ctx := context.Background()
	db := testutil.Database(t)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	store := assessment.Store{DB: db}
	service, err := store.CreateService(ctx, a, "AWS")
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := connections.NewRegistry(map[connections.Provider]connections.Adapter{connections.AWS: provider})
	s := &connections.Service{DB: db, Registry: registry, Credentials: connections.CredentialStore{DB: db}}
	var d connections.Detail
	if delegated {
		s.Credentials.Keys, _ = connections.NewKeyring("test", map[string]string{"test": base64.StdEncoding.EncodeToString(make([]byte, 32))})
		setup, e := s.BeginSetup(ctx, a, connections.BeginSetupInput{ServiceID: service.ID, Provider: connections.AWS, IdempotencyKey: "begin", Input: connections.SetupInput{Name: "AWS", AccountID: "123456789012"}})
		if e != nil {
			t.Fatal(e)
		}
		op := connections.SetupOperationInput{ServiceID: service.ID, SetupID: setup.ID, ExpectedRevision: setup.Revision, IdempotencyKey: "configure"}
		material, e := s.AWSSetupMaterial(ctx, a, op)
		if e != nil {
			t.Fatal(e)
		}
		setup.Input.RoleARN = material.RoleARN
		setup, e = s.UpdateSetup(ctx, a, op, setup.Input)
		if e != nil {
			t.Fatal(e)
		}
		op.ExpectedRevision = setup.Revision
		setup, e = s.VerifyAWSSetup(ctx, a, op)
		if e != nil {
			t.Fatal(e)
		}
		op.ExpectedRevision = setup.Revision
		op.IdempotencyKey = "complete"
		setup, e = s.CompleteSetup(ctx, a, op)
		if e != nil {
			t.Fatal(e)
		}
		d, err = s.Get(ctx, a, service.ID, setup.ConnectionID)
	} else {
		d, err = s.Configure(ctx, a, connections.ConfigureInput{ServiceID: service.ID, IdempotencyKey: "configure", Provider: connections.AWS, Name: "AWS", Mode: connections.ServerEnvironment, Configuration: connections.Configuration{AccountID: "123456789012", Region: "us-east-1", Profile: "read-only"}})
	}
	if err != nil {
		t.Fatal(err)
	}
	in := connections.OperationInput{ServiceID: service.ID, ConnectionID: d.Connection.ID, ExpectedRevision: 1, Generation: 1, IdempotencyKey: "test"}
	if result, err := s.TestWithReceipt(ctx, a, in); err != nil || result.State != "succeeded" {
		t.Fatal("identity test", err)
	}
	in.IdempotencyKey = "discover"
	session, err := s.StartServerDiscovery(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DiscoverServerPage(ctx, a, connections.ServerDiscoveryInput{ServiceID: service.ID, SessionID: session.ID, IdempotencyKey: "page", Limit: 2}); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListInventory(ctx, a, connections.InventoryQuery{ServiceID: service.ID, ConnectionID: d.Connection.ID})
	if err != nil || len(page.Resources) != 1 {
		t.Fatal("regional inventory", err)
	}
	resource := page.Resources[0].DiscoveredResource
	if _, err = s.SelectSources(ctx, a, connections.SelectionInput{ServiceID: service.ID, IdempotencyKey: "select", AddToScope: true, Sources: []connections.SourceConfigurationInput{{ServiceID: service.ID, SourceID: "aws", ConnectionID: d.Connection.ID, Generation: 1, Resource: resource.Resource, Source: assessment.EvidenceSource{Type: "aws", Name: "Account", Locator: resource.Locator, AccountID: "123456789012", Region: "us-east-1", Enabled: true, FreshnessHours: 24}}}}); err != nil {
		t.Fatal(err)
	}
	runner := Runner{DB: db, Connections: s}
	scope := assessment.Scope{Accounts: []assessment.Account{{ID: "123456789012", Regions: []string{"us-east-1"}}}}
	result, bindings, err := runner.collectConnected(ctx, a, service.ID, AWS{Kind: "guardduty"}, scope, assessment.Measure{Expected: []string{"EKS_AUDIT_LOGS"}})
	if err != nil || len(bindings) != 1 || result.Observed != 1 || result.Expected != 2 {
		t.Fatalf("partial collection lost successes: %+v %v", result, err)
	}
	in.IdempotencyKey = "agent-discover"
	agentSession, err := s.StartAgentDiscovery(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if receipt, err := s.SubmitAgentDiscovery(ctx, a, connections.DiscoverySubmission{ServiceID: service.ID, SessionID: agentSession.ID, IdempotencyKey: "page", ObservedAt: time.Now().UTC(), Method: "aws cli", Resources: []connections.DiscoveredResource{resource}}); err != nil || receipt.Mode != connections.AgentManaged {
		t.Fatal("agent parity attribution", err)
	}
	in.IdempotencyKey = "renew"
	expectedHealth := connections.AuthorizationRequired
	if delegated {
		expectedHealth = connections.NotConfigured
	}
	if renewed, err := s.Renew(ctx, a, in); err != nil || renewed.Access == nil || renewed.Access.State != expectedHealth {
		t.Fatal("environment renewal instructions missing", err)
	}
	if delegated {
		in.Generation++
	}
	in.IdempotencyKey = "disconnect"
	if _, err = s.ChangeLifecycle(ctx, a, in, "disconnect"); err != nil {
		t.Fatal(err)
	}
	result, _, err = runner.collectConnected(ctx, a, service.ID, AWS{Kind: "guardduty"}, scope, assessment.Measure{})
	if err != nil || result.Observed != 0 {
		t.Fatal("disconnected AWS binding collected", err)
	}
}

package connections

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
)

func TestOperatorKubernetesProvisioning(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	s.Credentials.Keys, _ = NewKeyring("v1", map[string]string{"v1": base64.StdEncoding.EncodeToString(make([]byte, 32))})
	adapter := testAdapter{test: func(_ context.Context, c Connection, k Credentials) (AccessHealth, error) {
		if k.Value("kubeconfig") == "invalid" {
			return AccessHealth{}, ErrInvalid
		}
		return AccessHealth{State: Verified, VerifiedAccountID: c.Tenant()}, nil
	}}
	s.Registry, _ = NewRegistry(map[Provider]Adapter{Kubernetes: adapter})
	provision := func(secret string) (Detail, error) {
		return s.ProvisionKubernetes(ctx, a, "svc", "Production", "cluster-uid", "https://cluster.example", secret)
	}
	first, err := provision("secret-one")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := provision("secret-one")
	if err != nil || replay.Connection.Generation != 1 || replay.Connection.ID != first.Connection.ID {
		t.Fatal("provisioning not idempotent", err)
	}
	rotated, err := provision("secret-two")
	if err != nil || rotated.Connection.Generation != 2 || rotated.Connection.Revision != 2 {
		t.Fatal("rotation did not fence generation", err)
	}
	if _, err = s.Credentials.Get(ctx, a, "svc", first.Connection.ID, "check", 1); !errors.Is(err, ErrConflict) {
		t.Fatal("old generation still usable", err)
	}
	if _, err = provision("invalid"); err == nil {
		t.Fatal("invalid credential accepted")
	}
	current, err := s.Credentials.Get(ctx, a, "svc", first.Connection.ID, "check", 2)
	if err != nil || current.Value("kubeconfig") != "secret-two" {
		t.Fatal("failed verification replaced credentials", err)
	}
	var history string
	var audits int
	if err = s.DB.QueryRow(ctx, `SELECT string_agg(data::text,'') FROM connection_revisions WHERE connection_id=$1`, first.Connection.ID).Scan(&history); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(history, "secret-one") || strings.Contains(history, "secret-two") {
		t.Fatal("credential in metadata history")
	}
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action='connection.operator_provisioned'`).Scan(&audits); err != nil || audits != 2 {
		t.Fatal("missing audit or replay mutated history", audits, err)
	}
	token, err := auth.IssueToken(ctx, s.DB, a, "svc", []string{"connect", "read"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := auth.TokenActor(ctx, s.DB, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ProvisionKubernetes(ctx, agent, "svc", "Production", "cluster-uid", "https://cluster.example", "secret-three"); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("agent provisioned operator credentials", err)
	}
	_, err = s.ChangeLifecycle(ctx, a, OperationInput{ServiceID: "svc", ConnectionID: first.Connection.ID, Generation: 2, ExpectedRevision: 2, IdempotencyKey: "disconnect"}, "disconnect")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provision("secret-three"); !errors.Is(err, ErrInactive) {
		t.Fatal("provisioning resurrected disconnected connection", err)
	}
}

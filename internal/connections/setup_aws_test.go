package connections

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
)

type setupAWSAdapter struct{ testAdapter }

func (setupAWSAdapter) SetupPrincipal(context.Context) (string, error) {
	return "arn:aws:iam::999999999999:role/Fed", nil
}
func (setupAWSAdapter) SetupMaterial(account, external, role string) (SetupMaterial, error) {
	b, _ := json.Marshal(map[string]string{"external_id": external})
	return SetupMaterial{TrustPolicy: b, RoleARN: role}, nil
}

func TestAWSSetupExternalIDLifecycleAndGeneration(t *testing.T) {
	ctx := context.Background()
	s, a := setupService(t)
	s.Credentials.Keys, _ = NewKeyring("test", map[string]string{"test": base64.StdEncoding.EncodeToString(make([]byte, 32))})
	s.Registry, _ = NewRegistry(map[Provider]Adapter{AWS: setupAWSAdapter{testAdapter{test: func(_ context.Context, c Connection, k Credentials) (AccessHealth, error) {
		if c.Strategy != DelegatedRole || k.Value("external_id") == "" {
			t.Fatal("verification omitted delegated binding")
		}
		return AccessHealth{State: Verified, VerifiedAccountID: c.Configuration.AccountID}, nil
	}}}})
	in := BeginSetupInput{ServiceID: "svc", Provider: AWS, Input: SetupInput{Name: "AWS", AccountID: "123456789012", RoleARN: "arn:aws:iam::123456789012:role/Customer"}, IdempotencyKey: "begin"}
	setup, err := s.BeginSetup(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	op := SetupOperationInput{ServiceID: "svc", SetupID: setup.ID, ExpectedRevision: 1, IdempotencyKey: "configure"}
	setup, err = s.UpdateSetup(ctx, a, op, setup.Input)
	if err != nil {
		t.Fatal(err)
	}
	op.ExpectedRevision = setup.Revision
	first, err := s.AWSSetupMaterial(ctx, a, op)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := s.AWSSetupMaterial(ctx, a, op)
	if err != nil || string(first.TrustPolicy) != string(repeated.TrustPolicy) {
		t.Fatal("retry changed external ID", err)
	}
	setup, err = s.VerifyAWSSetup(ctx, a, op)
	if err != nil {
		t.Fatal(err)
	}
	op.ExpectedRevision = setup.Revision
	op.IdempotencyKey = "complete"
	complete, err := s.CompleteSetup(ctx, a, op)
	if err != nil {
		t.Fatal(err)
	}
	in.ConnectionID = complete.ConnectionID
	in.Generation = 1
	in.ExpectedRevision = 1
	in.IdempotencyKey = "reconnect"
	reconnect, err := s.BeginSetup(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	op.SetupID = reconnect.ID
	op.ExpectedRevision = reconnect.Revision
	op.IdempotencyKey = "material"
	retained, err := s.AWSSetupMaterial(ctx, a, op)
	if err != nil || string(retained.TrustPolicy) != string(first.TrustPolicy) {
		t.Fatal("reauthorization rotated external ID", err)
	}
	op.IdempotencyKey = "rotate"
	rotated, err := s.RotateAWSSetupExternalID(ctx, a, op)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Credentials.Get(ctx, a, "svc", complete.ConnectionID, "connect", 1); err == nil {
		t.Fatal("old generation remained usable")
	}
	op.ExpectedRevision = rotated.Revision
	replaced, err := s.AWSSetupMaterial(ctx, a, op)
	if err != nil || string(replaced.TrustPolicy) == string(first.TrustPolicy) {
		t.Fatal("external ID did not change", err)
	}
	rotated, err = s.VerifyAWSSetup(ctx, a, op)
	if err != nil {
		t.Fatal(err)
	}
	op.ExpectedRevision = rotated.Revision
	op.IdempotencyKey = "finish-rotation"
	if _, err = s.CompleteSetup(ctx, a, op); err != nil {
		t.Fatal(err)
	}
	detail, err := s.Get(ctx, a, "svc", complete.ConnectionID)
	if err != nil || detail.Connection.Generation != 3 || detail.Connection.Revision != 1 {
		t.Fatal("rotation changed metadata or missed generation fence", err)
	}
}

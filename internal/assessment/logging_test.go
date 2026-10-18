package assessment

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestCommittedEventsExcludeTokenAndEvidence(t *testing.T) {
	db := testutil.Database(t)
	var logs bytes.Buffer
	l, _ := logging.New(&logs, nil, "debug", "json", "server")
	ctx := logging.WithLogger(context.Background(), l)
	a, err := auth.BootstrapAdmin(ctx, db, "test", "logging")
	if err != nil {
		t.Fatal(err)
	}
	s := Store{DB: db}
	service, err := s.CreateService(ctx, a, "secret-service-name")
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{Description: "secret-scope-description"}
	if _, err = s.SaveScope(ctx, a, service.ID, 0, scope); err != nil {
		t.Fatal(err)
	}
	committed := strings.Count(logs.String(), `"event":"assessment.record.committed"`)
	if _, err = s.SaveScope(ctx, a, service.ID, 0, scope); err == nil {
		t.Fatal("conflict not rejected")
	}
	if strings.Count(logs.String(), `"event":"assessment.record.committed"`) != committed {
		t.Fatal("conflict reported success")
	}
	token, err := auth.IssueToken(ctx, db, a, service.ID, []string{"read", "connect"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err = auth.RevokeToken(ctx, db, a, service.ID, storage.Hash([]byte(token))); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{token, storage.Hash([]byte(token)), "secret-service-name", "secret-scope-description"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("sensitive field logged")
		}
	}
	for _, event := range []string{"assessment.service.created", "assessment.record.committed", "auth.token.issued", "auth.token.revoked"} {
		if !strings.Contains(logs.String(), event) {
			t.Fatal("missing event", event)
		}
	}
	if !strings.Contains(logs.String(), `"lifetime_seconds":3600`) || !strings.Contains(logs.String(), `"actions":["read","connect"]`) {
		t.Fatal("missing token metadata")
	}
}

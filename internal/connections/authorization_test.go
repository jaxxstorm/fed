package connections

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/jaxxstorm/fed/internal/testutil"
)

func TestConnectRequiresExplicitTokenGrant(t *testing.T) {
	ctx := context.Background()
	db := testutil.Database(t)
	admin, err := auth.BootstrapAdmin(ctx, db, "test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO services(id,organization_id,name) VALUES('svc',$1,'Test'),('other',$1,'Other')`, admin.OrganizationID); err != nil {
		t.Fatal(err)
	}
	for _, connect := range []bool{false, true} {
		actions := []string{"read", "edit", "check", "export"}
		if connect {
			actions = append(actions, "connect")
		}
		token, err := auth.IssueToken(ctx, db, admin, "svc", actions, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		agent, err := auth.TokenActor(ctx, db, token)
		if err != nil {
			t.Fatal(err)
		}
		err = auth.Authorize(ctx, db, agent, "svc", "connect")
		if connect && err != nil || !connect && !errors.Is(err, auth.ErrForbidden) {
			t.Fatalf("connect=%v: %v", connect, err)
		}
		for _, boundary := range [][2]string{{"svc", "review"}, {"svc", "admin"}, {"other", "connect"}} {
			if !errors.Is(auth.Authorize(ctx, db, agent, boundary[0], boundary[1]), auth.ErrForbidden) {
				t.Fatal("token escaped boundary")
			}
		}
		if err = auth.RevokeToken(ctx, db, admin, "svc", storage.Hash([]byte(token))); err != nil {
			t.Fatal(err)
		}
		if !errors.Is(auth.Authorize(ctx, db, agent, "svc", "connect"), auth.ErrForbidden) {
			t.Fatal("revoked connection permission accepted")
		}
	}
}

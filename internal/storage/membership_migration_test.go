package storage

import (
	"context"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func legacyMembershipDatabase(t *testing.T) *pgxpool.Pool {
	return databaseBeforeMigration(t, "migrations/014_")
}

func databaseBeforeMigration(t *testing.T, before string) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("KSI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set KSI_TEST_DATABASE_URL for migration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_membership_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		_, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
		if err != nil {
			t.Error(err)
		}
	})
	if _, err = db.Exec(ctx, `CREATE TABLE schema_migrations(name text PRIMARY KEY,hash text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	names, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	for _, name := range names {
		if name >= before {
			break
		}
		b, err := migrationFiles.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, string(b)); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO schema_migrations(name,hash) VALUES($1,$2)`, name, Hash(b)); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestMembershipMigrationPreservesIdentityAndHistory(t *testing.T) {
	ctx := context.Background()
	db := legacyMembershipDatabase(t)
	_, err := db.Exec(ctx, `INSERT INTO organizations(id,name) VALUES('existing','Existing');
 INSERT INTO identities(id,organization_id,kind,issuer,subject,name) VALUES('admin','existing','human','issuer','subject','Admin');
 INSERT INTO permissions VALUES('admin','*','admin');
 INSERT INTO services(id,organization_id,name) VALUES('svc','existing','Service');
 INSERT INTO records(service_id,kind,id,revision,requirement_id,actor_id,data) VALUES('svc','scope','scope',1,'','admin','{"description":"preserve"}');
 INSERT INTO sessions(hash,identity_id,csrf_token,expires_at) VALUES('session','admin','csrf',now()+interval '1 hour');`)
	if err != nil {
		t.Fatal(err)
	}
	var before, after string
	if err = db.QueryRow(ctx, `SELECT md5(string_agg(row_to_json(r)::text,'' ORDER BY id)) FROM records r`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal("repeat", err)
	}
	if err = db.QueryRow(ctx, `SELECT md5(string_agg(row_to_json(r)::text,'' ORDER BY id)) FROM records r`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("history changed")
	}
	var org, role, state string
	var complete bool
	var sessions int
	if err = db.QueryRow(ctx, `SELECT s.organization_id,s.bootstrap_complete,m.role,m.state FROM organization_settings s JOIN organization_memberships m USING(organization_id) WHERE m.identity_id='admin'`).Scan(&org, &complete, &role, &state); err != nil {
		t.Fatal(err)
	}
	if org != "existing" || !complete || role != "admin" || state != "active" {
		t.Fatal(org, complete, role, state)
	}
	if err = db.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatal(sessions, err)
	}
}

func TestMembershipMigrationRejectsAmbiguityAtomically(t *testing.T) {
	for _, fixture := range []string{
		`INSERT INTO organizations VALUES('one','One'),('two','Two'); INSERT INTO services(id,organization_id,name) VALUES('s1','one','One'),('s2','two','Two');`,
		`INSERT INTO organizations VALUES('one','One'); INSERT INTO identities(id,organization_id,kind,subject,name) VALUES('limited','one','human','limited','Limited'); INSERT INTO permissions VALUES('limited','svc','read');`,
	} {
		t.Run(fixture[:35], func(t *testing.T) {
			db := legacyMembershipDatabase(t)
			ctx := context.Background()
			if _, err := db.Exec(ctx, fixture); err != nil {
				t.Fatal(err)
			}
			if err := Migrate(ctx, db); err == nil {
				t.Fatal("ambiguous migration accepted")
			}
			var exists bool
			if err := db.QueryRow(ctx, `SELECT to_regclass('organization_settings') IS NOT NULL`).Scan(&exists); err != nil || exists {
				t.Fatal("partial migration", err)
			}
		})
	}
}

func TestMembershipMigrationFreshConstraints(t *testing.T) {
	ctx := context.Background()
	db := legacyMembershipDatabase(t)
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var complete bool
	if err := db.QueryRow(ctx, `SELECT bootstrap_complete FROM organization_settings`).Scan(&complete); err != nil || complete {
		t.Fatal("fresh bootstrap", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO identities(id,organization_id,kind,subject,name) VALUES('new','default','human','new','New'); INSERT INTO organization_memberships(identity_id,organization_id,email,email_verified) VALUES('new','default','new@example.com',true)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE organization_memberships SET state='active' WHERE identity_id='new'`); err == nil {
		t.Fatal("active without role")
	}
	if _, err := db.Exec(ctx, `UPDATE organization_memberships SET role='admin' WHERE identity_id='new'`); err == nil {
		t.Fatal("pending with role")
	}
	if _, err := db.Exec(ctx, `UPDATE organization_memberships SET category='guest' WHERE identity_id='new'`); err == nil {
		t.Fatal("unsupported guest")
	}
}

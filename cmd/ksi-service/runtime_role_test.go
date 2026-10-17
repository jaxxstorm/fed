package main

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

func TestProvisionRuntimeRejectsUnsafePassword(t *testing.T) {
	for _, password := range []string{"", "short", strings.Repeat("a", 40) + "'"} {
		if err := provisionRuntime(context.Background(), nil, password); err == nil {
			t.Fatal("unsafe password accepted")
		}
	}
}

// Use only an isolated, disposable database: this command intentionally manages
// a cluster-level login and grants on the application's public schema.
func TestProvisionRuntimePermissions(t *testing.T) {
	dsn := os.Getenv("KSI_RUNTIME_ROLE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires a disposable database via KSI_RUNTIME_ROLE_TEST_DATABASE_URL")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	password := strings.Repeat("a", 40)
	for range 2 {
		if err := provisionRuntime(ctx, db, password); err != nil {
			t.Fatal(err)
		}
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword("ksi_runtime", password)
	runtime, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := auth.BootstrapAdmin(ctx, runtime, "https://test.invalid", "test-subject"); err != nil {
		t.Fatalf("runtime cannot bootstrap administrator: %v", err)
	}
	for _, sql := range []string{
		`CREATE TABLE forbidden(id int)`,
		`DELETE FROM schema_migrations`,
		`DELETE FROM catalogs`,
		`DELETE FROM requirements`,
		`DELETE FROM catalog_activation`,
	} {
		if _, err := runtime.Exec(ctx, sql); err == nil {
			t.Fatalf("runtime unexpectedly permitted: %s", sql)
		}
	}
	var count int
	if err := runtime.QueryRow(ctx, `SELECT count(*) FROM catalogs`).Scan(&count); err != nil {
		t.Fatal("runtime cannot read catalog")
	}
}

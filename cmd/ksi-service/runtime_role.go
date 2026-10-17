package main

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// provisionRuntime runs only with the migration credential. Runtime processes
// get a separate DML-only login; they cannot migrate schemas or replace catalogs.
// Repeat after each migration so newly created tables receive the same grants.
func provisionRuntime(ctx context.Context, db *pgxpool.Pool, password string) error {
	// Password DDL does not support bind parameters. Restrict the generated secret
	// to an injection-safe alphabet, and never return SQL/driver errors to logs.
	if !regexp.MustCompile(`^[A-Za-z0-9]{40,128}$`).MatchString(password) {
		return errors.New("runtime password must contain 40-128 alphanumeric characters")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := db.Begin(ctx)
	if err != nil {
		return errors.New("runtime role provisioning failed")
	}
	defer tx.Rollback(ctx)
	statements := []string{
		`SELECT pg_advisory_xact_lock(817034)`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='ksi_runtime') THEN CREATE ROLE ksi_runtime LOGIN; END IF; END $$`,
		// RDS administrators are not PostgreSQL superusers. These attributes
		// default to false; verify them rather than using superuser-only ALTERs.
		`DO $$ BEGIN IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='ksi_runtime' AND (rolsuper OR rolreplication OR rolbypassrls)) THEN RAISE EXCEPTION 'runtime role is privileged'; END IF; END $$`,
		`ALTER ROLE ksi_runtime WITH LOGIN NOCREATEDB NOCREATEROLE PASSWORD '` + password + `'`,
		`REVOKE CREATE ON SCHEMA public FROM PUBLIC`,
		`GRANT USAGE ON SCHEMA public TO ksi_runtime`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO ksi_runtime`,
		`GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO ksi_runtime`,
		`REVOKE ALL ON schema_migrations FROM ksi_runtime`,
		`REVOKE INSERT, UPDATE, DELETE ON catalogs, requirements, catalog_activation FROM ksi_runtime`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return errors.New("runtime role provisioning failed")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("runtime role provisioning failed")
	}
	return nil
}

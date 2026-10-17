package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/config"
	"github.com/jaxxstorm/fed/internal/connections"
)

func maintainConnections(ctx context.Context, db *pgxpool.Pool, cfg config.Config, command string) error {
	issuer, subject := cfg.OIDCIssuer, cfg.AdminSubject
	if cfg.DevIdentity {
		issuer, subject = "local-development", "developer"
	}
	if issuer == "" || subject == "" {
		return errors.New("connection maintenance requires an administrator identity configuration")
	}
	a, err := auth.BootstrapAdmin(ctx, db, issuer, subject)
	if err != nil {
		return errors.New("connection maintenance identity unavailable")
	}
	store := assessment.Store{DB: db}
	services, err := store.Services(ctx, a)
	if err != nil {
		return errors.New("connection maintenance service listing failed")
	}
	registry, _ := connections.NewRegistry(nil)
	s := connections.Service{DB: db, Registry: registry, Credentials: connections.CredentialStore{DB: db}}
	bindings := connections.EnvironmentBindings{}
	if command == "import-connections" {
		for name, target := range map[string]any{"KSI_AWS_ACCOUNT_PROFILES": &bindings.AWSProfiles, "KSI_KUBERNETES_CONTEXTS": &bindings.KubernetesContexts} {
			if raw := os.Getenv(name); raw != "" {
				if json.Unmarshal([]byte(raw), target) != nil {
					return errors.New("invalid legacy provider bindings")
				}
			}
		}
	} else {
		keys, err := connections.LoadKeyring()
		if err != nil {
			return err
		}
		s.Credentials.Keys = keys
	}
	for _, service := range services {
		if command == "import-connections" {
			if _, err = s.ImportSources(ctx, a, service.ID, bindings); err != nil {
				return errors.New("connection import failed")
			}
		} else {
			rows, err := db.Query(ctx, `SELECT c.id,c.generation FROM evidence_connections c JOIN connection_credentials k ON k.service_id=c.service_id AND k.connection_id=c.id WHERE c.service_id=$1 AND c.lifecycle='active'`, service.ID)
			if err != nil {
				return errors.New("credential rotation inventory failed")
			}
			type target struct {
				id         string
				generation int64
			}
			targets := []target{}
			for rows.Next() {
				var item target
				if err = rows.Scan(&item.id, &item.generation); err != nil {
					rows.Close()
					return errors.New("credential rotation inventory failed")
				}
				targets = append(targets, item)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return errors.New("credential rotation inventory failed")
			}
			for _, item := range targets {
				if err = s.Credentials.RotateKey(ctx, a, service.ID, item.id, item.generation); err != nil {
					return errors.New("credential rotation failed; retain old keys and retry")
				}
			}
		}
	}
	slog.Info("connection maintenance completed", "operation", command, "services", len(services))
	return nil
}

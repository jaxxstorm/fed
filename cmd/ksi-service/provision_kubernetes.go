package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/config"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/logging"
	"github.com/jaxxstorm/fed/internal/providers"
	"github.com/rs/zerolog"
)

type provisionedCluster struct {
	ServiceID     string `json:"service_id"`
	Name          string `json:"name"`
	ClusterID     string `json:"cluster_id"`
	Server        string `json:"server"`
	Kubeconfig    string `json:"kubeconfig"`
	SelectCluster bool   `json:"select_cluster"`
}

func provisionKubernetes(ctx context.Context, db *pgxpool.Pool, cfg config.Config) error {
	raw := os.Getenv("KSI_PROVISION_KUBERNETES")
	if len(raw) == 0 || len(raw) > connections.MaxResponseBytes {
		return errors.New("invalid Kubernetes provisioning input")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var inputs []provisionedCluster
	if decoder.Decode(&inputs) != nil || len(inputs) == 0 || len(inputs) > 20 || decoder.Decode(new(any)) != io.EOF {
		return errors.New("invalid Kubernetes provisioning input")
	}
	var owner bool
	if err := db.QueryRow(ctx, `SELECT pg_get_userbyid(relowner)=current_user FROM pg_class WHERE oid='evidence_connections'::regclass`).Scan(&owner); err != nil || !owner {
		return auth.ErrForbidden
	}
	a, err := auth.ResolveAdministrator(ctx, db, cfg.OIDCIssuer, cfg.AdminSubject, cfg.AdminEmail)
	if err != nil {
		return errors.New("provisioning administrator unavailable")
	}
	adapters, err := providers.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	registry, err := connections.NewRegistry(adapters)
	if err != nil {
		return err
	}
	keys, err := connections.LoadKeyring()
	if err != nil {
		return err
	}
	s := connections.Service{DB: db, Registry: registry, Credentials: connections.CredentialStore{DB: db, Keys: keys}}
	for _, in := range inputs {
		d, err := s.ProvisionKubernetes(ctx, a, in.ServiceID, in.Name, in.ClusterID, in.Server, in.Kubeconfig)
		if err != nil {
			return errors.New("Kubernetes connection provisioning failed")
		}
		if in.SelectCluster {
			if err = selectProvisionedCluster(ctx, &s, a, d); err != nil {
				return fmt.Errorf("Kubernetes source selection failed: %w", err)
			}
		}
		logging.Emit(ctx, zerolog.InfoLevel, logging.Fields{Event: "runtime.kubernetes.provisioned", Component: "runtime", ServiceID: in.ServiceID, ConnectionID: d.Connection.ID, Outcome: "success"})
	}
	return nil
}

func selectProvisionedCluster(ctx context.Context, s *connections.Service, a auth.Actor, d connections.Detail) error {
	c := d.Connection
	// Replays must not re-enable a source deliberately disabled by an operator.
	var exists bool
	if err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_source_bindings WHERE service_id=$1 AND provider='kubernetes' AND tenant=$2 AND resource_kind='cluster' AND resource_id=$2)`, c.ServiceID, c.Tenant()).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	op := connections.OperationInput{ServiceID: c.ServiceID, ConnectionID: c.ID, ExpectedRevision: c.Revision, Generation: c.Generation, IdempotencyKey: uuid.NewString()}
	session, err := s.StartServerDiscovery(ctx, a, op)
	if err != nil {
		return err
	}
	cursor := ""
	for page := 0; ; page++ {
		if page >= 100 {
			return errors.New("Kubernetes inventory page limit reached")
		}
		receipt, err := s.DiscoverServerPage(ctx, a, connections.ServerDiscoveryInput{ServiceID: c.ServiceID, SessionID: session.ID, IdempotencyKey: uuid.NewString(), Cursor: cursor, Limit: 100})
		if err != nil {
			return err
		}
		if receipt.NextCursor == "" {
			break
		}
		cursor = receipt.NextCursor
	}
	var revision int
	if err = s.DB.QueryRow(ctx, `SELECT revision FROM records WHERE service_id=$1 AND kind='scope' AND id='scope' ORDER BY revision DESC LIMIT 1`, c.ServiceID).Scan(&revision); err != nil {
		return err
	}
	id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("fed:operator:source:"+c.ServiceID+":"+c.Tenant())).String()
	_, err = s.SelectSources(ctx, a, connections.SelectionInput{ServiceID: c.ServiceID, ExpectedScopeRevision: revision, IdempotencyKey: uuid.NewString(), AddToScope: true, Sources: []connections.SourceConfigurationInput{{ServiceID: c.ServiceID, SourceID: id, ConnectionID: c.ID, Generation: c.Generation, Resource: connections.ResourceKey{Provider: connections.Kubernetes, Tenant: c.Tenant(), Kind: "cluster", ID: c.Tenant()}, Source: assessment.EvidenceSource{Name: c.Name, Type: "kubernetes", ClusterID: c.Tenant(), Locator: "kubernetes://cluster/" + url.PathEscape(c.Tenant()) + "/cluster/" + url.PathEscape(c.Tenant()), Enabled: true, ScopeRevision: revision, FreshnessHours: 48}}}})
	return err
}

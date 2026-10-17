package connections

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type SourceConfigurationInput struct {
	ServiceID               string                    `json:"service_id"`
	SourceID                string                    `json:"source_id"`
	ConnectionID            string                    `json:"connection_id"`
	Generation              int64                     `json:"generation"`
	ExpectedRevision        int                       `json:"expected_revision"`
	ExpectedBindingRevision int                       `json:"expected_binding_revision"`
	IdempotencyKey          string                    `json:"idempotency_key"`
	Resource                ResourceKey               `json:"resource"`
	Source                  assessment.EvidenceSource `json:"source"`
}

func (s *Service) ConfigureSource(ctx context.Context, a auth.Actor, in SourceConfigurationInput) (assessment.Record, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return assessment.Record{}, err
	}
	defer tx.Rollback(ctx)
	out, err := s.configureSourceTx(ctx, tx, a, in)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (s *Service) configureSourceTx(ctx context.Context, tx pgx.Tx, a auth.Actor, in SourceConfigurationInput) (assessment.Record, error) {
	if in.SourceID == "" || in.ExpectedRevision < 0 || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 {
		return assessment.Record{}, ErrInvalid
	}
	var err error
	for _, action := range []string{"connect", "edit"} {
		if err = auth.Authorize(ctx, tx, a, in.ServiceID, action); err != nil {
			return assessment.Record{}, err
		}
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return assessment.Record{}, err
	}
	b, _ := json.Marshal(in)
	digest := storage.Hash(b)
	var priorHash string
	var prior []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation='source.configure' AND key=$3`, a.ID, in.ServiceID, in.IdempotencyKey).Scan(&priorHash, &prior)
	if err == nil {
		if priorHash != digest {
			return assessment.Record{}, ErrConflict
		}
		var out assessment.Record
		if err = json.Unmarshal(prior, &out); err != nil {
			return out, err
		}
		return out, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return assessment.Record{}, err
	}
	if _, err = lockCredentialConnection(ctx, tx, a, in.ServiceID, in.ConnectionID, "connect", in.Generation); err != nil {
		return assessment.Record{}, err
	}
	d, err := readConnection(ctx, tx, in.ServiceID, in.ConnectionID)
	if err != nil {
		return assessment.Record{}, err
	}
	if in.Resource.Provider != d.Connection.Provider || in.Resource.Tenant != d.Connection.Tenant() || in.Resource.ID == "" {
		return assessment.Record{}, ErrInvalid
	}
	v := in.Source
	if in.ExpectedRevision == 0 {
		if in.ExpectedBindingRevision != 0 {
			return assessment.Record{}, ErrConflict
		}
		if d.Connection.Mode == Manual && d.Connection.Provider == Documents {
			u, e := url.Parse(v.Locator)
			if e != nil || u.Scheme != "https" || u.Host != in.Resource.Tenant || in.Resource.ID != v.Locator || in.Resource.Kind != v.Type || (v.Type != "document" && v.Type != "operational_records") {
				return assessment.Record{}, ErrInvalid
			}
		} else {
			var raw []byte
			err = tx.QueryRow(ctx, `SELECT data FROM connection_inventory WHERE service_id=$1 AND connection_id=$2 AND tenant=$3 AND resource_kind=$4 AND resource_id=$5 AND available`, in.ServiceID, in.ConnectionID, in.Resource.Tenant, in.Resource.Kind, in.Resource.ID).Scan(&raw)
			if err != nil {
				return assessment.Record{}, ErrInvalid
			}
			var resource DiscoveredResource
			if json.Unmarshal(raw, &resource) != nil || !sourceMatchesResource(v, resource) {
				return assessment.Record{}, ErrInvalid
			}
		}
		state := Enabled
		if !v.Enabled {
			state = Disabled
		}
		var exists bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_source_bindings WHERE service_id=$1 AND (source_id=$2 OR (provider=$3 AND tenant=$4 AND resource_kind=$5 AND resource_id=$6)))`, in.ServiceID, in.SourceID, in.Resource.Provider, in.Resource.Tenant, in.Resource.Kind, in.Resource.ID).Scan(&exists)
		if err != nil {
			return assessment.Record{}, err
		}
		if exists {
			return assessment.Record{}, ErrConflict
		}
		_, err = tx.Exec(ctx, `INSERT INTO connection_source_bindings(service_id,source_id,connection_id,provider,tenant,resource_kind,resource_id,lifecycle,revision,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,1,$9)`, in.ServiceID, in.SourceID, in.ConnectionID, in.Resource.Provider, in.Resource.Tenant, in.Resource.Kind, in.Resource.ID, state, a.ID)
		if err != nil {
			return assessment.Record{}, err
		}
		v.ConnectionID, v.ProviderTenant, v.ResourceKind, v.ResourceID, v.Lifecycle = in.ConnectionID, in.Resource.Tenant, in.Resource.Kind, in.Resource.ID, string(state)
	} else {
		var binding SourceBinding
		err = tx.QueryRow(ctx, `SELECT connection_id,provider,tenant,resource_kind,resource_id,lifecycle,revision FROM connection_source_bindings WHERE service_id=$1 AND source_id=$2`, in.ServiceID, in.SourceID).Scan(&binding.ConnectionID, &binding.Resource.Provider, &binding.Resource.Tenant, &binding.Resource.Kind, &binding.Resource.ID, &binding.Lifecycle, &binding.Revision)
		if err != nil {
			return assessment.Record{}, err
		}
		if binding.ConnectionID != in.ConnectionID || binding.Resource != in.Resource || binding.Revision != in.ExpectedBindingRevision || binding.Lifecycle == Retired {
			return assessment.Record{}, ErrConflict
		}
	}
	out, err := assessment.SaveBoundSourceTx(ctx, tx, a, in.ServiceID, in.SourceID, in.ExpectedRevision, v)
	if err != nil {
		return out, err
	}
	b, _ = json.Marshal(out)
	_, err = tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,'source.configure',$3,$4,$5)`, a.ID, in.ServiceID, in.IdempotencyKey, digest, b)
	if err != nil {
		return out, err
	}
	return out, nil
}

func sourceMatchesResource(source assessment.EvidenceSource, resource DiscoveredResource) bool {
	if source.Locator != resource.Locator {
		return false
	}
	switch resource.Resource.Provider {
	case GitHub:
		return resource.Resource.Kind == "repository" && source.Type == "repository" && source.Locator == "https://github.com/"+source.Repository
	case AWS:
		return source.AccountID == resource.Resource.Tenant && source.Region == "us-east-1" && ((resource.Resource.Kind == "account_region" && source.Type == "aws") || (resource.Resource.Kind == "eks_cluster" && source.Type == "kubernetes" && source.Cluster == resource.Name))
	case Documents:
		return source.Type == resource.Resource.Kind
	case Workspace:
		return source.Type == "google_workspace" && (resource.Resource.Kind == "org_unit" || resource.Resource.Kind == "group")
	case Kubernetes:
		if source.Type != "kubernetes" || source.ClusterID != resource.Resource.Tenant || source.AccountID != "" || source.Region != "" {
			return false
		}
		if resource.Resource.Kind == "cluster" {
			return source.Namespace == "" && source.NamespaceUID == ""
		}
		return resource.Resource.Kind == "namespace" && source.Namespace == resource.Name && source.NamespaceUID == resource.Resource.ID
	}
	return false
}

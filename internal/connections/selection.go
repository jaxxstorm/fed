package connections

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

type SelectionInput struct {
	ServiceID             string                     `json:"service_id"`
	ExpectedScopeRevision int                        `json:"expected_scope_revision"`
	IdempotencyKey        string                     `json:"idempotency_key"`
	AddToScope            bool                       `json:"add_to_scope"`
	Sources               []SourceConfigurationInput `json:"sources"`
}

type SelectionReceipt struct {
	ScopeRevision  int                 `json:"scope_revision"`
	ScopeAdditions []ResourceKey       `json:"scope_additions"`
	Sources        []assessment.Record `json:"sources"`
}

func (s *Service) SelectSources(ctx context.Context, a auth.Actor, in SelectionInput) (SelectionReceipt, error) {
	if in.ExpectedScopeRevision < 0 || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 256 || len(in.Sources) == 0 || len(in.Sources) > 100 {
		return SelectionReceipt{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return SelectionReceipt{}, err
	}
	defer tx.Rollback(ctx)
	for _, action := range []string{"connect", "edit"} {
		if err = auth.Authorize(ctx, tx, a, in.ServiceID, action); err != nil {
			return SelectionReceipt{}, err
		}
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, in.ServiceID); err != nil {
		return SelectionReceipt{}, err
	}
	request, _ := json.Marshal(in)
	if len(request) > MaxResponseBytes {
		return SelectionReceipt{}, ErrInvalid
	}
	digest := storage.Hash(request)
	var priorHash string
	var prior []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM idempotency WHERE actor_id=$1 AND service_id=$2 AND operation='source.select' AND key=$3`, a.ID, in.ServiceID, in.IdempotencyKey).Scan(&priorHash, &prior)
	if err == nil {
		if priorHash != digest {
			return SelectionReceipt{}, ErrConflict
		}
		var out SelectionReceipt
		if err = json.Unmarshal(prior, &out); err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SelectionReceipt{}, err
	}
	var current assessment.Scope
	var raw []byte
	var revision int
	err = tx.QueryRow(ctx, `SELECT revision,data FROM records WHERE service_id=$1 AND kind='scope' AND id='scope' ORDER BY revision DESC LIMIT 1`, in.ServiceID).Scan(&revision, &raw)
	if errors.Is(err, pgx.ErrNoRows) && in.ExpectedScopeRevision == 0 && in.AddToScope {
		current = assessment.Scope{Description: "Explicitly selected provider resources"}
	} else {
		if err != nil {
			return SelectionReceipt{}, err
		}
		if err = json.Unmarshal(raw, &current); err != nil {
			return SelectionReceipt{}, err
		}
	}
	if revision != in.ExpectedScopeRevision {
		return SelectionReceipt{}, ErrConflict
	}
	out := SelectionReceipt{ScopeRevision: revision, ScopeAdditions: []ResourceKey{}, Sources: []assessment.Record{}}
	seen := map[string]bool{}
	for _, item := range in.Sources {
		if item.ServiceID != in.ServiceID || item.Source.ScopeRevision != revision || seen[item.SourceID] {
			return out, ErrInvalid
		}
		seen[item.SourceID] = true
		if _, err = lockCredentialConnection(ctx, tx, a, in.ServiceID, item.ConnectionID, "connect", item.Generation); err != nil {
			return out, err
		}
		d, err := readConnection(ctx, tx, in.ServiceID, item.ConnectionID)
		if err != nil {
			return out, err
		}
		if d.Connection.Provider != item.Resource.Provider || d.Connection.Tenant() != item.Resource.Tenant {
			return out, ErrInvalid
		}
		if in.AddToScope {
			before, _ := json.Marshal(current)
			if err = addSelectedScope(&current, d.Connection, item); err != nil {
				return out, err
			}
			after, _ := json.Marshal(current)
			if string(before) != string(after) {
				out.ScopeAdditions = append(out.ScopeAdditions, item.Resource)
			}
		}
	}
	if len(out.ScopeAdditions) > 0 || (revision == 0 && in.AddToScope) {
		record, err := assessment.SaveScopeTx(ctx, tx, a, in.ServiceID, revision, current)
		if err != nil {
			return out, err
		}
		out.ScopeRevision = record.Revision
	}
	for _, item := range in.Sources {
		item.Source.ScopeRevision = out.ScopeRevision
		item.IdempotencyKey = "selection-" + storage.Hash([]byte(in.IdempotencyKey+"/"+item.SourceID))
		record, err := s.configureSourceTx(ctx, tx, a, item)
		if err != nil {
			return SelectionReceipt{}, err
		}
		out.Sources = append(out.Sources, record)
	}
	b, _ := json.Marshal(out)
	_, err = tx.Exec(ctx, `INSERT INTO idempotency(actor_id,service_id,operation,key,request_hash,response) VALUES($1,$2,'source.select',$3,$4,$5)`, a.ID, in.ServiceID, in.IdempotencyKey, digest, b)
	if err != nil {
		return SelectionReceipt{}, err
	}
	return out, tx.Commit(ctx)
}

func addSelectedScope(scope *assessment.Scope, c Connection, item SourceConfigurationInput) error {
	v := item.Source
	switch item.Resource.Provider {
	case GitHub:
		for i := range scope.Repositories {
			r := &scope.Repositories[i]
			if r.Name != v.Repository {
				continue
			}
			if r.Commit != v.Commit {
				return ErrConflict
			}
			for _, path := range v.Paths {
				if !slices.Contains(r.Paths, path) {
					r.Paths = append(r.Paths, path)
				}
			}
			return nil
		}
		scope.Repositories = append(scope.Repositories, assessment.Repository{Name: v.Repository, Commit: v.Commit, Paths: slices.Clone(v.Paths)})
	case AWS:
		if v.AccountID != c.Configuration.AccountID || v.Region != "us-east-1" {
			return ErrInvalid
		}
		index := -1
		for i := range scope.Accounts {
			if scope.Accounts[i].ID == v.AccountID {
				index = i
				break
			}
		}
		if index < 0 {
			scope.Accounts = append(scope.Accounts, assessment.Account{ID: v.AccountID})
			index = len(scope.Accounts) - 1
		}
		account := &scope.Accounts[index]
		if !slices.Contains(account.Regions, v.Region) {
			account.Regions = append(account.Regions, v.Region)
		}
		if item.Resource.Kind == "eks_cluster" && !slices.Contains(account.Clusters, v.Cluster) {
			account.Clusters = append(account.Clusters, v.Cluster)
		}
	case Workspace:
		index := -1
		for i := range scope.Workspace {
			if scope.Workspace[i].CustomerID == item.Resource.Tenant {
				index = i
				break
			}
		}
		if index < 0 {
			scope.Workspace = append(scope.Workspace, assessment.WorkspaceScope{CustomerID: item.Resource.Tenant, OrgUnitIDs: []string{}, GroupIDs: []string{}})
			index = len(scope.Workspace) - 1
		}
		entry := &scope.Workspace[index]
		switch item.Resource.Kind {
		case "org_unit":
			if !slices.Contains(entry.OrgUnitIDs, item.Resource.ID) {
				entry.OrgUnitIDs = append(entry.OrgUnitIDs, item.Resource.ID)
			}
		case "group":
			if !slices.Contains(entry.GroupIDs, item.Resource.ID) {
				entry.GroupIDs = append(entry.GroupIDs, item.Resource.ID)
			}
		default:
			return ErrInvalid
		}
	case Kubernetes:
		if v.ClusterID != c.Configuration.ClusterID || v.ClusterID != item.Resource.Tenant {
			return ErrInvalid
		}
		index := -1
		for i := range scope.Kubernetes {
			if scope.Kubernetes[i].ClusterID == v.ClusterID {
				index = i
				break
			}
		}
		if index < 0 {
			scope.Kubernetes = append(scope.Kubernetes, assessment.KubernetesScope{ClusterID: v.ClusterID, Server: c.Configuration.Server, Namespaces: []assessment.NamespaceScope{}})
			index = len(scope.Kubernetes) - 1
		}
		entry := &scope.Kubernetes[index]
		if entry.Server != c.Configuration.Server {
			return ErrConflict
		}
		if item.Resource.Kind == "cluster" {
			entry.ClusterWide = true
			return nil
		}
		if item.Resource.Kind != "namespace" {
			return ErrInvalid
		}
		for _, namespace := range entry.Namespaces {
			if namespace.Name == v.Namespace {
				if namespace.UID != v.NamespaceUID {
					return ErrConflict
				}
				return nil
			}
		}
		entry.Namespaces = append(entry.Namespaces, assessment.NamespaceScope{Name: v.Namespace, UID: v.NamespaceUID})
	case Documents:
	default:
		return ErrInvalid
	}
	return nil
}

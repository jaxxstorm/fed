package assessment

import (
	"context"
	"errors"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

func sourceInScope(v EvidenceSource, scope Scope) bool {
	switch v.Type {
	case "document", "operational_records":
		u, err := url.Parse(v.Locator)
		return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
	case "repository":
		for _, r := range scope.Repositories {
			if r.Name != v.Repository || r.Commit != v.Commit {
				continue
			}
			for _, p := range v.Paths {
				if !repositoryPathAllowed(r.Paths, p) {
					return false
				}
			}
			return len(v.Paths) > 0 && v.Locator == "https://github.com/"+r.Name
		}
	case "aws", "kubernetes":
		if v.Type == "kubernetes" && v.ClusterID != "" {
			for _, cluster := range scope.Kubernetes {
				if cluster.ClusterID != v.ClusterID || v.AccountID != "" || v.Region != "" || v.ProviderTenant != v.ClusterID {
					continue
				}
				if v.ResourceKind == "cluster" {
					return cluster.ClusterWide && v.ResourceID == v.ClusterID && v.Namespace == "" && v.NamespaceUID == "" && v.Locator == "kubernetes://cluster/"+url.PathEscape(v.ClusterID)+"/cluster/"+url.PathEscape(v.ClusterID)
				}
				if v.ResourceKind != "namespace" || v.NamespaceUID == "" || v.ResourceID != v.NamespaceUID || v.Locator != "kubernetes://cluster/"+url.PathEscape(v.ClusterID)+"/namespace/"+url.PathEscape(v.NamespaceUID) {
					return false
				}
				for _, namespace := range cluster.Namespaces {
					if namespace.Name == v.Namespace && namespace.UID == v.NamespaceUID {
						return true
					}
				}
				return cluster.ClusterWide
			}
			return false
		}
		for _, a := range scope.Accounts {
			if a.ID == v.AccountID && slices.Contains(a.Regions, v.Region) {
				return v.Locator == a.ID+"/"+v.Region && (v.Type == "aws" || (v.Cluster != "" && slices.Contains(a.Clusters, v.Cluster)))
			}
		}
	case "google_workspace":
		for _, customer := range scope.Workspace {
			if customer.CustomerID != v.ProviderTenant {
				continue
			}
			if v.Locator != "google-workspace://"+url.PathEscape(customer.CustomerID)+"/"+v.ResourceKind+"/"+url.PathEscape(v.ResourceID) {
				return false
			}
			return v.ResourceKind == "org_unit" && slices.Contains(customer.OrgUnitIDs, v.ResourceID) || v.ResourceKind == "group" && slices.Contains(customer.GroupIDs, v.ResourceID)
		}
	}
	return false
}

func (s *Store) SaveEvidenceSource(ctx context.Context, a auth.Actor, service, id string, expected int, v EvidenceSource) (Record, error) {
	if err := validateSourceFields(v); err != nil {
		return Record{}, err
	}
	return s.mutate(ctx, a, record(service, "evidence_source", id, "", v), expected, "edit", func(tx pgx.Tx) error {
		return validateSourceTx(ctx, tx, service, id, v)
	})
}

func validateSourceFields(v EvidenceSource) error {
	if strings.TrimSpace(v.Name) == "" || len(v.Name) > 256 || len(v.AccessHint) > 256 || v.FreshnessHours < 1 || v.FreshnessHours > 24*366 || len(v.Locator) > 2048 || len(v.Paths) > 100 || len(v.RequirementIDs) > 100 {
		return ErrInvalid
	}
	// Hints are symbolic client-side names, not free-form connection strings.
	if strings.ContainsAny(v.AccessHint, "\r\n=:@?&") {
		return ErrInvalid
	}
	return nil
}

func validateSourceTx(ctx context.Context, tx pgx.Tx, service, id string, v EvidenceSource) error {
	// Binding lifecycle and access paths are controlled by connect operations,
	// never by edit-only legacy source forms or agent submissions.
	var connection, lifecycle, kind, resource, tenant string
	err := tx.QueryRow(ctx, `SELECT connection_id,lifecycle,resource_kind,resource_id,tenant FROM connection_source_bindings WHERE service_id=$1 AND source_id=$2`, service, id).Scan(&connection, &lifecycle, &kind, &resource, &tenant)
	if err == nil {
		previous, e := latest(ctx, tx, service, "evidence_source", id)
		if e != nil {
			return e
		}
		old, e := Decode[EvidenceSource](previous)
		if e != nil {
			return e
		}
		// Imported bindings deliberately leave historical source payloads
		// untouched. Preserve their absent metadata in legacy edit requests.
		metadataMatches := v.ConnectionID == old.ConnectionID && v.ResourceKind == old.ResourceKind && v.ResourceID == old.ResourceID && v.ProviderTenant == old.ProviderTenant && v.Lifecycle == old.Lifecycle
		metadataMatches = metadataMatches && v.Type == old.Type && v.Locator == old.Locator && v.Repository == old.Repository && v.AccountID == old.AccountID && v.Region == old.Region && v.Cluster == old.Cluster && v.ClusterID == old.ClusterID && v.Namespace == old.Namespace && v.NamespaceUID == old.NamespaceUID
		if lifecycle == "retired" || !metadataMatches || v.Enabled != (lifecycle == "enabled") {
			return ErrConflict
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	} else if v.ConnectionID != "" || v.ResourceID != "" || v.ResourceKind != "" || v.ProviderTenant != "" || v.Lifecycle != "" {
		return ErrInvalid
	}
	return validateSourceScopeTx(ctx, tx, service, v)
}

func validateSourceScopeTx(ctx context.Context, tx pgx.Tx, service string, v EvidenceSource) error {
	r, err := latest(ctx, tx, service, "scope", "scope")
	if err != nil {
		return err
	}
	if r.Revision != v.ScopeRevision {
		return ErrConflict
	}
	scope, err := Decode[Scope](r)
	if err != nil {
		return err
	}
	if !sourceInScope(v, scope) {
		return ErrInvalid
	}
	for _, rid := range v.RequirementIDs {
		if err := requireRule(ctx, tx, service, rid); err != nil {
			return err
		}
	}
	return nil
}

// SaveBoundSourceTx participates in the connection selection transaction. The
// binding must already exist and must exactly match the supplied metadata.
func SaveBoundSourceTx(ctx context.Context, tx pgx.Tx, a auth.Actor, service, id string, expected int, v EvidenceSource) (Record, error) {
	if service == "" || id == "" || expected < 0 {
		return Record{}, ErrInvalid
	}
	if err := auth.Authorize(ctx, tx, a, service, "connect"); err != nil {
		return Record{}, err
	}
	if err := auth.Authorize(ctx, tx, a, service, "edit"); err != nil {
		return Record{}, err
	}
	if err := validateSourceFields(v); err != nil {
		return Record{}, err
	}
	previous, err := latest(ctx, tx, service, "evidence_source", id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Record{}, err
	}
	if previous.Revision != expected {
		return Record{}, ErrConflict
	}
	if expected == 0 {
		var connection, lifecycle, kind, resource, tenant string
		if err = tx.QueryRow(ctx, `SELECT connection_id,lifecycle,resource_kind,resource_id,tenant FROM connection_source_bindings WHERE service_id=$1 AND source_id=$2`, service, id).Scan(&connection, &lifecycle, &kind, &resource, &tenant); err != nil {
			return Record{}, err
		}
		if connection != v.ConnectionID || lifecycle == "retired" || lifecycle != v.Lifecycle || kind != v.ResourceKind || resource != v.ResourceID || tenant != v.ProviderTenant || v.Enabled != (lifecycle == "enabled") {
			return Record{}, ErrInvalid
		}
		// Source field/scope checks are shared with legacy edits; only the new
		// binding check differs because no previous source revision exists.
		if err = validateSourceScopeTx(ctx, tx, service, v); err != nil {
			return Record{}, err
		}
	} else if err = validateSourceTx(ctx, tx, service, id, v); err != nil {
		return Record{}, err
	}
	return insertRecord(ctx, tx, a, record(service, "evidence_source", id, "", v), expected+1)
}

// ReconcileEvidenceSources materializes scope-derived sources even without a worker.
// Existing records are never replaced; scope membership is checked at use time.
func (s *Store) ReconcileEvidenceSources(ctx context.Context, a auth.Actor, service string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "read"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, service); err != nil {
		return err
	}
	r, err := latest(ctx, tx, service, "scope", "scope")
	if err != nil {
		return err
	}
	scope, err := Decode[Scope](r)
	if err != nil {
		return err
	}
	var candidates []EvidenceSource
	for _, repo := range scope.Repositories {
		if len(repo.Paths) == 0 {
			continue
		}
		candidates = append(candidates, EvidenceSource{Type: "repository", Name: repo.Name, Locator: "https://github.com/" + repo.Name, Repository: repo.Name, Commit: repo.Commit, Paths: repo.Paths})
	}
	for _, account := range scope.Accounts {
		for _, region := range account.Regions {
			v := EvidenceSource{Type: "aws", Name: account.ID + "/" + region, Locator: account.ID + "/" + region, AccountID: account.ID, Region: region}
			candidates = append(candidates, v)
			for _, cluster := range account.Clusters {
				k := v
				k.Type = "kubernetes"
				k.Name = cluster
				k.Cluster = cluster
				candidates = append(candidates, k)
			}
		}
	}
	for _, v := range candidates {
		id := "scope-" + storage.Hash([]byte(v.Type + "/" + v.Locator + "/" + v.Cluster))[:24]
		// Explicitly selected sources may have non-scope IDs. Resolve their
		// existing binding before generating a legacy ID, including tombstones.
		rows, e := tx.Query(ctx, `SELECT b.source_id FROM connection_source_bindings b JOIN LATERAL (SELECT data FROM records r WHERE r.service_id=b.service_id AND r.kind='evidence_source' AND r.id=b.source_id ORDER BY revision DESC LIMIT 1) r ON true WHERE b.service_id=$1 AND r.data->>'type'=$2 AND r.data->>'locator'=$3 AND COALESCE(r.data->>'cluster','')=$4 ORDER BY b.source_id`, service, v.Type, v.Locator, v.Cluster)
		if e != nil {
			return e
		}
		boundIDs, e := pgx.CollectRows(rows, pgx.RowTo[string])
		if e != nil {
			return e
		}
		if len(boundIDs) > 1 {
			continue
		}
		if len(boundIDs) == 1 {
			id = boundIDs[0]
		}
		var retired bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_source_bindings WHERE service_id=$1 AND source_id=$2 AND lifecycle='retired')`, service, id).Scan(&retired); err != nil {
			return err
		}
		if retired {
			continue
		}
		previous, e := latest(ctx, tx, service, "evidence_source", id)
		if e != nil && !errors.Is(e, ErrNotFound) {
			return e
		}
		revision := 1
		if e == nil {
			old, e := Decode[EvidenceSource](previous)
			if e != nil {
				return e
			}
			if old.ScopeRevision == r.Revision {
				continue
			}
			v.Name = old.Name
			v.AccessHint = old.AccessHint
			v.Enabled = old.Enabled
			v.FreshnessHours = old.FreshnessHours
			v.RequirementIDs = old.RequirementIDs
			v.ConnectionID = old.ConnectionID
			v.ResourceKind = old.ResourceKind
			v.ResourceID = old.ResourceID
			v.ProviderTenant = old.ProviderTenant
			v.Lifecycle = old.Lifecycle
			if v.Type == "repository" {
				allowed := []string{}
				for _, p := range old.Paths {
					if slices.Contains(v.Paths, p) {
						allowed = append(allowed, p)
					}
				}
				v.Paths = allowed
			}
			revision = previous.Revision + 1
		} else {
			v.Enabled = true
			v.FreshnessHours = 48
		}
		v.ScopeRevision = r.Revision
		if _, err = insertRecord(ctx, tx, a, record(service, "evidence_source", id, "", v), revision); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func locatorWithin(source EvidenceSource, locator string) bool {
	switch source.Type {
	case "document", "operational_records":
		base, e := url.Parse(source.Locator)
		if e != nil {
			return false
		}
		u, e := url.Parse(locator)
		if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Scheme != base.Scheme || u.Host != base.Host {
			return false
		}
		bp := strings.TrimSuffix(path.Clean(base.Path), "/")
		p := path.Clean(u.Path)
		return bp == "." || bp == "" || bp == "/" || p == bp || strings.HasPrefix(p, bp+"/")
	case "repository":
		return repositoryPathAllowed(source.Paths, locator)
	case "aws", "kubernetes":
		return locator == source.Locator || strings.HasPrefix(locator, source.Locator+"/")
	case "google_workspace":
		return locator == source.Locator
	}
	return false
}

func repositoryPathAllowed(roots []string, p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") {
		return false
	}
	for _, root := range roots {
		if root == "." || p == root || strings.HasPrefix(p, strings.TrimSuffix(root, "/")+"/") {
			return true
		}
	}
	return false
}

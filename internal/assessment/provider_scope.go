package assessment

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
)

func validateProviderScope(scope Scope) error {
	if len(scope.Workspace) > 100 || len(scope.Kubernetes) > 1000 {
		return ErrInvalid
	}
	customers := map[string]bool{}
	for _, entry := range scope.Workspace {
		if entry.CustomerID == "" || len(entry.CustomerID) > 256 || customers[entry.CustomerID] || len(entry.OrgUnitIDs)+len(entry.GroupIDs) > 10000 {
			return ErrInvalid
		}
		customers[entry.CustomerID] = true
		for _, ids := range [][]string{entry.OrgUnitIDs, entry.GroupIDs} {
			seen := map[string]bool{}
			for _, id := range ids {
				if id == "" || len(id) > 512 || seen[id] {
					return ErrInvalid
				}
				seen[id] = true
			}
		}
	}
	clusters := map[string]bool{}
	for _, entry := range scope.Kubernetes {
		endpoint, err := url.Parse(entry.Server)
		if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
			return ErrInvalid
		}
		if strings.TrimSpace(entry.ClusterID) == "" || len(entry.ClusterID) > 256 || clusters[entry.ClusterID] || len(entry.Namespaces) > 10000 || (!entry.ClusterWide && len(entry.Namespaces) == 0) {
			return ErrInvalid
		}
		clusters[entry.ClusterID] = true
		seen := map[string]bool{}
		for _, namespace := range entry.Namespaces {
			if !regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`).MatchString(namespace.Name) || seen[namespace.Name] {
				return ErrInvalid
			}
			if _, err := uuid.Parse(namespace.UID); err != nil {
				return ErrInvalid
			}
			seen[namespace.Name] = true
		}
	}
	return nil
}

// SaveScopeTx lets explicit connection selection commit scope and sources
// together under the existing service lock.
func SaveScopeTx(ctx context.Context, tx pgx.Tx, a auth.Actor, service string, expected int, v Scope) (Record, error) {
	if err := auth.Authorize(ctx, tx, a, service, "edit"); err != nil {
		return Record{}, err
	}
	if err := validateScope(v); err != nil {
		return Record{}, err
	}
	current, err := latest(ctx, tx, service, "scope", "scope")
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Record{}, err
	}
	if current.Revision != expected {
		return Record{}, ErrConflict
	}
	return insertRecord(ctx, tx, a, record(service, "scope", "scope", "", v), expected+1)
}

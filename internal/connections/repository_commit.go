package connections

import (
	"context"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
)

type ResolveCommitInput struct {
	ServiceID    string `json:"service_id"`
	ConnectionID string `json:"connection_id"`
	Generation   int64  `json:"generation"`
	RepositoryID string `json:"repository_id"`
	Ref          string `json:"ref"`
}
type ResolvedCommit struct {
	Commit       string `json:"commit"`
	RepositoryID string `json:"repository_id"`
	Generation   int64  `json:"generation"`
}

func (s *Service) ResolveRepositoryCommit(ctx context.Context, a auth.Actor, in ResolveCommitInput) (ResolvedCommit, error) {
	if err := auth.Authorize(ctx, s.DB, a, in.ServiceID, "connect"); err != nil {
		return ResolvedCommit{}, err
	}
	d, err := readConnection(ctx, s.DB, in.ServiceID, in.ConnectionID)
	if err != nil {
		return ResolvedCommit{}, err
	}
	if err = d.Connection.CheckGeneration(in.Generation); err != nil {
		return ResolvedCommit{}, err
	}
	if d.Connection.Provider != GitHub || d.Connection.Mode != ServerManaged {
		return ResolvedCommit{}, ErrInvalid
	}
	var exists bool
	err = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connection_inventory WHERE service_id=$1 AND connection_id=$2 AND tenant=$3 AND resource_kind='repository' AND resource_id=$4 AND available)`, in.ServiceID, in.ConnectionID, d.Connection.Tenant(), in.RepositoryID).Scan(&exists)
	if err != nil {
		return ResolvedCommit{}, err
	}
	if !exists {
		return ResolvedCommit{}, ErrInvalid
	}
	k, err := s.Credentials.Get(ctx, a, in.ServiceID, in.ConnectionID, "connect", in.Generation)
	if err != nil {
		return ResolvedCommit{}, err
	}
	adapter, err := s.Registry.Adapter(GitHub)
	if err != nil {
		return ResolvedCommit{}, err
	}
	p, ok := adapter.(interface {
		ResolveCommit(context.Context, Connection, Credentials, string, string) (string, error)
	})
	if !ok {
		return ResolvedCommit{}, ErrInvalid
	}
	call, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	commit, err := p.ResolveCommit(call, d.Connection, k, in.RepositoryID, in.Ref)
	if err != nil {
		return ResolvedCommit{}, err
	}
	if err = auth.Authorize(ctx, s.DB, a, in.ServiceID, "connect"); err != nil {
		return ResolvedCommit{}, err
	}
	latest, err := readConnection(ctx, s.DB, in.ServiceID, in.ConnectionID)
	if err != nil {
		return ResolvedCommit{}, err
	}
	if err = latest.Connection.CheckGeneration(in.Generation); err != nil {
		return ResolvedCommit{}, err
	}
	return ResolvedCommit{Commit: commit, RepositoryID: in.RepositoryID, Generation: in.Generation}, nil
}

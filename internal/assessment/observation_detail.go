package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/observation"
)

type ObservationDetail struct {
	Retired              bool                   `json:"retired"`
	LifecycleExplanation string                 `json:"lifecycle_explanation"`
	ID                   string                 `json:"id"`
	SourceKind           string                 `json:"source_kind"`
	ArtifactHash         string                 `json:"artifact_hash"`
	ReceivedAt           time.Time              `json:"received_at"`
	Observation          observation.Submission `json:"observation"`
}

func (s *Store) ObservationDetail(ctx context.Context, a auth.Actor, service, id string) (ObservationDetail, error) {
	out := ObservationDetail{ID: id}
	if err := auth.Authorize(ctx, s.query(), a, service, "read"); err != nil {
		return out, err
	}
	var raw []byte
	err := s.query().QueryRow(ctx, `SELECT source_kind,artifact_hash,received_at,data FROM observations WHERE service_id=$1 AND id=$2`, service, id).Scan(&out.SourceKind, &out.ArtifactHash, &out.ReceivedAt, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(raw, &out.Observation)
	if err == nil {
		err = s.query().QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM current_resource_observations WHERE service_id=$1 AND id=$2)`, service, id).Scan(&out.Retired)
		if out.Retired {
			out.LifecycleExplanation = "A newer complete server inventory with matching scope, profile, source and credential generation confirmed this stable resource identity absent. Historical evidence is preserved."
		} else {
			out.LifecycleExplanation = "Retained evidence; absence has not been established by a newer complete compatible inventory."
		}
	}
	return out, err
}

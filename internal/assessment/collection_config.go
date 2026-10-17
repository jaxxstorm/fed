package assessment

import (
	"context"
	"encoding/json"
	"net/netip"
	"regexp"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
)

type CollectionConfiguration struct {
	Revision       int                        `json:"revision"`
	Paused         bool                       `json:"paused"`
	CadenceHours   int                        `json:"cadence_hours"`
	FreshnessHours int                        `json:"freshness_hours"`
	Overrides      map[string]json.RawMessage `json:"overrides"`
	NextRunAt      time.Time                  `json:"next_run_at"`
}

func (s *Store) CollectionConfiguration(ctx context.Context, a auth.Actor, service string) (CollectionConfiguration, error) {
	if err := auth.Authorize(ctx, s.DB, a, service, "read"); err != nil {
		return CollectionConfiguration{}, err
	}
	var c CollectionConfiguration
	var raw []byte
	err := s.DB.QueryRow(ctx, `SELECT revision,paused,cadence_hours,freshness_hours,overrides,next_run_at FROM collection_schedules WHERE service_id=$1`, service).Scan(&c.Revision, &c.Paused, &c.CadenceHours, &c.FreshnessHours, &raw, &c.NextRunAt)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(raw, &c.Overrides)
	return c, err
}
func (s *Store) ConfigureCollection(ctx context.Context, a auth.Actor, service string, expected int, c CollectionConfiguration) (CollectionConfiguration, error) {
	if c.CadenceHours < 1 || c.CadenceHours > 8760 || c.FreshnessHours < 1 || c.FreshnessHours > 8760 || expected < 1 {
		return c, ErrInvalid
	}
	// Policy overrides are accepted only through known, bounded fields.
	for key, raw := range c.Overrides {
		switch key {
		case "minimum_retention_days":
			var days int
			if json.Unmarshal(raw, &days) != nil || days < 1 || days > 35 {
				return c, ErrInvalid
			}
		case "approved_cidrs", "namespace_exclusions":
			var values []string
			if json.Unmarshal(raw, &values) != nil || len(values) > 100 {
				return c, ErrInvalid
			}
			for _, value := range values {
				if key == "approved_cidrs" {
					if _, err := netip.ParsePrefix(value); err != nil {
						return c, ErrInvalid
					}
				} else if len(value) > 63 || !regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`).MatchString(value) {
					return c, ErrInvalid
				}
			}
		default:
			return c, ErrInvalid
		}
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return c, err
	}
	defer tx.Rollback(ctx)
	if err = auth.Authorize(ctx, tx, a, service, "edit"); err != nil {
		return c, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM services WHERE id=$1 FOR UPDATE`, service); err != nil {
		return c, err
	}
	var rev int
	if err = tx.QueryRow(ctx, `SELECT revision FROM collection_schedules WHERE service_id=$1 FOR UPDATE`, service).Scan(&rev); err != nil {
		return c, err
	}
	if rev != expected {
		return c, ErrConflict
	}
	c.Revision = expected + 1
	if c.Overrides == nil {
		c.Overrides = map[string]json.RawMessage{}
	}
	err = tx.QueryRow(ctx, `UPDATE collection_schedules SET revision=$2,paused=$3,cadence_hours=$4,freshness_hours=$5,overrides=$6,updated_at=now(),next_run_at=CASE WHEN paused AND NOT $3 THEN now() ELSE next_run_at END WHERE service_id=$1 RETURNING next_run_at`, service, c.Revision, c.Paused, c.CadenceHours, c.FreshnessHours, data(c.Overrides)).Scan(&c.NextRunAt)
	if err != nil {
		return c, err
	}
	if err = audit(ctx, tx, a, service, "collection.configured", service, c.Revision); err != nil {
		return c, err
	}
	return c, tx.Commit(ctx)
}

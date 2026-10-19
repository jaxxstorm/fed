package observation

import (
	"slices"
	"time"

	"github.com/jaxxstorm/fed/internal/storage"
)

// CheckRows preserves each mapped check result independently of requirement
// compliance. Incomplete reads cannot become passing or absent-control claims.
func CheckRows(in Submission, link Link, source, id string, at time.Time, hours int, now time.Time) []Finding {
	out := []Finding{}
	for _, e := range in.Evaluations {
		if len(link.Conditions) > 0 && !slices.Contains(link.Conditions, e.Condition) {
			continue
		}
		outcome := e.Outcome
		if in.Coverage.Error != "" || in.Coverage.State == "error" {
			outcome = "error"
		} else if !in.Coverage.Complete || in.Coverage.State != "observed" {
			outcome = "unknown"
		}
		freshness := "current"
		if hours <= 0 || !at.Add(time.Duration(hours)*time.Hour).After(now) {
			freshness = "stale"
		}
		basis := "agent_assertion"
		if source == "collector_observation" {
			basis = "collector_evaluation"
		}
		out = append(out, Finding{ID: storage.Hash([]byte(id + "\x00" + e.Condition)), ObservationID: id, Target: in.Target, ObservedAt: at, Condition: e.Condition, Outcome: outcome, Explanation: e.Explanation, Basis: basis, ArtifactSchema: in.Artifact.Schema, Configuration: in.Artifact.Configuration, Coverage: &in.Coverage, Freshness: freshness})
	}
	return out
}

package assessment

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/jaxxstorm/fed/internal/observation"
)

type ObservationStatus struct {
	Recovery        []observation.Recovery `json:"recovery"`
	Errors          []string               `json:"errors"`
	MissingMeasures int                    `json:"missing_measures"`
	Count           int                    `json:"count"`
	Freshness       string                 `json:"freshness"`
	Technical       string                 `json:"technical"`
	LastObservedAt  *time.Time             `json:"last_observed_at,omitempty"`
	Expected        int                    `json:"expected"`
	Observed        int                    `json:"observed"`
	Findings        []observation.Finding  `json:"findings"`
	Sources         []string               `json:"sources"`
}

func (s *Store) observationStatus(ctx context.Context, service, requirement string, now time.Time) (ObservationStatus, error) {
	if s.presentationCache != nil {
		if v, ok := s.presentationCache.collection[requirement]; ok {
			return v, nil
		}
		return ObservationStatus{Freshness: "missing", Technical: "unknown", Findings: []observation.Finding{}, Sources: []string{}}, nil
	}
	out := ObservationStatus{Freshness: "missing", Technical: "unknown", Findings: []observation.Finding{}, Sources: []string{}}
	rows, err := s.query().Query(ctx, `SELECT DISTINCT ON(o.target_key,l.measure_id) o.data,o.observed_at,o.source_kind,(m.data->>'freshness_hours')::int,l.data
FROM observation_links l JOIN observations o ON o.service_id=l.service_id AND o.id=l.observation_id
JOIN records m ON m.service_id=l.service_id AND m.kind='measure' AND m.id=l.measure_id AND m.revision=l.measure_revision
WHERE l.service_id=$1 AND l.requirement_id=$2
AND o.scope_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope' AND id='scope')
AND o.profile_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='profile' AND id='profile')
AND l.measure_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='measure' AND id=l.measure_id)
AND (coalesce((m.data->>'configuration_revision')::int,0)=0 OR (m.data->>'configuration_revision')::int=(SELECT revision FROM collection_schedules WHERE service_id=$1))
ORDER BY o.target_key,l.measure_id,o.observed_at DESC,o.received_at DESC`, service, requirement)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	rank := 0
	measuresSeen := map[string]bool{}
	sourceSeen := map[string]bool{}
	for rows.Next() {
		var raw []byte
		var linkRaw []byte
		var at time.Time
		var source string
		var hours int
		if err = rows.Scan(&raw, &at, &source, &hours, &linkRaw); err != nil {
			return out, err
		}
		var in observation.Submission
		var link observation.Link
		if err = json.Unmarshal(linkRaw, &link); err != nil {
			return out, err
		}
		measuresSeen[link.Measure.ID] = true
		if err = json.Unmarshal(raw, &in); err != nil {
			return out, err
		}
		out.Count++
		out.Expected += in.Coverage.Expected
		out.Observed += in.Coverage.Observed
		if out.LastObservedAt == nil || at.After(*out.LastObservedAt) {
			copy := at
			out.LastObservedAt = &copy
		}
		if out.Freshness == "missing" {
			out.Freshness = "current"
		}
		if at.Add(time.Duration(hours) * time.Hour).Before(now) {
			out.Freshness = "stale"
		}
		if !sourceSeen[source] {
			out.Sources = append(out.Sources, source)
			sourceSeen[source] = true
		}
		outcome := "pass"
		if !in.Coverage.Complete || in.Coverage.State != "observed" || len(in.Evaluations) == 0 {
			outcome = "unknown"
		}
		matched := 0
		for _, evaluation := range in.Evaluations {
			if len(link.Conditions) > 0 && !slices.Contains(link.Conditions, evaluation.Condition) {
				continue
			}
			matched++
			if evaluation.Outcome == "fail" {
				outcome = "fail"
				basis := "agent_assertion"
				if source == "collector_observation" {
					basis = "collector_evaluation"
				}
				out.Findings = append(out.Findings, observation.Finding{Condition: evaluation.Condition, Outcome: "fail", Explanation: evaluation.Explanation, Basis: basis})
			} else if outcome == "pass" && evaluation.Outcome != "pass" {
				outcome = "unknown"
			}
		}
		if matched == 0 {
			outcome = "unknown"
		}
		if in.Coverage.Error != "" {
			out.Recovery = append(out.Recovery, observation.ExplainError(in.Coverage.Error, in.Target.AccountID+"/"+in.Target.Region+"/"+in.Target.ResourceID, source))
			outcome = "error"
			out.Errors = append(out.Errors, in.Target.AccountID+"/"+in.Target.Region+"/"+in.Target.ResourceID+": "+in.Coverage.Error)
		}
		value := map[string]int{"pass": 1, "unknown": 2, "fail": 3, "error": 4}[outcome]
		if value > rank {
			rank = value
			out.Technical = outcome
		}
	}
	if out.Freshness == "stale" && out.Technical == "pass" {
		out.Technical = "unknown"
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	findingRows, err := s.query().Query(ctx, `SELECT f.data FROM (SELECT DISTINCT ON(f.finding_id) f.finding_id,f.outcome,f.data
FROM finding_events f JOIN observations o ON o.service_id=f.service_id AND o.id=f.observation_id
WHERE f.service_id=$1 AND o.scope_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope' AND id='scope')
AND EXISTS(SELECT 1 FROM observation_links l WHERE l.service_id=f.service_id AND l.observation_id=f.observation_id AND l.requirement_id=$2 AND (coalesce(jsonb_array_length(l.data->'conditions'),0)=0 OR (l.data->'conditions') ? f.condition))
ORDER BY f.finding_id,o.observed_at DESC,o.received_at DESC) f WHERE f.outcome='fail'`, service, requirement)
	if err != nil {
		return out, err
	}
	defer findingRows.Close()
	out.Findings = []observation.Finding{}
	for findingRows.Next() {
		var b []byte
		if err = findingRows.Scan(&b); err != nil {
			return out, err
		}
		var f observation.Finding
		if err = json.Unmarshal(b, &f); err != nil {
			return out, err
		}
		out.Findings = append(out.Findings, f)
	}
	if len(out.Findings) > 0 && out.Technical != "error" {
		out.Technical = "fail"
	}
	err = findingRows.Err()
	findingRows.Close()
	if err != nil {
		return out, err
	}
	var expectedMeasures int
	if err = s.query().QueryRow(ctx, `SELECT count(*) FROM (SELECT DISTINCT ON(id) data FROM records WHERE service_id=$1 AND kind='measure' AND requirement_id=$2 ORDER BY id,revision DESC) m WHERE (data->>'scope_revision')::int=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope' AND id='scope')`, service, requirement).Scan(&expectedMeasures); err != nil {
		return out, err
	}
	out.MissingMeasures = max(0, expectedMeasures-len(measuresSeen))
	if out.MissingMeasures > 0 && out.Technical == "pass" {
		out.Technical = "unknown"
	}
	return out, nil
}

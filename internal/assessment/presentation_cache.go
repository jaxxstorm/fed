package assessment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jaxxstorm/fed/internal/observation"
	"slices"
	"time"
)

// The cache is request-local and only installed after authorization inside a
// repeatable-read transaction. It supplies the existing Status projection with
// batched inputs, rather than introducing a second definition of readiness.
type presentationCache struct {
	service, actor               string
	latest, refs, reviews        map[string]Record
	technical, automation        map[string]string
	failures                     map[string]*time.Time
	findingTimes, contradictions map[string]time.Time
	measureHours                 map[string]*int
	collection                   map[string]ObservationStatus
}

func (s *Store) presentationReference(ctx context.Context, service, kind string, ref Ref) (Record, error) {
	if c := s.presentationCache; c != nil && c.service == service {
		if r, ok := c.refs[fmt.Sprintf("%s/%s/%d", kind, ref.ID, ref.Revision)]; ok {
			return r, nil
		}
		return Record{}, ErrInvalid
	}
	return reference(ctx, s.query(), service, kind, ref)
}
func rank(outcome string) int {
	return map[string]int{"pass": 1, "unknown": 2, "fail": 3, "error": 4}[outcome]
}
func (s *Store) loadPresentationCache(ctx context.Context, service, actor string, now time.Time) (*presentationCache, error) {
	c := &presentationCache{service: service, actor: actor, latest: map[string]Record{}, refs: map[string]Record{}, reviews: map[string]Record{}, technical: map[string]string{}, automation: map[string]string{}, failures: map[string]*time.Time{}, findingTimes: map[string]time.Time{}, contradictions: map[string]time.Time{}, measureHours: map[string]*int{}, collection: map[string]ObservationStatus{}}
	q := s.query()
	rows, err := q.Query(ctx, `SELECT service_id,kind,id,revision,requirement_id,actor_id,data,created_at FROM records WHERE service_id=$1 AND kind IN ('scope','profile','measure','evidence','assessment','review','rule_result','evidence_source','gap') ORDER BY kind,id,revision`, service)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r Record
		if err = rows.Scan(&r.ServiceID, &r.Kind, &r.ID, &r.Revision, &r.RequirementID, &r.ActorID, &r.Data, &r.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		c.latest[r.Kind+"/"+r.ID] = r
		c.refs[fmt.Sprintf("%s/%s/%d", r.Kind, r.ID, r.Revision)] = r
		if r.Kind == "review" && r.CreatedAt.After(c.reviews[r.RequirementID].CreatedAt) {
			c.reviews[r.RequirementID] = r
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	scopeRevision := c.latest["scope/scope"].Revision
	for _, r := range c.latest {
		if r.Kind == "measure" {
			m, e := Decode[Measure](r)
			if e != nil {
				return nil, e
			}
			if previous := c.measureHours[r.RequirementID]; previous == nil || m.FreshnessHours < *previous {
				hours := m.FreshnessHours
				c.measureHours[r.RequirementID] = &hours
			}
			if m.ScopeRevision == scopeRevision {
				v := c.collection[r.RequirementID]
				v.MissingMeasures++
				c.collection[r.RequirementID] = v
			}
		}
	}
	rows, err = q.Query(ctx, `SELECT DISTINCT ON(requirement_id,check_id,input->'measure'->>'id') requirement_id,state,coalesce(result->>'outcome','unknown') FROM jobs WHERE service_id=$1
 AND (NOT input ? 'scope_revision' OR (input->>'scope_revision')::int=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope' AND id='scope'))
 AND (coalesce((input->>'profile_revision')::int,0)=0 OR (input->>'profile_revision')::int=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='profile' AND id='profile'))
 AND (coalesce((input->>'configuration_revision')::int,0)=0 OR (input->>'configuration_revision')::int=(SELECT revision FROM collection_schedules WHERE service_id=$1))
 ORDER BY requirement_id,check_id,input->'measure'->>'id',created_at DESC`, service)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var rid, state, outcome string
		if err = rows.Scan(&rid, &state, &outcome); err != nil {
			rows.Close()
			return nil, err
		}
		if state == "failed" || state == "timed-out" {
			outcome = "error"
		}
		if rank(outcome) == 0 {
			outcome = "unknown"
		}
		if rank(outcome) > rank(c.technical[rid]) {
			c.technical[rid] = outcome
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = q.Query(ctx, `SELECT requirement_id,max(updated_at) FROM jobs WHERE service_id=$1 AND (result->>'outcome' IN ('fail','error','unknown') OR state IN ('failed','timed-out')) GROUP BY requirement_id`, service)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var rid string
		var at time.Time
		if err = rows.Scan(&rid, &at); err != nil {
			rows.Close()
			return nil, err
		}
		c.failures[rid] = &at
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = q.Query(ctx, `SELECT DISTINCT ON(data->>'requirement_id') data->>'requirement_id',data->>'state' FROM collection_entries WHERE service_id=$1 AND plan_revision=(SELECT max(revision) FROM collection_plans WHERE service_id=$1) ORDER BY data->>'requirement_id',id`, service)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var rid, state string
		if err = rows.Scan(&rid, &state); err != nil {
			rows.Close()
			return nil, err
		}
		c.automation[rid] = state
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = q.Query(ctx, `SELECT l.requirement_id,max(o.received_at),max(o.received_at) FILTER(WHERE o.scope_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope' AND id='scope') AND o.profile_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='profile' AND id='profile') AND (coalesce(jsonb_array_length(l.data->'conditions'),0)=0 OR (l.data->'conditions') ? f.condition))
 FROM finding_events f JOIN observations o ON o.service_id=f.service_id AND o.id=f.observation_id JOIN observation_links l ON l.service_id=o.service_id AND l.observation_id=o.id WHERE f.service_id=$1 AND f.outcome='fail' GROUP BY l.requirement_id`, service)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var rid string
		var at time.Time
		var current *time.Time
		if err = rows.Scan(&rid, &at, &current); err != nil {
			rows.Close()
			return nil, err
		}
		c.findingTimes[rid] = at
		if current != nil {
			c.contradictions[rid] = *current
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = q.Query(ctx, `SELECT DISTINCT ON(l.requirement_id,o.target_key,l.measure_id) l.requirement_id,o.data,o.observed_at,o.source_kind,(m.data->>'freshness_hours')::int,l.data,o.id
 FROM observation_links l JOIN current_resource_observations o ON o.service_id=l.service_id AND o.id=l.observation_id
 JOIN records m ON m.service_id=l.service_id AND m.kind='measure' AND m.id=l.measure_id AND m.revision=l.measure_revision
 WHERE l.service_id=$1 AND o.scope_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope' AND id='scope')
 AND o.profile_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='profile' AND id='profile')
 AND l.measure_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='measure' AND id=l.measure_id)
 AND (coalesce((m.data->>'configuration_revision')::int,0)=0 OR (m.data->>'configuration_revision')::int=(SELECT revision FROM collection_schedules WHERE service_id=$1))
 ORDER BY l.requirement_id,o.target_key,l.measure_id,o.observed_at DESC,o.received_at DESC`, service)
	if err != nil {
		return nil, err
	}
	seen := map[string]map[string]bool{}
	for rows.Next() {
		var rid, source, observationID string
		var raw, linkRaw []byte
		var at time.Time
		var hours int
		if err = rows.Scan(&rid, &raw, &at, &source, &hours, &linkRaw, &observationID); err != nil {
			rows.Close()
			return nil, err
		}
		var in observation.Submission
		var link observation.Link
		if err = json.Unmarshal(raw, &in); err != nil {
			rows.Close()
			return nil, err
		}
		if err = json.Unmarshal(linkRaw, &link); err != nil {
			rows.Close()
			return nil, err
		}
		v := c.collection[rid]
		v.Checks = append(v.Checks, observation.CheckRows(in, link, source, observationID, at, hours, now)...)
		if seen[rid] == nil {
			seen[rid] = map[string]bool{}
		}
		seen[rid][link.Measure.ID] = true
		v.Count++
		v.Expected += in.Coverage.Expected
		v.Observed += in.Coverage.Observed
		if v.LastObservedAt == nil || at.After(*v.LastObservedAt) {
			copy := at
			v.LastObservedAt = &copy
		}
		if v.Freshness == "" {
			v.Freshness = "current"
		}
		if at.Add(time.Duration(hours) * time.Hour).Before(now) {
			v.Freshness = "stale"
		}
		if !slices.Contains(v.Sources, source) {
			v.Sources = append(v.Sources, source)
		}
		if partitionAccessOnly(in) {
			c.collection[rid] = v
			continue
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
			} else if outcome == "pass" && evaluation.Outcome != "pass" {
				outcome = "unknown"
			}
		}
		if matched == 0 {
			outcome = "unknown"
		}
		if in.Coverage.Error != "" {
			outcome = "error"
			target := observationTargetLabel(in.Target)
			v.Errors = append(v.Errors, target+": "+in.Coverage.Error)
			v.Recovery = append(v.Recovery, observation.ExplainError(in.Coverage.Error, target, source))
		}
		if rank(outcome) > rank(v.Technical) {
			v.Technical = outcome
		}
		c.collection[rid] = v
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = q.Query(ctx, `SELECT requirement_id,data FROM (SELECT DISTINCT ON(l.requirement_id,f.finding_id) l.requirement_id,f.finding_id,f.outcome,f.data || jsonb_build_object('artifact_schema',o.data->'artifact'->>'schema','configuration',o.data->'artifact'->'configuration','coverage',o.data->'coverage','target',o.data->'target','observed_at',o.observed_at,'observation_id',o.id) AS data FROM finding_events f JOIN current_resource_observations o ON o.service_id=f.service_id AND o.id=f.observation_id JOIN observation_links l ON l.service_id=f.service_id AND l.observation_id=f.observation_id WHERE f.service_id=$1 AND o.scope_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope' AND id='scope') AND o.profile_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='profile' AND id='profile') AND (coalesce(jsonb_array_length(l.data->'conditions'),0)=0 OR (l.data->'conditions') ? f.condition) ORDER BY l.requirement_id,f.finding_id,o.observed_at DESC,o.received_at DESC) f WHERE outcome='fail'`, service)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var rid string
		var raw []byte
		if err = rows.Scan(&rid, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		var f observation.Finding
		if err = json.Unmarshal(raw, &f); err != nil {
			rows.Close()
			return nil, err
		}
		v := c.collection[rid]
		v.Findings = append(v.Findings, f)
		c.collection[rid] = v
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	view := *s
	view.reader = q
	inventories, err := view.inventorySummaries(ctx, service)
	if err != nil {
		return nil, err
	}
	for rid := range inventories {
		if _, ok := c.collection[rid]; !ok {
			c.collection[rid] = ObservationStatus{}
		}
	}
	for rid, v := range c.collection {
		if inventory, ok := inventories[rid]; ok {
			v.Inventory = &inventory
		}
		v.MissingMeasures = max(0, v.MissingMeasures-len(seen[rid]))
		if v.Freshness == "" {
			v.Freshness = "missing"
		}
		if v.Technical == "" {
			v.Technical = "unknown"
		}
		if v.Freshness == "stale" && v.Technical == "pass" {
			v.Technical = "unknown"
		}
		if len(v.Findings) > 0 && v.Technical != "error" {
			v.Technical = "fail"
		}
		if v.MissingMeasures > 0 && v.Technical == "pass" {
			v.Technical = "unknown"
		}
		if v.Findings == nil {
			v.Findings = []observation.Finding{}
		}
		if v.Sources == nil {
			v.Sources = []string{}
		}
		c.collection[rid] = v
	}
	return c, nil
}

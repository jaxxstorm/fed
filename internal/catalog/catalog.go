// Package catalog imports immutable official source snapshots. Normalization
// preserves the original rule and container metadata for later interpretation.
package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaxxstorm/fed/internal/storage"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const PinnedCommit = "a89d78477ddea98dc57eca24f5007794c2b4eeb6"

type schemaRegexp struct{ *regexp2.Regexp }

func (r schemaRegexp) MatchString(s string) bool {
	matched, err := r.Regexp.MatchString(s)
	return err == nil && matched
}
func compileRegexp(pattern string) (jsonschema.Regexp, error) {
	r, err := regexp2.Compile(pattern, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	r.MatchTimeout = time.Second
	return schemaRegexp{r}, nil
}

type Object map[string]any

func object(v any) Object {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	if m, ok := v.(Object); ok {
		return m
	}
	return Object{}
}
func text(m Object, key string) string { s, _ := m[key].(string); return s }
func values(v any) []string {
	var out []string
	if a, ok := v.([]any); ok {
		for _, s := range a {
			if t, ok := s.(string); ok {
				out = append(out, t)
			}
		}
	}
	return out
}
func decode(b []byte) (Object, error) {
	var m Object
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	err := d.Decode(&m)
	return m, err
}

type Requirement struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Document      string `json:"document"`
	Subset        string `json:"subset,omitempty"`
	Track         string `json:"track"`
	NarrativePath string `json:"narrative_path"`
	Raw           Object `json:"raw"`
	Metadata      Object `json:"metadata"`
}
type Snapshot struct {
	ID              string
	Commit          string
	SourceURL       string
	Payload, Schema []byte
	Requirements    []Requirement
}

func Parse(payload, schema []byte, commit string) (Snapshot, error) {
	if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(commit) {
		return Snapshot{}, errors.New("source commit must be a full Git SHA")
	}
	doc, err := decode(payload)
	if err != nil {
		return Snapshot{}, err
	}
	schemaDoc, err := decode(schema)
	if err != nil {
		return Snapshot{}, err
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseRegexpEngine(compileRegexp)
	if err = c.AddResource("https://ksi.local/catalog-schema.json", map[string]any(schemaDoc)); err != nil {
		return Snapshot{}, err
	}
	compiled, err := c.Compile("https://ksi.local/catalog-schema.json")
	if err != nil {
		return Snapshot{}, err
	}
	if err = compiled.Validate(map[string]any(doc)); err != nil {
		return Snapshot{}, fmt.Errorf("catalog schema validation: %w", err)
	}
	s := Snapshot{ID: commit + ":" + storage.Hash(payload), Commit: commit, SourceURL: "https://raw.githubusercontent.com/FedRAMP/rules/" + commit + "/fedramp-consolidated-rules.json", Payload: payload, Schema: schema}
	seen := map[string]bool{}
	for name, v := range object(doc["FRR"]) {
		group := object(v)
		info := object(group["info"])
		for track, v := range object(group["data"]) {
			for subset, v := range object(v) {
				for id, v := range object(v) {
					if seen[id] {
						return Snapshot{}, fmt.Errorf("duplicate rule ID %s requires explicit track resolution", id)
					}
					seen[id] = true
					raw := object(v)
					s.Requirements = append(s.Requirements, Requirement{ID: id, Kind: "FRR", Name: text(raw, "name"), Document: name, Subset: subset, Track: track, Raw: raw, Metadata: info, NarrativePath: "2026-markdown/providers/20x/rules/" + text(info, "web_name") + ".md"})
				}
			}
		}
	}
	for name, v := range object(doc["KSI"]) {
		group := object(v)
		for id, v := range object(group["indicators"]) {
			raw := object(v)
			s.Requirements = append(s.Requirements, Requirement{ID: id, Kind: "KSI", Name: text(raw, "name"), Document: name, Track: "20x", Raw: raw, Metadata: Object{"status": group["status"]}, NarrativePath: "2026-markdown/providers/20x/key-security-indicators/" + text(group, "web_name") + ".md"})
		}
	}
	sort.Slice(s.Requirements, func(i, j int) bool { return s.Requirements[i].ID < s.Requirements[j].ID })
	return s, nil
}

// Import runs only via the administrator's explicit import command. A rejected
// payload cannot change activation or partially populate a catalog.
func Import(ctx context.Context, db *pgxpool.Pool, payload, schema []byte, commit string) (string, error) {
	s, err := Parse(payload, schema, commit)
	if err != nil {
		return "", err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(817033)`); err != nil {
		return "", err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalogs WHERE id=$1)`, s.ID).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		if _, err = tx.Exec(ctx, `INSERT INTO catalogs(id,commit_sha,source_url,payload_hash,schema_hash,payload,schema_payload) VALUES($1,$2,$3,$4,$5,$6,$7)`, s.ID, s.Commit, s.SourceURL, storage.Hash(payload), storage.Hash(schema), payload, schema); err != nil {
			return "", err
		}
		for _, r := range s.Requirements {
			b, err := json.Marshal(r)
			if err != nil {
				return "", err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO requirements(catalog_id,id,kind,data) VALUES($1,$2,$3,$4)`, s.ID, r.ID, r.Kind, b); err != nil {
				return "", err
			}
		}
	} else {
		var hash string
		if err = tx.QueryRow(ctx, `SELECT schema_hash FROM catalogs WHERE id=$1`, s.ID).Scan(&hash); err != nil {
			return "", err
		}
		if hash != storage.Hash(schema) {
			return "", errors.New("existing catalog has a different schema hash")
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO catalog_activation(singleton,catalog_id) VALUES(true,$1) ON CONFLICT(singleton) DO UPDATE SET catalog_id=excluded.catalog_id`, s.ID); err != nil {
		return "", err
	}
	return s.ID, tx.Commit(ctx)
}

func Load(ctx context.Context, db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, id string) ([]Requirement, error) {
	rows, err := db.Query(ctx, `SELECT data FROM requirements WHERE catalog_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Requirement{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var r Requirement
		if err = json.Unmarshal(b, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type Profile struct {
	CatalogID             string `json:"catalog_id"`
	Type                  string `json:"type"`
	Path                  string `json:"path"`
	Class                 string `json:"class"`
	Date                  string `json:"date"`
	Context               string `json:"context"`
	NextAssessmentStarted *bool  `json:"next_assessment_started,omitempty"`
}

func (p Profile) Validate() error {
	if p.CatalogID == "" || p.Type != "20x" || !slices.Contains([]string{"Program", "Agency"}, p.Path) || !slices.Contains([]string{"A", "B", "C", "D"}, p.Class) || !slices.Contains([]string{"initial", "ongoing"}, p.Context) {
		return errors.New("invalid certification profile")
	}
	_, err := time.Parse(time.DateOnly, p.Date)
	return err
}

type Resolved struct {
	Requirement   Requirement `json:"requirement"`
	Applicability string      `json:"applicability"`
	Force         string      `json:"force,omitempty"`
	Statement     string      `json:"statement"`
	Details       Object      `json:"details"`
	Reason        string      `json:"reason"`
	IncludedBy    string      `json:"included_by,omitempty"`
}

func strength(force string) string {
	switch force {
	case "MUST", "MUST NOT":
		return "required"
	case "SHOULD", "SHOULD NOT":
		return "recommended"
	case "MAY":
		return "optional"
	}
	return "unresolved"
}

func Resolve(all []Requirement, p Profile) ([]Resolved, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	classRefs := map[string]string{}
	if p.Class == "A" && p.Path == "Program" {
		for _, r := range all {
			if slices.Contains([]string{"FRC-CLA-MFR", "FRC-CLA-RFR", "FRC-CLA-OFR"}, r.ID) {
				for _, id := range values(r.Raw["related"]) {
					classRefs[id] = r.ID
				}
			}
		}
	}
	out := make([]Resolved, 0, len(all))
	for _, r := range all {
		out = append(out, resolve(r, p, classRefs[r.ID]))
	}
	return out, nil
}

func resolve(r Requirement, p Profile, include string) Resolved {
	x := Resolved{Requirement: r, Applicability: "unresolved", Statement: text(r.Raw, "statement"), Details: r.Raw, IncludedBy: include}
	set := func(status, reason string) Resolved { x.Applicability = status; x.Reason = reason; return x }
	if r.Track != "all" && r.Track != p.Type {
		return set("not_applicable", "Different certification type")
	}
	if text(r.Metadata, "status") != "stable" {
		return set("unresolved", "Source ruleset is not stable")
	}
	if r.Kind == "FRR" {
		app := object(object(object(r.Metadata["subsets"])[r.Subset])["applicability"])
		if override := object(object(object(object(r.Metadata[p.Type])["subsets"])[r.Subset])["applicability"]); len(override) > 0 {
			app = override
		}
		if len(app) == 0 {
			return set("unresolved", "Missing subset applicability")
		}
		if !slices.Contains(values(app["types"]), p.Type) || !slices.Contains(values(app["paths"]), p.Path) {
			return set("not_applicable", "Different subset type or path")
		}
		if !slices.Contains(values(r.Raw["affects"]), "Providers") && !slices.Contains(values(r.Raw["affects"]), "Everyone") {
			return set("not_applicable", "Different affected audience")
		}
		if !slices.Contains(values(app["classes"]), p.Class) && include == "" {
			if _, ok := object(r.Raw["varies_by_class"])[strings.ToLower(p.Class)]; ok {
				return set("unresolved", "Class variant conflicts with subset applicability")
			}
			return set("not_applicable", "Class excluded by subset")
		}
	} else {
		if p.Path != "Program" {
			return set("unresolved", "KSI applicability for this path requires review")
		}
		if p.Class == "A" && include == "" {
			return set("not_applicable", "Not included in Class A enumerated obligations")
		}
	}
	variant := object(r.Raw["varies_by_class"])
	selected := r.Raw
	if len(variant) > 0 {
		selected = object(variant[strings.ToLower(p.Class)])
		if len(selected) == 0 {
			return set("unresolved", "No explicit variant for selected class")
		}
		x.Statement = text(selected, "statement")
		x.Details = selected
	}
	x.Force = text(selected, "force")
	x.Applicability = strength(x.Force)
	if r.Kind == "KSI" {
		x.Applicability = "required"
		if strings.HasPrefix(x.Statement, "**Optional:**") {
			x.Applicability = "optional"
		}
	}
	if include != "" {
		switch include {
		case "FRC-CLA-MFR":
			x.Applicability = "required"
		case "FRC-CLA-RFR":
			x.Applicability = "recommended"
		case "FRC-CLA-OFR":
			x.Applicability = "optional"
		}
	}
	// Source dates are metadata, not assumed effective merely because imported.
	effective := object(r.Metadata["effective"])
	if len(effective) == 0 {
		effective = object(object(r.Metadata[p.Type])["effective"])
	}
	if text(effective, "is") == "no" {
		return set("not_applicable", "Source marks ruleset inactive")
	}
	if text(effective, "is") == "optional" {
		x.Applicability = "optional"
	}
	dates := object(effective["date"])
	if d := object(r.Raw["effective_date"]); len(d) > 0 {
		dates = d
	}
	if d := object(selected["effective_date"]); len(d) > 0 {
		dates = d
	}
	dateKey := "obtain"
	if p.Context == "ongoing" {
		dateKey = "maintain"
	}
	if starts := text(dates, dateKey); starts != "" {
		if p.Date < starts {
			if optional := text(dates, "optional_adoption"); optional != "" && p.Date >= optional {
				return set("optional", "Optional adoption before required effective date")
			}
			return set("not_applicable", "Requirement not yet effective")
		}
		grace := object(dates["grace"])
		if p.Context == "ongoing" {
			if until := text(grace, "default"); until != "" && p.Date < until {
				return set("optional", "Within published transition grace period")
			}
			if waits, _ := grace["until_next_assessment"].(bool); waits && (p.NextAssessmentStarted == nil || !*p.NextAssessmentStarted) {
				return set("unresolved", "Transition grace depends on next independent assessment")
			}
		}
	}
	lower := strings.ToLower(x.Statement)
	if strings.Contains(lower, "pilot") || strings.Contains(lower, "if applicable") || strings.Contains(lower, "unless") {
		return set("unresolved", "Source contains a condition requiring applicability review")
	}
	x.Reason = "Resolved from source class and profile metadata"
	return x
}

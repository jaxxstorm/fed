package guidancecontent

import (
	"encoding/json"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/readiness"
	"github.com/jaxxstorm/fed/internal/storage"
)

func TestTechnicalDomainsExactInventoryAndSources(t *testing.T) {
	rules, markdown := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := fs.ReadFile(rules, "schema.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := catalog.Parse(raw, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	i, err := catalog.Inventory(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		domain string
		build  func(fs.FS, fs.FS) (readiness.Package, error)
		count  int
		digest string
	}{
		{"CNA", CNA, 26, "f1cb86f6097425562207a6c795f8e03287f34be2070e20efa9f47fe81e75af2f"},
		{"IAM", IAM, 18, "aa9528f1b070e059b474ebde27ecc2b4ad31ea377bce49a1d1b7943840ca9be6"},
		{"MLA", MLA, 17, "475a2d72bf9bbd8c44182908ad59f2ead8e3f3c280b81bc7d0c246739799b79a"},
		{"SVC", SVC, 30, "6f94c364a852c723575691eada69774cc0c6add1ce4127c351db176a218e7af8"},
		{"CMT", CMT, 12, "ad4afb507a316ab1512422499e5288c63bfb443946068b3576299a262f9567a2"},
		{"SCR", SCR, 6, "ac53c616c0963c6987674998f811ccbf017d700a7c37293895cad96ba56859fe"},
		{"PIY", PIY, 15, "ef0a6d67d5b78bf193858d3b992d94db83885289d09e40bc24fb0746a31d5c96"},
	} {
		t.Run(tc.domain, func(t *testing.T) {
			p, err := tc.build(rules, markdown)
			if err != nil {
				t.Fatal(err)
			}
			if err = readiness.ValidateInventory(p, i); err != nil {
				t.Fatal(err)
			}
			if err = readiness.VerifySources(p, rules, markdown); err != nil {
				t.Fatal(err)
			}
			want := map[string]catalog.GuidanceVariant{}
			for _, v := range i.Variants {
				if strings.HasPrefix(v.RequirementID, "KSI-"+tc.domain+"-") {
					want[v.ID] = v
				}
			}
			ids := []string{}
			for _, b := range p.Playbooks {
				if b.Review != nil {
					t.Fatal("fabricated authenticated review", b.ID)
				}
				for _, id := range b.VariantIDs {
					v, ok := want[id]
					if !ok {
						t.Fatal("foreign/duplicate variant", id)
					}
					delete(want, id)
					ids = append(ids, id)
					if b.OfficialStatement != v.Statement {
						t.Fatal("statement/force changed", id)
					}
					switch v.Applicability {
					case "required", "optional":
						if b.Kind != "actionable" {
							t.Fatal("lost actionable variant", id)
						}
						if v.Applicability == "optional" && !strings.Contains(b.Applicability.Text, "optional") {
							t.Fatal("lost optionality", id)
						}
						for _, a := range b.Assertions {
							if len(a.Acceptance.Alternatives) < 2 {
								t.Fatal("missing evidence alternative", b.ID)
							}
						}
					case "not_applicable":
						if b.Kind != "excluded" || len(b.Assertions) != 0 {
							t.Fatal("exclusion imposes obligations", id)
						}
					case "unresolved":
						if b.Kind != "needs_decision" || len(b.Assertions) != 0 || !strings.Contains(b.DecisionQuestion, v.Reason) {
							t.Fatal("lost unresolved reason", id)
						}
					}
				}
			}
			sort.Strings(ids)
			if len(ids) != tc.count || len(want) != 0 || storage.Hash([]byte(strings.Join(ids, "\n")+"\n")) != tc.digest {
				t.Fatal("exact authored denominator changed", len(ids), len(want))
			}
			for _, c := range p.Capabilities {
				if technicalCapabilitySetup(c.ID) == "" {
					t.Fatal("missing shared capability authoring", c.ID)
				}
			}
			// Every requirement has a rules excerpt and a real heading in its own page.
			for _, c := range p.Citations {
				if !strings.HasSuffix(c.ID, "-page") {
					continue
				}
				page, err := fs.ReadFile(markdown, c.Path)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, line := range strings.Split(string(page), "\n") {
					if strings.HasPrefix(line, "### ") {
						anchor := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(line, "### "), ",", ""), " ", "-"))
						found = found || anchor == c.Anchor
					}
				}
				if !found {
					t.Fatal("citation heading missing", c.ID, c.Anchor)
				}
			}
		})
	}
}

func TestTechnicalDraftSourceDriftRejected(t *testing.T) {
	rules, markdown := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	// Overlay one changed primary source; all other reads use the real pinned FS.
	for _, tc := range []struct{ path, old, new string }{
		{"providers/20x/key-security-indicators/supply-chain-risk.md", "Persistently identify, review, and mitigate potential supply chain risks.", "Identify supply chain risks."},
		{"definitions.md", persistentDefinition, "A single annual check."},
		{"providers/20x/key-security-indicators/index.md", "We are intentionally vague about the actual activities expected for each Key Security", "Changed narrative about each Key Security"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			data, err := fs.ReadFile(markdown, tc.path)
			if err != nil {
				t.Fatal(err)
			}
			changed := strings.Replace(string(data), tc.old, tc.new, 1)
			if changed == string(data) {
				t.Fatal("bad mutation")
			}
			m := technicalOverlayFS{FS: markdown, overrides: fstest.MapFS{tc.path: &fstest.MapFile{Data: []byte(changed)}}}
			if _, err = SCR(rules, m); err == nil {
				t.Fatal("accepted changed source", tc.path)
			}
		})
	}
}

func TestTechnicalDraftRejectsMetadataOnlyVariantDrift(t *testing.T) {
	rules, markdown := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	indicators := doc["KSI"].(map[string]any)["SCR"].(map[string]any)["indicators"].(map[string]any)
	indicators["KSI-SCR-MIT"].(map[string]any)["name"] = "Changed upstream indicator name"
	changed, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	m := technicalOverlayFS{FS: rules, overrides: fstest.MapFS{"rules.json": &fstest.MapFile{Data: changed}}}
	if _, err = SCR(m, markdown); err == nil || !strings.Contains(err.Error(), "exact variant identities changed") {
		t.Fatal("metadata drift must require reauthoring", err)
	}
}

type technicalOverlayFS struct {
	fs.FS
	overrides fstest.MapFS
}

// Export content-bound review inputs only when explicitly requested during a
// source review. This creates no authenticated Review and no approval result.
func TestTechnicalReviewInputs(t *testing.T) {
	path := os.Getenv("TECHNICAL_REVIEW_INPUTS")
	if path == "" {
		t.Skip("source-review export not requested")
	}
	rules, markdown := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	type input struct {
		Package readiness.Package `json:"package"`
		Hashes  map[string]string `json:"hashes"`
	}
	inputs := map[string]input{}
	for domain, build := range map[string]func(fs.FS, fs.FS) (readiness.Package, error){"CNA": CNA, "IAM": IAM, "MLA": MLA, "SVC": SVC, "CMT": CMT, "SCR": SCR, "PIY": PIY} {
		p, err := build(rules, markdown)
		if err != nil {
			t.Fatal(err)
		}
		hashes := map[string]string{}
		for _, b := range p.Playbooks {
			h, err := readiness.ReviewHash(p, b.ID)
			if err != nil {
				t.Fatal(err)
			}
			hashes[b.ID] = h
		}
		inputs[domain] = input{p, hashes}
	}
	raw, err := json.MarshalIndent(inputs, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func (m technicalOverlayFS) Open(name string) (fs.File, error) {
	if _, ok := m.overrides[name]; ok {
		return m.overrides.Open(name)
	}
	return m.FS.Open(name)
}

func TestTechnicalConditionalAndCompositeOutcomes(t *testing.T) {
	rules, markdown := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	for _, tc := range []struct {
		build  func(fs.FS, fs.FS) (readiness.Package, error)
		checks map[string][]string
	}{
		{CNA, map[string][]string{"KSI-CNA-EIS": {"automatically enforce", "all machine-based"}, "KSI-CNA-MAT": {"lateral movement", "minimal attack surface"}, "KSI-CNA-RNT": {"inbound", "outbound"}}},
		{IAM, map[string][]string{"KSI-IAM-APM": {"when feasible", "otherwise strong passwords", "with phishing-resistant MFA"}, "KSI-IAM-JIT": {"role and attribute-based", "just-in-time", "all user and non-user"}}},
		{SVC, map[string][]string{"KSI-SVC-RUD": {"when requested by an agency", "including from backups if appropriate"}, "KSI-SVC-ASM": {"regular rotation", "Management", "protection"}, "KSI-SVC-EIS": {"those improvements are persistently made"}}},
		{CMT, map[string][]string{"KSI-CMT-RMV": {"wherever reasonable"}, "KSI-CMT-VTD": {"testing", "validation"}}},
		{SCR, map[string][]string{"KSI-SCR-MON": {"automatically monitored", "may include contractual"}, "KSI-SCR-MIT": {"identify", "review", "mitigate"}}},
		{PIY, map[string][]string{"KSI-PIY-GIV": {"Authoritative sources", "all information resources", "real-time"}, "KSI-PIY-RSD": {"security", "privacy considerations", "CISA Secure By Design"}}},
	} {
		p, err := tc.build(rules, markdown)
		if err != nil {
			t.Fatal(err)
		}
		for requirement, clauses := range tc.checks {
			found := false
			for _, b := range p.Playbooks {
				if b.RequirementID != requirement || b.Kind != "actionable" {
					continue
				}
				found = true
				for _, clause := range clauses {
					covered := false
					for _, a := range b.Assertions {
						for _, c := range a.Clauses {
							covered = covered || strings.Contains(c, clause)
						}
					}
					if !covered {
						t.Fatal("lost condition/facet", requirement, clause)
					}
				}
				if !strings.Contains(b.ReviewSteps.Items[0].CompletionCriteria[0].Text, "unknown implementation") {
					t.Fatal("missing evidence improperly implies failure", requirement)
				}
			}
			if !found {
				t.Fatal("no actionable conditional outcome", requirement)
			}
		}
	}
}

func TestTechnicalSourceComparisons(t *testing.T) {
	rules, markdown := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	wantAssertions := map[string]int{
		"KSI-CNA-DFP": 2, "KSI-CNA-EIS": 2, "KSI-CNA-IBP": 2, "KSI-CNA-MAT": 2, "KSI-CNA-OFA": 2, "KSI-CNA-RNT": 2, "KSI-CNA-RVP": 2, "KSI-CNA-ULN": 2,
		"KSI-IAM-AAM": 2, "KSI-IAM-APM": 4, "KSI-IAM-ELP": 2, "KSI-IAM-JIT": 4, "KSI-IAM-SNU": 2, "KSI-IAM-SUS": 1,
		"KSI-MLA-ALA": 5, "KSI-MLA-EVC": 2, "KSI-MLA-LET": 4, "KSI-MLA-OSM": 3, "KSI-MLA-RVL": 2,
		"KSI-SVC-ACM": 2, "KSI-SVC-ASM": 3, "KSI-SVC-EIS": 2, "KSI-SVC-PRR": 4, "KSI-SVC-RUD": 2, "KSI-SVC-SIN": 2, "KSI-SVC-VCM": 2, "KSI-SVC-VRI": 1,
		"KSI-CMT-LMC": 2, "KSI-CMT-RMV": 1, "KSI-CMT-RVP": 1, "KSI-CMT-VTD": 2,
		"KSI-SCR-MIT": 3, "KSI-SCR-MON": 2,
		"KSI-PIY-GIV": 3, "KSI-PIY-RES": 2, "KSI-PIY-RIS": 1, "KSI-PIY-RSD": 3, "KSI-PIY-RVD": 1,
	}
	total, assertions := 0, 0
	seen := map[string]bool{}
	for domain, build := range map[string]func(fs.FS, fs.FS) (readiness.Package, error){"CNA": CNA, "IAM": IAM, "MLA": MLA, "SVC": SVC, "CMT": CMT, "SCR": SCR, "PIY": PIY} {
		t.Run(domain, func(t *testing.T) {
			p, err := build(rules, markdown)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile("../../guidance/reviews/" + domain + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var bundle ComparisonBundle
			if err = json.Unmarshal(raw, &bundle); err != nil {
				t.Fatal(err)
			}
			if bundle.RecordingState != "pending_authenticated_service_record" || len(bundle.Comparisons) != len(p.Playbooks) {
				t.Fatal("comparison denominator or recording state changed")
			}
			for _, b := range p.Playbooks {
				total++
				seen[b.RequirementID] = true
				input, ok := bundle.Comparisons[b.ID]
				if !ok {
					t.Fatal("missing exact comparison", b.ID)
				}
				hash, err := readiness.ReviewHash(p, b.ID)
				if err != nil || input.ContentHash != hash {
					t.Fatal("stale comparison", b.ID, err)
				}
				if err = readiness.ValidateComparison(p, b.ID, input.Comparison); err != nil {
					t.Fatal(b.ID, err)
				}
				if len(input.Comparison.Uncertainties) == 0 || b.Review != nil {
					t.Fatal("uncertainty or authenticated-review boundary lost", b.ID)
				}
				// Actual citation closure, not merely one convenient rule citation.
				encoded, err := json.Marshal(input.Comparison)
				if err != nil {
					t.Fatal(err)
				}
				var mutated readiness.SourceComparison
				if err = json.Unmarshal(encoded, &mutated); err != nil {
					t.Fatal(err)
				}
				findings := []*readiness.ReviewFinding{&mutated.Applicability, &mutated.Obligations, &mutated.Prerequisites, &mutated.Evidence, &mutated.Timing}
				for n := range mutated.Uncertainties {
					findings = append(findings, &mutated.Uncertainties[n])
				}
				for _, finding := range findings {
					var kept []string
					for _, ref := range finding.Citations {
						if ref != "class-a" {
							kept = append(kept, ref)
						}
					}
					finding.Citations = kept
				}
				if err = readiness.ValidateComparison(p, b.ID, mutated); err == nil || !strings.Contains(err.Error(), "omitted bound citation class-a") {
					t.Fatal("accepted omitted applicability source", b.ID, err)
				}
				changed := p
				changed.Playbooks = append([]readiness.Playbook(nil), p.Playbooks...)
				for n := range changed.Playbooks {
					if changed.Playbooks[n].ID == b.ID {
						changed.Playbooks[n].Purpose.Text += " Changed interpretation."
					}
				}
				if _, err = RebindComparison(changed, p, b.ID, input); err == nil || !strings.Contains(err.Error(), "stale") {
					t.Fatal("accepted stale source comparison", b.ID, err)
				}
				if b.Kind != "actionable" {
					if len(b.Assertions) != 0 || len(b.Setup.Items) != 0 || len(b.Operate.Items) != 0 {
						t.Fatal("non-actionable checklist", b.ID)
					}
					continue
				}
				assertions += len(b.Assertions)
				if len(b.Assertions) != wantAssertions[b.RequirementID] {
					t.Fatal("separate proof duties changed", b.ID, len(b.Assertions))
				}
				if technicalPrerequisite[b.RequirementID] == "" || len(b.Setup.Items) != 2 || b.Setup.Items[1].ID != "establish-outcome-prerequisites" {
					t.Fatal("missing substantive prerequisite", b.ID)
				}
				facts := map[string]bool{}
				for _, a := range b.Assertions {
					if facts[a.Acceptance.Fact] {
						t.Fatal("duplicate acceptance duty", b.ID)
					}
					facts[a.Acceptance.Fact] = true
					if len(b.Assertions) > 1 {
						for _, clause := range a.Clauses {
							if clause == b.OfficialStatement || clause == strings.TrimPrefix(b.OfficialStatement, "**Optional:** ") {
								t.Fatal("aggregate clause hides proof duties", b.ID)
							}
						}
					}
				}
			}
		})
	}
	if total != 124 || len(seen) != 38 {
		t.Fatal("review denominator changed", total, len(seen))
	}
	t.Logf("validated %d exact comparisons, %d indicators, %d actionable assertions", total, len(seen), assertions)
}

func TestTechnicalClassDIsNotInheritedOptionality(t *testing.T) {
	rules, markdown := os.DirFS("../../testdata/fedramp"), os.DirFS("../../2026-markdown")
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := fs.ReadFile(rules, "schema.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := catalog.Parse(raw, schema, catalog.PinnedCommit)
	if err != nil {
		t.Fatal(err)
	}
	i, err := catalog.Inventory(s)
	if err != nil {
		t.Fatal(err)
	}
	profiles := map[string]catalog.Profile{}
	for _, p := range i.Profiles {
		profiles[p.ID] = p.Profile
	}
	books := map[string]readiness.Playbook{}
	for _, build := range []func(fs.FS, fs.FS) (readiness.Package, error){CNA, MLA, SVC} {
		p, err := build(rules, markdown)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range p.Playbooks {
			books[b.VariantIDs[0]] = b
		}
	}
	varies := map[string]bool{"KSI-CNA-EIS": true, "KSI-MLA-ALA": true, "KSI-SVC-PRR": true, "KSI-SVC-RUD": true, "KSI-SVC-VCM": true}
	d, optional := map[string]bool{}, map[string]bool{}
	for _, v := range i.Variants {
		if !varies[v.RequirementID] {
			continue
		}
		b := books[v.ID]
		for _, pid := range v.Profiles {
			profile := profiles[pid]
			if profile.Path != "Program" {
				continue
			}
			if profile.Class == "D" {
				d[v.RequirementID] = true
				if v.Applicability != "unresolved" || v.Reason != "No explicit variant for selected class" || b.Kind != "needs_decision" || len(b.Assertions) != 0 {
					t.Fatal("Class D inherited unsupported force", v.RequirementID)
				}
			}
			if v.Applicability == "optional" {
				optional[v.RequirementID] = true
				if profile.Class != "B" || !strings.Contains(b.Applicability.Text, "Class D has a separate unresolved variant") {
					t.Fatal("optional class mislabel", v.RequirementID, profile.Class)
				}
			}
		}
	}
	if len(d) != 5 || len(optional) != 5 {
		t.Fatal("missing class variants", len(d), len(optional))
	}
}

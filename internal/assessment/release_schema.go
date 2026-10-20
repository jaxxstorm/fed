package assessment

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const ReleaseManifestSchema = `{
 "$schema":"https://json-schema.org/draft/2020-12/schema",
 "$id":"https://fed.local/schemas/assessment-release-v1.json",
 "type":"object",
 "required":["format","label","service_id","prepared_at","period","profile","profile_revision","scope","context","requirements","evidence","public","changes","blockers","incomplete"],
 "properties":{
   "format":{"const":"fed-assessment-release/v1"},
   "label":{"type":"string","minLength":1},
   "service_id":{"type":"string","minLength":1},
   "prepared_at":{"type":"string","format":"date-time"},
   "period":{"type":"object","required":["start","end"],"properties":{"start":{"type":"string","format":"date-time"},"end":{"type":"string","format":"date-time"}},"additionalProperties":false},
   "profile":{"type":"object","required":["catalog_id"]},
   "profile_revision":{"type":"integer","minimum":1},
   "scope":{"type":"object","required":["id","revision","data"]},
   "context":{"type":"object","required":["description","contacts","boundary","responsibilities","deliverables"]},
   "requirements":{"type":"array","items":{"type":"object","required":["requirement","expected_deliverables","collection","issues"]}},
   "evidence":{"type":"array","items":{"type":"object","required":["reference","hash","path","metadata"],"properties":{"hash":{"type":"string","pattern":"^[a-f0-9]{64}$"},"path":{"type":"string","pattern":"^evidence/[a-f0-9]{64}$"},"metadata":{"type":"object"},"reference":{"type":"object","required":["id","revision"],"properties":{"id":{"type":"string","minLength":1},"revision":{"type":"integer","minimum":1}}}},"additionalProperties":false}},
   "public":{"type":"object","required":["service_name","summary","contacts","requirements"],"properties":{"service_name":{"type":"string","minLength":1},"summary":{"type":"string"},"contacts":{"type":["array","null"],"items":{"type":"string"}},"requirements":{"type":["array","null"],"items":{"type":"object","required":["requirement_id","summary"],"additionalProperties":false,"properties":{"requirement_id":{"type":"string"},"summary":{"type":"string"}}}}},"additionalProperties":false},
   "previous_release_id":{"type":"string"},
   "changes":{"type":"array","items":{"type":"object","required":["category","explanation"]}},
   "blockers":{"type":"array","items":{"type":"string"}},
   "incomplete":{"type":"boolean"}
 },"additionalProperties":false
}`

var releaseSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	var doc any
	if err := json.Unmarshal([]byte(ReleaseManifestSchema), &doc); err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("https://fed.local/schemas/assessment-release-v1.json", doc); err != nil {
		return nil, err
	}
	return c.Compile("https://fed.local/schemas/assessment-release-v1.json")
})

func validateReleaseManifest(v ReleaseManifest) error {
	schema, err := releaseSchema()
	if err != nil {
		return err
	}
	var doc any
	if err = json.Unmarshal(data(v), &doc); err != nil {
		return err
	}
	if err = schema.Validate(doc); err != nil {
		return fmt.Errorf("%w: invalid assessment release: %v", ErrInvalid, err)
	}
	if v.PreparedAt.IsZero() || v.Period.Start.IsZero() || v.Period.End.Before(v.Period.Start) {
		return ErrInvalid
	}
	seen := map[Ref]bool{}
	for _, e := range v.Evidence {
		if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(e.Hash) || e.Path != "evidence/"+e.Hash || e.Metadata.ArtifactKey != e.Hash || seen[e.Reference] {
			return ErrInvalid
		}
		seen[e.Reference] = true
	}
	return nil
}

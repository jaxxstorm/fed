package observation

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func validSubmission() Submission {
	return Submission{ServiceID: "service", ScopeRevision: 1, ProfileRevision: 1, IdempotencyKey: "key", Target: Target{Kind: "aws"}, ObservedAt: time.Now(), Method: "DescribeDBInstances", ToolVersion: "aws-cli/2", Coverage: Coverage{Expected: 1, Observed: 1, Complete: true, State: "observed"}, Artifact: Artifact{Schema: "aws.rds/v1", Configuration: map[string]json.RawMessage{"storage_encrypted": json.RawMessage(`false`)}}, Links: []Link{{RequirementID: "KSI-SVC-SIN", Measure: Ref{"m", 1}, MappingVersion: "v1", Rationale: "Encryption at rest"}}}
}
func TestObservationValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*Submission)
		want   error
	}{
		{"valid", func(*Submission) {}, nil},
		{"future", func(s *Submission) { s.ObservedAt = time.Now().Add(time.Hour) }, ErrInvalid},
		{"coverage", func(s *Submission) { s.Coverage.Observed = 0 }, ErrInvalid},
		{"secret field", func(s *Submission) { s.Artifact.Configuration["password"] = json.RawMessage(`"secret"`) }, ErrInvalid},
		{"nested arbitrary payload", func(s *Submission) {
			s.Artifact.Configuration["storage_encrypted"] = json.RawMessage(`{"arbitrary":"payload"}`)
		}, ErrInvalid},
		{"oversize", func(s *Submission) {
			s.Artifact.Configuration["engine"] = json.RawMessage(`"` + strings.Repeat("a", MaxArtifactBytes) + `"`)
		}, ErrTooLarge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := validSubmission()
			tt.change(&s)
			if err := s.Validate(time.Now()); !errors.Is(err, tt.want) {
				t.Fatalf("got %v want %v", err, tt.want)
			}
		})
	}
}

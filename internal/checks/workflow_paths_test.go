package checks

import (
	"context"
	"github.com/jaxxstorm/fed/internal/assessment"
	"testing"
)

func TestWorkflowEvidenceExcludesOtherRepositoryDocuments(t *testing.T) {
	for _, path := range []string{".github/CODEOWNERS", "projects/security/__main__.py", ".github/workflows/nested/file.yml", "README.md"} {
		if workflowEvidencePath(path) {
			t.Fatalf("non-workflow selected: %s", path)
		}
	}
	for _, path := range []string{".github/workflows/pulumi.yaml", ".github/workflows/test.yml"} {
		if !workflowEvidencePath(path) {
			t.Fatalf("workflow excluded: %s", path)
		}
	}
	result, err := (GitHub{}).Collect(context.Background(), assessment.Scope{Repositories: []assessment.Repository{{Name: "org/repo", Paths: []string{".github/CODEOWNERS", ".github/workflows/pulumi.yaml"}}}}, assessment.Measure{})
	if err != nil || len(result.Observations) != 1 || result.Observations[0].Error != "github_credentials_missing" {
		t.Fatal("non-workflow checked or genuine workflow error hidden", result, err)
	}
}

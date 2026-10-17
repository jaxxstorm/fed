package assessment

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestEvidenceLibraryFilteringAndRevisionReads(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := s.SaveEvidenceSource(ctx, a, service, "policies", 0, EvidenceSource{Type: "document", Name: "Approved policies", Locator: "https://docs.example/security", ScopeRevision: 1, Enabled: true, FreshnessHours: 48}); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 27; n++ {
		_, err := s.SubmitEvidenceArtifact(ctx, a, ArtifactSubmission{ServiceID: service, IdempotencyKey: fmt.Sprintf("artifact-%d", n), Source: Ref{"policies", 1}, ScopeRevision: 1, Locator: "https://docs.example/security", SourceVersion: fmt.Sprint(n), MediaType: "text/plain", Content: "Approved policy content\n", ObservedAt: now.Add(-time.Hour), RetrievedAt: now, Method: "Read policy", Description: fmt.Sprintf("Policy record %02d", n)})
		if err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.EvidencePage(ctx, a, service, EvidenceFilter{Freshness: "current"}, now)
	if err != nil || len(first) != 26 {
		t.Fatalf("first page: %d %v", len(first), err)
	}
	next, err := s.EvidencePage(ctx, a, service, EvidenceFilter{Offset: 25}, now)
	if err != nil || len(next) != 2 {
		t.Fatalf("next page: %d %v", len(next), err)
	}
	if first[25].ID != next[0].ID {
		t.Fatal("page boundary changed")
	}
	found, err := s.EvidencePage(ctx, a, service, EvidenceFilter{Search: "Policy record 03", Source: "policies"}, now)
	if err != nil || len(found) != 1 {
		t.Fatal("search/source filtering", err)
	}
	content, err := s.ArtifactRevision(ctx, a, service, Ref{found[0].ID, found[0].Revision})
	if err != nil || string(content) != "Approved policy content\n" {
		t.Fatal("exact revision content", err)
	}
	if _, err = s.ArtifactRevision(ctx, a, service, Ref{found[0].ID, found[0].Revision + 1}); err == nil {
		t.Fatal("nonexistent revision returned current content")
	}
	stale, err := s.EvidencePage(ctx, a, service, EvidenceFilter{Freshness: "needs_refresh"}, now.Add(49*time.Hour))
	if err != nil || len(stale) != 26 {
		t.Fatal("freshness filter did not use observation time", err)
	}
	sources, err := s.SourcePage(ctx, a, service, "Approved policies", 0)
	if err != nil || len(sources) != 1 {
		t.Fatal("source filtering", err)
	}
}

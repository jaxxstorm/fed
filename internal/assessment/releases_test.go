package assessment

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/storage"
)

func TestReleaseSnapshotArchiveAndPublication(t *testing.T) {
	s, a, service := agentFixture(t)
	ctx := context.Background()
	contextValue := ServiceContext{Description: "Private implementation detail", Boundary: "Saved service boundary", Responsibilities: "Provider operates service", Contacts: []string{"owner@example.com"}, Deliverables: []ReleaseDeliverable{}}
	observed := time.Now().UTC().Add(-time.Hour)
	if _, err := s.SaveEvidenceSource(ctx, a, service, "policy", 0, EvidenceSource{Type: "document", Name: "Policy", Locator: "https://docs.example/policy", ScopeRevision: 1, Enabled: true, FreshnessHours: 48}); err != nil {
		t.Fatal(err)
	}
	ev, err := s.SubmitEvidenceArtifact(ctx, a, ArtifactSubmission{ServiceID: service, IdempotencyKey: "policy", Source: Ref{ID: "policy", Revision: 1}, ScopeRevision: 1, Locator: "https://docs.example/policy", SourceVersion: "v1", MediaType: "text/plain", Content: "Private implementation evidence", ObservedAt: observed, RetrievedAt: observed, Method: "Read policy", Description: "Policy"})
	if err != nil {
		t.Fatal(err)
	}
	contextValue.Deliverables = append(contextValue.Deliverables, ReleaseDeliverable{RequirementID: "KSI-CNA-RNT", Description: "Supporting policy", Evidence: Ref{ID: ev.ID, Revision: ev.Revision}})
	if _, err := s.SaveServiceContext(ctx, a, service, 0, contextValue); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-time.Minute)
	in := PrepareReleaseInput{ServiceID: service, IdempotencyKey: "release-one", Period: AssessmentPeriod{Start: now.Add(-24 * time.Hour), End: now}, Public: PublicRelease{ServiceName: "Public service", Summary: "Explicitly incomplete assessment", Contacts: []string{}, Requirements: []PublicRequirement{}}}
	release, err := s.PrepareAssessmentRelease(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if !release.Manifest.Incomplete {
		t.Fatal("unassessed release must be incomplete")
	}
	var first, second bytes.Buffer
	if err = s.WriteReleaseArchive(ctx, &first, release.Manifest); err != nil {
		t.Fatal(err)
	}
	if storage.Hash(first.Bytes()) != release.ArchiveHash {
		t.Fatal("archive differs from prepared hash")
	}
	contextValue.Description = "Later changed implementation"
	if _, err = s.SaveServiceContext(ctx, a, service, 1, contextValue); err != nil {
		t.Fatal(err)
	}
	replay, err := s.PrepareAssessmentRelease(ctx, a, in)
	if err != nil || replay.ID != release.ID {
		t.Fatalf("retry: %v", err)
	}
	if err = s.WriteReleaseArchive(ctx, &second, replay.Manifest); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("later edits changed frozen release")
	}
	originalArtifacts := s.Artifacts
	s.Artifacts = corruptReleaseArtifacts{originalArtifacts}
	bad := in
	bad.IdempotencyKey = "corrupt"
	if _, err = s.PrepareAssessmentRelease(ctx, a, bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("corrupt artifact allowed preparation: %v", err)
	}
	s.Artifacts = originalArtifacts
	entered, proceed := make(chan struct{}), make(chan struct{})
	s.Artifacts = &gatedReleaseArtifacts{Artifacts: originalArtifacts, entered: entered, proceed: proceed}
	prepared := make(chan AssessmentRelease, 1)
	prepareErrors := make(chan error, 1)
	concurrent := in
	concurrent.IdempotencyKey = "concurrent"
	go func() { r, e := s.PrepareAssessmentRelease(ctx, a, concurrent); prepared <- r; prepareErrors <- e }()
	<-entered
	edited := contextValue
	edited.Description = "Concurrent service edit"
	editErrors := make(chan error, 1)
	go func() { _, e := s.SaveServiceContext(ctx, a, service, 2, edited); editErrors <- e }()
	close(proceed)
	frozen := <-prepared
	if err = <-prepareErrors; err != nil {
		t.Fatal(err)
	}
	if err = <-editErrors; err != nil {
		t.Fatal(err)
	}
	if frozen.Manifest.Context.Description != contextValue.Description {
		t.Fatal("concurrent edit changed preparation snapshot")
	}
	s.Artifacts = originalArtifacts
	archive, err := zip.NewReader(bytes.NewReader(first.Bytes()), int64(first.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range archive.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(r)
		files[f.Name] = content
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == "manifest.json" && storage.Hash(content) != release.ManifestHash {
			t.Fatal("manifest hash mismatch")
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(string(files["SHA256SUMS"])), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 || storage.Hash(files[parts[1]]) != parts[0] {
			t.Fatalf("offline checksum invalid: %s", line)
		}
	}
	in.Public.Summary = "Changed disclosure"
	if _, err = s.PrepareAssessmentRelease(ctx, a, in); !errors.Is(err, ErrConflict) {
		t.Fatalf("idempotency payload conflict: %v", err)
	}
	if err = s.ConfigureTrustCentre(ctx, a, service, "test-service", true); err != nil {
		t.Fatal(err)
	}
	transition := ReleaseTransition{ServiceID: service, ID: release.ID, ExpectedRevision: 1, ManifestHash: release.ManifestHash, AudienceHash: release.AudienceHash, Action: "approve", Reason: "Reviewed incomplete disclosure", IdempotencyKey: "approve"}
	agent := a
	agent.Kind = "agent"
	if _, err = s.TransitionRelease(ctx, agent, transition); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("agent approval: %v", err)
	}
	transition.AudienceHash = "wrong"
	if _, err = s.TransitionRelease(ctx, a, transition); !errors.Is(err, ErrConflict) {
		t.Fatalf("wrong audience accepted: %v", err)
	}
	transition.AudienceHash = release.AudienceHash
	if _, err = s.TransitionRelease(ctx, a, transition); err != nil {
		t.Fatal(err)
	}
	transition.ExpectedRevision = 2
	transition.Action = "publish"
	transition.IdempotencyKey = "publish"
	if _, err = s.TransitionRelease(ctx, a, transition); err != nil {
		t.Fatal(err)
	}
	public, err := s.PublicReleaseIndex(ctx, "test-service", 10, 0)
	if err != nil || len(public) != 1 {
		t.Fatalf("public index: %v %v", public, err)
	}
	if bytes.Contains(data(public), []byte("Private implementation")) {
		t.Fatal("restricted data leaked")
	}
	member, err := auth.Admit(ctx, s.DB, auth.AdmissionConfig{}, auth.VerifiedIdentity{Issuer: "test", Subject: "external", Email: "external@example.com", EmailVerified: true, Name: "External reader"})
	if err != nil {
		t.Fatal(err)
	}
	reader := auth.Actor{ID: member.IdentityID, OrganizationID: a.OrganizationID, Kind: "human"}
	request, err := s.RequestTrustAccess(ctx, reader, "test-service", release.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RestrictedRelease(ctx, reader, "test-service", release.ID); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("unapproved access: %v", err)
	}
	if err = s.DecideTrustAccess(ctx, a, service, request.ID, "reader", time.Now().Add(time.Hour), true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RestrictedRelease(ctx, reader, "test-service", release.ID); err != nil {
		t.Fatal(err)
	}
	if err = auth.Authorize(ctx, s.DB, reader, service, "read"); err == nil {
		t.Fatal("external grant escalated workspace membership")
	}
	if _, err = s.RestrictedRelease(ctx, reader, "test-service", "other-release"); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("cross-release access: %v", err)
	}
	grants, err := s.TrustGrants(ctx, a, service, 10, 0)
	if err != nil || len(grants) != 1 {
		t.Fatalf("grants: %v", err)
	}
	if err = s.RevokeTrustGrant(ctx, a, service, grants[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RestrictedRelease(ctx, reader, "test-service", release.ID); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("revoked grant: %v", err)
	}
	transition.ExpectedRevision = 3
	transition.Action = "withdraw"
	transition.IdempotencyKey = "withdraw"
	if _, err = s.TransitionRelease(ctx, a, transition); err != nil {
		t.Fatal(err)
	}
	public, err = s.PublicReleaseIndex(ctx, "test-service", 10, 0)
	if err != nil || len(public) != 1 || public[0].State != "withdrawn" || public[0].Public.ServiceName != "" {
		t.Fatalf("withdrawal tombstone: %+v %v", public, err)
	}
	large := release.Manifest
	large.Evidence = []ReleaseEvidence{}
	generated := generatedReleaseArtifacts{}
	buffer := make([]byte, storage.MaxArtifactBytes)
	for i := 1; i <= 17; i++ {
		buffer[0] = byte(i)
		hash := storage.Hash(buffer)
		generated[hash] = byte(i)
		large.Evidence = append(large.Evidence, ReleaseEvidence{Reference: Ref{ID: fmt.Sprintf("large-%d", i), Revision: 1}, Hash: hash, Path: "evidence/" + hash, Metadata: Evidence{ArtifactKey: hash}})
	}
	s.Artifacts = generated
	if err = s.WriteReleaseArchive(ctx, io.Discard, large); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized archive accepted: %v", err)
	}
	s.Artifacts = originalArtifacts
	if err = s.WriteReleaseArchive(ctx, io.Discard, release.Manifest); err != nil {
		t.Fatalf("archive failure prevented recovery: %v", err)
	}
}

type generatedReleaseArtifacts map[string]byte

func (s generatedReleaseArtifacts) Get(_ context.Context, key string) ([]byte, error) {
	content := make([]byte, storage.MaxArtifactBytes)
	content[0] = s[key]
	return content, nil
}
func (s generatedReleaseArtifacts) Put(context.Context, []byte) (string, error) {
	return "", ErrInvalid
}

type corruptReleaseArtifacts struct{ storage.Artifacts }

func (s corruptReleaseArtifacts) Get(context.Context, string) ([]byte, error) {
	return []byte("corrupt bytes"), nil
}

type gatedReleaseArtifacts struct {
	storage.Artifacts
	once             sync.Once
	entered, proceed chan struct{}
}

func (s *gatedReleaseArtifacts) Get(ctx context.Context, key string) ([]byte, error) {
	s.once.Do(func() {
		close(s.entered)
		select {
		case <-ctx.Done():
		case <-s.proceed:
		}
	})
	return s.Artifacts.Get(ctx, key)
}

func TestReleaseComparisonSeparatesFreshnessFromImplementation(t *testing.T) {
	old := ReleaseManifest{Requirements: []ReleaseRequirement{{Implementation: &ImplementationContext{Assessment: &ImplementationAssessment{Version: ImplementationAssessmentVersion}, CurrentFreshness: "current"}}}}
	next := old
	next.Requirements = append([]ReleaseRequirement{}, old.Requirements...)
	copy := *old.Requirements[0].Implementation
	copy.CurrentFreshness = "stale"
	next.Requirements[0].Implementation = &copy
	changes := CompareAssessmentReleases(old, next)
	if len(changes) != 1 || changes[0].Category != "evidence_freshness" {
		t.Fatalf("freshness conflated with implementation: %+v", changes)
	}
}

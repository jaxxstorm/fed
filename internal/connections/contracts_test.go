package connections

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRenewalAndRetirementFenceLateWork(t *testing.T) {
	c := Connection{Lifecycle: Active, Revision: 4, Generation: 8, CredentialReference: "private-reference"}
	now := time.Now()
	renewed, err := c.Transition("renew", 4, now)
	if err != nil || renewed.Revision != 4 || renewed.Generation != 9 {
		t.Fatalf("renewal: %+v %v", renewed, err)
	}
	if !errors.Is(renewed.CheckGeneration(8), ErrConflict) {
		t.Fatal("old renewal accepted")
	}
	deleted, err := renewed.Transition("delete", 4, now)
	if err != nil || deleted.Revision != 5 || deleted.CredentialReference != "" {
		t.Fatalf("deletion: %+v %v", deleted, err)
	}
	if !errors.Is(deleted.CheckGeneration(10), ErrInactive) {
		t.Fatal("deleted generation accepted")
	}
	if _, err = deleted.Transition("configure", 5, now); !errors.Is(err, ErrInactive) {
		t.Fatal("configure restored deleted connection")
	}
	if _, err = deleted.Transition("restore", 4, now); !errors.Is(err, ErrConflict) {
		t.Fatal("stale restore accepted")
	}
	restored, err := deleted.Transition("restore", 5, now)
	if err != nil || restored.Generation != 11 || restored.CredentialReference != "" {
		t.Fatalf("restore: %+v %v", restored, err)
	}
	b, err := json.Marshal(c)
	if err != nil || strings.Contains(string(b), "private-reference") {
		t.Fatal("credential reference exposed")
	}
}

func TestHealthAndDiscoveryDoNotInferCoverage(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)
	h := AccessHealth{State: Verified, Generation: 2, CheckedAt: &now, ValidUntil: &later}
	if h.Stale(now, 2) || !h.Stale(later, 2) || !h.Stale(now, 3) {
		t.Fatal("incorrect health expiry")
	}
	for _, coverage := range []KindCoverage{{Complete: true}, {Authoritative: true}, {Complete: true, Authoritative: true, ErrorCode: "denied"}, {Complete: true, Authoritative: true, Limitations: []string{"namespace restricted"}}} {
		if coverage.CanMarkAbsent() {
			t.Fatal("partial scan can remove resources")
		}
	}
	if !(KindCoverage{Complete: true, Authoritative: true}).CanMarkAbsent() {
		t.Fatal("complete scan cannot reconcile")
	}
}

package auth

import "testing"

func TestAdmissionPolicy(t *testing.T) {
	for _, raw := range []string{"User@EXAMPLE.COM", " user@example.com "} {
		email, err := NormalizeEmail(raw)
		if err != nil || email != "user@example.com" {
			t.Fatal(email, err)
		}
	}
	for _, raw := range []string{"Name <user@example.com>", "user@*.example.com", "user@example.com.", "user@exam\nple.com", "user@-bad.com"} {
		if _, err := NormalizeEmail(raw); err == nil {
			t.Fatal("invalid email accepted", raw)
		}
	}
	for _, email := range []string{"user@badexample.com", "user@sub.example.com", "user@example.com.evil"} {
		if EmailAllowed(email, true, []string{"example.com"}) {
			t.Fatal("domain bypass", email)
		}
	}
	if EmailAllowed("user@example.com", false, nil) {
		t.Fatal("unverified accepted")
	}
	if !EmailAllowed("user@example.com", true, nil) {
		t.Fatal("empty policy")
	}
	c := AdmissionConfig{OrganizationClaim: "tid", OrganizationValue: "tenant"}
	if c.ClaimsAllowed(map[string]any{"tid": []string{"tenant"}}) || c.ClaimsAllowed(nil) || !c.ClaimsAllowed(map[string]any{"tid": "tenant"}) {
		t.Fatal("tenant claim matching")
	}
	if (AdmissionConfig{AdminEmail: "a@example.com", AdminSubject: "subject"}).Validate() == nil {
		t.Fatal("ambiguous bootstrap")
	}
	if (AdmissionConfig{OrganizationClaim: "tid"}).Validate() == nil {
		t.Fatal("incomplete tenant settings")
	}
}

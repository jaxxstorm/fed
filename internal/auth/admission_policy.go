package auth

import (
	"context"
	"errors"
	"net/mail"
	"slices"
	"strings"

	"golang.org/x/net/idna"
)

// Bootstrap readiness is database-dependent: established installations no
// longer need to retain a bootstrap principal in deployment configuration.
func CheckBootstrap(ctx context.Context, q Querier, c AdmissionConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	var complete bool
	if err := q.QueryRow(ctx, `SELECT bootstrap_complete FROM organization_settings WHERE singleton`).Scan(&complete); err != nil {
		return err
	}
	if !complete && c.AdminEmail == "" && c.AdminSubject == "" {
		return errors.New("KSI_ADMIN_EMAIL or KSI_ADMIN_SUBJECT is required for initial bootstrap")
	}
	return nil
}

var ErrMembershipPending = errors.New("membership_pending")
var ErrMembershipConflict = errors.New("membership_conflict")
var ErrMembershipInvalid = errors.New("invalid_membership")
var ErrLastAdmin = errors.New("last_administrator")

// NormalizeEmail defines the membership comparison key. It deliberately does
// not apply provider-specific dot or plus-address rewriting.
func NormalizeEmail(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 254 || strings.ContainsAny(raw, "\r\n") {
		return "", ErrMembershipInvalid
	}
	at := strings.LastIndexByte(raw, '@')
	if at <= 0 {
		return "", ErrMembershipInvalid
	}
	domain, err := NormalizeDomain(raw[at+1:])
	if err != nil {
		return "", err
	}
	email := strings.ToLower(raw[:at]) + "@" + domain
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || address.Name != "" || strings.ContainsAny(raw[:at], "<>\" ") {
		return "", ErrMembershipInvalid
	}
	return email, nil
}

func NormalizeDomain(raw string) (string, error) {
	domain, err := idna.Lookup.ToASCII(strings.ToLower(strings.TrimSpace(raw)))
	if err != nil || domain == "" || len(domain) > 253 || strings.ContainsAny(domain, "/@:* ") || strings.HasSuffix(domain, ".") {
		return "", ErrMembershipInvalid
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrMembershipInvalid
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", ErrMembershipInvalid
			}
		}
	}
	return domain, nil
}

func EmailAllowed(email string, verified bool, domains []string) bool {
	if !verified {
		return false
	}
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return false
	}
	return len(domains) == 0 || slices.Contains(domains, normalized[strings.LastIndexByte(normalized, '@')+1:])
}

type AdmissionConfig struct {
	AdminEmail        string
	AdminSubject      string
	OrganizationClaim string
	OrganizationValue string
}

func (c AdmissionConfig) Validate() error {
	if c.AdminEmail != "" && c.AdminSubject != "" {
		return errors.New("configure only one of KSI_ADMIN_EMAIL and KSI_ADMIN_SUBJECT")
	}
	if c.AdminEmail != "" {
		if _, err := NormalizeEmail(c.AdminEmail); err != nil {
			return errors.New("invalid KSI_ADMIN_EMAIL")
		}
	}
	if (c.OrganizationClaim == "") != (c.OrganizationValue == "") {
		return errors.New("KSI_OIDC_ORG_CLAIM and KSI_OIDC_ORG_VALUE must be configured together")
	}
	return nil
}

func (c AdmissionConfig) ClaimsAllowed(claims map[string]any) bool {
	if c.OrganizationClaim == "" {
		return true
	}
	value, ok := claims[c.OrganizationClaim].(string)
	return ok && value == c.OrganizationValue
}

// Package config validates startup settings before any listener is opened.
package config

import (
	"errors"
	"fmt"
	"github.com/jaxxstorm/fed/internal/auth"
	"net"
	"net/url"
	"os"
	"strconv"
)

type Config struct {
	Mode                      string
	Listen                    string
	DatabaseURL               string
	DevIdentity               bool
	ConnectionsEnabled        bool
	GuidanceEnabled           bool
	GuidanceRulesDirectory    string
	GuidanceMarkdownDirectory string
	OIDCIssuer                string
	OIDCClientID              string
	OIDCClientSecret          string
	PublicURL                 string
	AdminSubject              string
	AdminEmail                string
	OIDCOrgClaim              string
	OIDCOrgValue              string
	ArtifactDirectory         string
	ArtifactBucket            string
	ArtifactEndpoint          string
}

func Load() (Config, error) { return FromEnv(os.Getenv) }

func FromEnv(get func(string) string) (Config, error) {
	value := func(key, fallback string) string {
		if v := get(key); v != "" {
			return v
		}
		return fallback
	}
	dev, err := strconv.ParseBool(value("KSI_DEV_IDENTITY", "false"))
	if err != nil {
		return Config{}, errors.New("KSI_DEV_IDENTITY must be a boolean")
	}
	connectionManagement, err := strconv.ParseBool(value("KSI_CONNECTIONS_ENABLED", "false"))
	if err != nil {
		return Config{}, errors.New("KSI_CONNECTIONS_ENABLED must be a boolean")
	}
	guidanceEnabled, err := strconv.ParseBool(value("KSI_GUIDANCE_ENABLED", "false"))
	if err != nil {
		return Config{}, errors.New("KSI_GUIDANCE_ENABLED must be a boolean")
	}
	c := Config{
		Mode: value("KSI_MODE", "server"), Listen: value("KSI_LISTEN", "127.0.0.1:8080"),
		DatabaseURL: get("DATABASE_URL"), DevIdentity: dev,
		ConnectionsEnabled:        connectionManagement,
		GuidanceEnabled:           guidanceEnabled,
		GuidanceRulesDirectory:    value("KSI_GUIDANCE_RULES_DIRECTORY", "/app/catalog"),
		GuidanceMarkdownDirectory: value("KSI_GUIDANCE_MARKDOWN_DIRECTORY", "/app/2026-markdown"),
		OIDCIssuer:                get("KSI_OIDC_ISSUER"), OIDCClientID: get("KSI_OIDC_CLIENT_ID"),
		OIDCClientSecret: get("KSI_OIDC_CLIENT_SECRET"), PublicURL: get("KSI_PUBLIC_URL"),
		AdminSubject: get("KSI_ADMIN_SUBJECT"), ArtifactDirectory: value("KSI_ARTIFACT_DIRECTORY", ".data/artifacts"),
		AdminEmail: get("KSI_ADMIN_EMAIL"), OIDCOrgClaim: get("KSI_OIDC_ORG_CLAIM"), OIDCOrgValue: get("KSI_OIDC_ORG_VALUE"),
		ArtifactBucket: get("KSI_ARTIFACT_BUCKET"), ArtifactEndpoint: get("KSI_ARTIFACT_ENDPOINT"),
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if err := (auth.AdmissionConfig{AdminEmail: c.AdminEmail, AdminSubject: c.AdminSubject, OrganizationClaim: c.OIDCOrgClaim, OrganizationValue: c.OIDCOrgValue}).Validate(); err != nil {
		return err
	}
	if c.Mode != "server" && c.Mode != "worker" && c.Mode != "all" && c.Mode != "migrate" {
		return errors.New("KSI_MODE must be server, worker, all, or migrate")
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return errors.New("DATABASE_URL must be a PostgreSQL connection URL")
	}
	host, _, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return errors.New("KSI_LISTEN must be a host:port address")
	}
	if c.DevIdentity {
		// A literal loopback address avoids DNS-dependent bypasses.
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return errors.New("development identity requires a literal loopback listen address")
		}
	} else if c.Mode == "server" || c.Mode == "all" {
		for _, pair := range [][2]string{{"KSI_OIDC_ISSUER", c.OIDCIssuer}, {"KSI_OIDC_CLIENT_ID", c.OIDCClientID}, {"KSI_OIDC_CLIENT_SECRET", c.OIDCClientSecret}, {"KSI_PUBLIC_URL", c.PublicURL}} {
			if pair[1] == "" {
				return fmt.Errorf("%s is required for authenticated startup", pair[0])
			}
		}
		for _, raw := range []string{c.PublicURL, c.OIDCIssuer} {
			u, err := url.Parse(raw)
			if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
				return errors.New("production public URL and OIDC issuer must be HTTPS URLs without credentials, query, or fragment")
			}
		}
	}
	if c.ArtifactEndpoint != "" && c.ArtifactBucket == "" {
		return errors.New("artifact endpoint requires an artifact bucket")
	}
	return nil
}

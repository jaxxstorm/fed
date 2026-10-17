package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/jaxxstorm/fed/internal/connections"
)

func TestDelegatedAWSPrincipalPathAndNoFallback(t *testing.T) {
	deny := false
	assumed := 0
	reads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/clusters" {
			reads++
			if !strings.Contains(r.Header.Get("Authorization"), "ASSUMED") {
				t.Error("ambient resource read")
			}
			fmt.Fprint(w, `{"clusters":[]}`)
			return
		}
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "text/xml")
		switch r.Form.Get("Action") {
		case "GetCallerIdentity":
			if strings.Contains(r.Header.Get("Authorization"), "BASE") {
				fmt.Fprint(w, `<GetCallerIdentityResponse><GetCallerIdentityResult><Account>999999999999</Account><Arn>arn:aws:sts::999999999999:assumed-role/Fed/task</Arn><UserId>ROLEID:task</UserId></GetCallerIdentityResult></GetCallerIdentityResponse>`)
			} else {
				fmt.Fprint(w, `<GetCallerIdentityResponse><GetCallerIdentityResult><Account>123456789012</Account><Arn>arn:aws:sts::123456789012:assumed-role/Customer/fed</Arn><UserId>TARGET:fed</UserId></GetCallerIdentityResult></GetCallerIdentityResponse>`)
			}
		case "GetRole":
			if r.Form.Get("RoleName") != "Fed" {
				t.Error("role path passed as role name")
			}
			fmt.Fprint(w, `<GetRoleResponse><GetRoleResult><Role><Arn>arn:aws:iam::999999999999:role/application/Fed</Arn><RoleId>ROLEID</RoleId><RoleName>Fed</RoleName><Path>/application/</Path></Role></GetRoleResult></GetRoleResponse>`)
		case "AssumeRole":
			assumed++
			if r.Form.Get("ExternalId") != "external" {
				t.Error("external ID missing")
			}
			if deny {
				w.WriteHeader(403)
				fmt.Fprint(w, `<ErrorResponse><Error><Code>AccessDenied</Code><Message>Denied</Message></Error></ErrorResponse>`)
				return
			}
			fmt.Fprint(w, `<AssumeRoleResponse><AssumeRoleResult><Credentials><AccessKeyId>ASSUMED</AccessKeyId><SecretAccessKey>secret</SecretAccessKey><SessionToken>session</SessionToken><Expiration>2099-01-01T00:00:00Z</Expiration></Credentials><AssumedRoleUser><Arn>arn:aws:sts::123456789012:assumed-role/Customer/fed</Arn><AssumedRoleId>TARGET:fed</AssumedRoleId></AssumedRoleUser></AssumeRoleResult></AssumeRoleResponse>`)
		default:
			t.Errorf("unexpected AWS operation: %s", r.Form.Get("Action"))
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	p := AWS{FedRoleARN: "arn:aws:iam::999999999999:role/application/Fed", sessions: &awsRoleSessions{}, Load: func(context.Context, ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
		return aws.Config{Region: "us-east-1", BaseEndpoint: aws.String(server.URL), HTTPClient: server.Client(), Credentials: credentials.NewStaticCredentialsProvider("BASE", "secret", "")}, nil
	}}
	if principal, err := p.SetupPrincipal(context.Background()); err != nil || principal != p.FedRoleARN {
		t.Fatal("principal", principal, err)
	}
	c := connections.Connection{Provider: connections.AWS, Strategy: connections.DelegatedRole, Configuration: connections.Configuration{AccountID: "123456789012", RoleARN: "arn:aws:iam::123456789012:role/Customer", Region: "us-east-1"}}
	k := connections.NewCredentials(map[string]string{"external_id": "external"})
	if _, err := p.Discover(context.Background(), c, k, "", 2); err != nil || reads != 1 || assumed != 1 {
		t.Fatal("delegated read", err, reads, assumed)
	}
	if _, err := p.Discover(context.Background(), c, k, "", 2); err != nil || assumed != 1 {
		t.Fatal("scoped session cache missed", err)
	}
	c.Generation++
	if _, err := p.Discover(context.Background(), c, k, "", 2); err != nil || assumed != 2 {
		t.Fatal("generation reused old session", err)
	}
	c.ServiceID = "other-service"
	if _, err := p.Discover(context.Background(), c, k, "", 2); err != nil || assumed != 3 {
		t.Fatal("service reused another session", err)
	}
	for _, entry := range p.sessions.entries {
		entry.credentials.Invalidate()
	}
	previousReads := reads
	deny = true
	if _, err := p.Discover(context.Background(), c, k, "", 2); err == nil || reads != previousReads {
		t.Fatal("fallback after denied trust", err)
	}
	c.Configuration.RoleARN = ""
	if _, err := p.Configuration(context.Background(), c, k); err == nil {
		t.Fatal("missing role accepted")
	}
	p.FedRoleARN = "arn:aws:iam::999999999999:role/wrong-path/Fed"
	if _, err := p.SetupPrincipal(context.Background()); err == nil {
		t.Fatal("wrong IAM path accepted")
	}
}

func TestAWSSetupMaterialScriptAndPolicies(t *testing.T) {
	p := AWS{FedRoleARN: "arn:aws:iam::999999999999:role/application/Fed"}
	external := strings.Repeat("a", 64)
	a, err := p.SetupMaterial("123456789012", external, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.SetupMaterial("123456789012", external, "")
	if err != nil || a.CLI != b.CLI || a.RoleARN != b.RoleARN {
		t.Fatal("unstable setup", err)
	}
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(a.CLI)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatal("script syntax", err, string(output))
	}
	if !strings.Contains(string(a.TrustPolicy), p.FedRoleARN) || !strings.Contains(string(a.TrustPolicy), external) || !strings.Contains(a.CLI, "NoSuchEntity") || !strings.Contains(a.CLI, "Existing role has different trust") {
		t.Fatal("missing trust/conflict guard")
	}
	if _, err = p.SetupMaterial("123456789012", external+"\nEOF", ""); err == nil {
		t.Fatal("script input injection accepted")
	}
}

func TestAWSSetupScriptRejectsWrongAccountAndConflictingRole(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required to execute the administrator script fixture")
	}
	p := AWS{FedRoleARN: "arn:aws:iam::999999999999:role/Fed"}
	material, err := p.SetupMaterial("123456789012", strings.Repeat("a", 64), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"wrong-account", "conflicting-trust", "matching"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			awsScript := `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1 $2" == 'sts get-caller-identity' ]]; then echo "$TEST_ACCOUNT"; exit 0; fi
if [[ "$1 $2" == 'iam get-role' ]]; then cat "$TEST_DIRECTORY/role.json"; exit 0; fi
if [[ "$1 $2" == 'iam get-role-policy' ]]; then cat "$TEST_DIRECTORY/policy.json"; exit 0; fi
echo unexpected-write >> "$TEST_DIRECTORY/writes"
exit 99
`
			if err = os.WriteFile(filepath.Join(dir, "aws"), []byte(awsScript), 0700); err != nil {
				t.Fatal(err)
			}
			trust := material.TrustPolicy
			if mode == "conflicting-trust" {
				trust = json.RawMessage(`{"Statement":[]}`)
			}
			role, _ := json.Marshal(map[string]any{"Role": map[string]any{"Arn": material.RoleARN, "AssumeRolePolicyDocument": trust}})
			policy, _ := json.Marshal(map[string]any{"PolicyDocument": material.ReadPolicy})
			if err = os.WriteFile(filepath.Join(dir, "role.json"), role, 0600); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, "policy.json"), policy, 0600); err != nil {
				t.Fatal(err)
			}
			account := "123456789012"
			if mode == "wrong-account" {
				account = "999999999999"
			}
			cmd := exec.Command("bash")
			cmd.Stdin = strings.NewReader(material.CLI)
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "TEST_DIRECTORY="+dir, "TEST_ACCOUNT="+account)
			output, err := cmd.CombinedOutput()
			if (mode == "matching") != (err == nil) {
				t.Fatal("unexpected script result", err, string(output))
			}
			if _, err = os.Stat(filepath.Join(dir, "writes")); !os.IsNotExist(err) {
				t.Fatal("script attempted an IAM write", string(output))
			}
		})
	}
}

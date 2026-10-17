package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/jaxxstorm/fed/internal/connections"
)

func TestAWSIdentityAssumeRoleAndBoundedDiscovery(t *testing.T) {
	assumed := false
	lists := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/" {
			if err := r.ParseForm(); err != nil {
				t.Error(err)
				return
			}
			w.Header().Set("Content-Type", "text/xml")
			switch r.Form.Get("Action") {
			case "AssumeRole":
				if r.Form.Get("RoleArn") != "arn:aws:iam::123456789012:role/evidence" || r.Form.Get("ExternalId") != "external-secret" {
					t.Error("incorrect role binding")
				}
				assumed = true
				fmt.Fprint(w, `<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><AssumeRoleResult><Credentials><AccessKeyId>ASSUMED</AccessKeyId><SecretAccessKey>secret</SecretAccessKey><SessionToken>session</SessionToken><Expiration>2099-01-01T00:00:00Z</Expiration></Credentials><AssumedRoleUser><Arn>arn:aws:sts::123456789012:assumed-role/evidence/fed</Arn><AssumedRoleId>id</AssumedRoleId></AssumedRoleUser></AssumeRoleResult></AssumeRoleResponse>`)
			case "GetCallerIdentity":
				if strings.Contains(r.Header.Get("Authorization"), "BASE") {
					fmt.Fprint(w, `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Account>999999999999</Account><Arn>arn:aws:iam::999999999999:role/server</Arn><UserId>base</UserId></GetCallerIdentityResult></GetCallerIdentityResponse>`)
					return
				}
				fmt.Fprint(w, `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Account>123456789012</Account><Arn>arn:aws:sts::123456789012:assumed-role/evidence/fed</Arn><UserId>id</UserId></GetCallerIdentityResult></GetCallerIdentityResponse>`)
			default:
				t.Errorf("unexpected write/API action: %s", r.Form.Get("Action"))
				w.WriteHeader(400)
			}
			return
		}
		if r.Method != "GET" || r.URL.Path != "/clusters" {
			t.Errorf("unexpected infrastructure operation: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(400)
			return
		}
		lists++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("maxResults") != "1" {
			t.Error("unbounded EKS page")
		}
		if r.URL.Query().Get("nextToken") == "next" {
			fmt.Fprint(w, `{"clusters":["second"]}`)
		} else {
			fmt.Fprint(w, `{"clusters":["first"],"nextToken":"next"}`)
		}
	}))
	defer server.Close()
	p := AWS{Load: func(ctx context.Context, options ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
		var settings awsconfig.LoadOptions
		for _, option := range options {
			if err := option(&settings); err != nil {
				t.Fatal(err)
			}
		}
		if settings.Region != "us-east-1" || settings.SharedConfigProfile != "server-profile" {
			t.Fatal("connection profile/region not applied")
		}
		return aws.Config{Region: "us-east-1", BaseEndpoint: aws.String(server.URL), HTTPClient: server.Client(), Credentials: credentials.NewStaticCredentialsProvider("BASE", "secret", "")}, nil
	}}
	c := connections.Connection{Provider: connections.AWS, Configuration: connections.Configuration{AccountID: "123456789012", Profile: "server-profile", RoleARN: "arn:aws:iam::123456789012:role/evidence", Region: "us-east-1"}}
	secret := connections.NewCredentials(map[string]string{"external_id": "external-secret"})
	first, err := p.Discover(context.Background(), c, secret, "", 2)
	if err != nil || !assumed || len(first.Resources) != 2 || first.NextCursor == "" || lists != 1 {
		t.Fatalf("first: %+v %v", first, err)
	}
	second, err := p.Discover(context.Background(), c, secret, first.NextCursor, 1)
	if err != nil || len(second.Resources) != 1 || second.NextCursor != "" || second.Resources[0].Name != "second" {
		t.Fatalf("second: %+v %v", second, err)
	}
	c.Configuration.Region = "us-west-2"
	if _, err = p.Configuration(context.Background(), c, secret); err == nil {
		t.Fatal("unsupported region accepted")
	}
	c.Configuration.Region = "us-east-1"
	c.Configuration.AccountID = "999999999999"
	c.Configuration.RoleARN = ""
	p.Load = func(context.Context, ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
		return aws.Config{Region: "us-east-1", BaseEndpoint: aws.String(server.URL), HTTPClient: server.Client(), Credentials: credentials.NewStaticCredentialsProvider("ASSUMED", "secret", "")}, nil
	}
	if _, err = p.Configuration(context.Background(), c, secret); err == nil || err.Error() != "wrong_tenant" {
		t.Fatal("wrong AWS account accepted")
	}
}

func TestAWSProfileAndRoleFailureRecovery(t *testing.T) {
	c := connections.Connection{Provider: connections.AWS, Configuration: connections.Configuration{AccountID: "123456789012", Region: "us-east-1", Profile: "expired-profile"}}
	p := AWS{Load: func(context.Context, ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
		return aws.Config{}, errors.New("expired SSO credentials")
	}}
	health, err := p.Test(context.Background(), c, connections.Credentials{})
	if err != nil || health.State != connections.Expired || health.ErrorCode != "profile_sign_in_required" || !strings.Contains(strings.Join(health.Limitations, " "), "expired-profile") {
		t.Fatalf("profile recovery: %+v %v", health, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "text/xml")
		if r.Form.Get("Action") == "AssumeRole" {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `<ErrorResponse><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>Denied</Message></Error><RequestId>test</RequestId></ErrorResponse>`)
			return
		}
		fmt.Fprint(w, `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Account>999999999999</Account><Arn>arn:aws:iam::999999999999:role/server</Arn><UserId>base</UserId></GetCallerIdentityResult></GetCallerIdentityResponse>`)
	}))
	defer server.Close()
	p.Load = func(context.Context, ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
		return aws.Config{Region: "us-east-1", BaseEndpoint: aws.String(server.URL), HTTPClient: server.Client(), Credentials: credentials.NewStaticCredentialsProvider("BASE", "secret", "")}, nil
	}
	c.Configuration.RoleARN = "arn:aws:iam::123456789012:role/evidence"
	health, err = p.Test(context.Background(), c, connections.Credentials{})
	if err != nil || health.ErrorCode != "role_trust_denied" || health.Principal != "arn:aws:iam::999999999999:role/server" {
		t.Fatalf("role recovery: %+v %v", health, err)
	}
}

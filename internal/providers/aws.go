package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
	"github.com/jaxxstorm/fed/internal/connections"
)

type ProviderError string

func (e ProviderError) Error() string        { return string(e) }
func (e ProviderError) RecoveryCode() string { return string(e) }

// AWS resolves each connection independently. Load is injectable for controlled
// SDK endpoint tests; production uses the ordinary server SDK credential chain.
type AWS struct {
	Load func(context.Context, ...func(*awsconfig.LoadOptions) error) (aws.Config, error)
}

func (p AWS) Configuration(ctx context.Context, c connections.Connection, credentials connections.Credentials) (aws.Config, error) {
	cfg, _, err := p.configuration(ctx, c, credentials)
	return cfg, err
}

func (p AWS) configuration(ctx context.Context, c connections.Connection, credentials connections.Credentials) (aws.Config, string, error) {
	if c.Provider != connections.AWS || c.Configuration.Region != "us-east-1" || len(c.Configuration.AccountID) != 12 {
		return aws.Config{}, "", ProviderError("wrong_tenant")
	}
	if _, err := strconv.ParseUint(c.Configuration.AccountID, 10, 64); err != nil {
		return aws.Config{}, "", ProviderError("wrong_tenant")
	}
	load := p.Load
	if load == nil {
		load = awsconfig.LoadDefaultConfig
	}
	options := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion("us-east-1"), awsconfig.WithRetryMaxAttempts(2), awsconfig.WithHTTPClient(&http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }})}
	if c.Configuration.Profile != "" {
		options = append(options, awsconfig.WithSharedConfigProfile(c.Configuration.Profile))
	}
	cfg, err := load(ctx, options...)
	if err != nil {
		return cfg, "", ProviderError("profile_sign_in_required")
	}
	cfg.Region = "us-east-1"
	identity, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return cfg, "", ProviderError("profile_sign_in_required")
	}
	principal := aws.ToString(identity.Arn)
	if c.Configuration.RoleARN != "" {
		cfg.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), c.Configuration.RoleARN, func(options *stscreds.AssumeRoleOptions) {
			if external := credentials.Value("external_id"); external != "" {
				options.ExternalID = aws.String(external)
			}
			options.RoleSessionName = "fed-evidence"
		}))
		identity, err = sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return cfg, principal, ProviderError("role_trust_denied")
		}
	}
	if aws.ToString(identity.Account) != c.Configuration.AccountID {
		return cfg, principal, ProviderError("wrong_tenant")
	}
	return cfg, principal, nil
}

func (p AWS) Test(ctx context.Context, c connections.Connection, credentials connections.Credentials) (connections.AccessHealth, error) {
	_, principal, err := p.configuration(ctx, c, credentials)
	if err != nil {
		state := connections.Denied
		if err.Error() == "profile_sign_in_required" {
			state = connections.Expired
		}
		limitations := []string{}
		if c.Configuration.Profile != "" {
			limitations = append(limitations, "Server profile: "+c.Configuration.Profile+". Renew SSO with aws sso login --profile "+c.Configuration.Profile+" in the server environment.")
		}
		return connections.AccessHealth{State: state, ErrorCode: err.Error(), Limitations: limitations, Principal: principal}, nil
	}
	return connections.AccessHealth{State: connections.Verified, VerifiedAccountID: c.Configuration.AccountID, Principal: principal, Limitations: []string{"Identity verification does not establish permissions for every resource or collector."}}, nil
}

type awsCursor struct {
	Phase string `json:"phase"`
	Token string `json:"token,omitempty"`
}

func (p AWS) Discover(ctx context.Context, c connections.Connection, credentials connections.Credentials, cursor string, requested int) (connections.DiscoveryPage, error) {
	limit, err := connections.PageSize(requested)
	if err != nil {
		return connections.DiscoveryPage{}, err
	}
	position := awsCursor{Phase: "account"}
	if cursor != "" && (json.Unmarshal([]byte(cursor), &position) != nil || position.Phase != "eks") {
		return connections.DiscoveryPage{}, connections.ErrInvalid
	}
	cfg, err := p.Configuration(ctx, c, credentials)
	if err != nil {
		return connections.DiscoveryPage{}, err
	}
	out := connections.DiscoveryPage{Resources: []connections.DiscoveredResource{}, Coverage: []connections.KindCoverage{{Kind: "account_region", Complete: true, Authoritative: true}}}
	locator := c.Configuration.AccountID + "/us-east-1"
	if position.Phase == "account" {
		out.Resources = append(out.Resources, connections.DiscoveredResource{Resource: connections.ResourceKey{Provider: connections.AWS, Tenant: c.Tenant(), Kind: "account_region", ID: "us-east-1"}, Name: locator, Locator: locator})
		limit--
		position.Phase = "eks"
	}
	if limit == 0 {
		b, _ := json.Marshal(position)
		out.NextCursor = string(b)
		return out, nil
	}
	input := &eks.ListClustersInput{MaxResults: aws.Int32(int32(limit))}
	if position.Token != "" {
		input.NextToken = aws.String(position.Token)
	}
	page, err := eks.NewFromConfig(cfg).ListClusters(ctx, input)
	if err != nil {
		code := "provider_error"
		var api smithy.APIError
		if errors.As(err, &api) && (api.ErrorCode() == "AccessDeniedException" || api.ErrorCode() == "UnauthorizedException") {
			code = "insufficient_permissions"
		}
		if errors.As(err, &api) && (api.ErrorCode() == "ThrottlingException" || api.ErrorCode() == "TooManyRequestsException") {
			code = "rate_limited"
			b, _ := json.Marshal(position)
			out.NextCursor = string(b)
		}
		out.Coverage = append(out.Coverage, connections.KindCoverage{Kind: "eks_cluster", ErrorCode: code})
		return out, nil
	}
	for _, name := range page.Clusters {
		out.Resources = append(out.Resources, connections.DiscoveredResource{Resource: connections.ResourceKey{Provider: connections.AWS, Tenant: c.Tenant(), Kind: "eks_cluster", ID: "arn:aws:eks:us-east-1:" + c.Configuration.AccountID + ":cluster/" + name}, Name: name, Locator: locator})
	}
	position.Token = aws.ToString(page.NextToken)
	out.Coverage = append(out.Coverage, connections.KindCoverage{Kind: "eks_cluster", Complete: position.Token == "", Authoritative: true})
	if position.Token != "" {
		b, _ := json.Marshal(position)
		out.NextCursor = string(b)
	}
	return out, nil
}

func (p AWS) Renew(ctx context.Context, c connections.Connection, credentials connections.Credentials) (connections.Credentials, error) {
	_, err := p.Configuration(ctx, c, credentials)
	return credentials, err
}

func (AWS) Read(context.Context, connections.Connection, connections.Credentials, connections.DiscoveredResource) (connections.Snapshot, error) {
	return connections.Snapshot{}, ProviderError("use_registered_aws_collector")
}

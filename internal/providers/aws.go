package providers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
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
	FedRoleARN string
	sessions   *awsRoleSessions
	Load       func(context.Context, ...func(*awsconfig.LoadOptions) error) (aws.Config, error)
}

func (p AWS) Configuration(ctx context.Context, c connections.Connection, credentials connections.Credentials) (aws.Config, error) {
	cfg, _, err := p.configuration(ctx, c, credentials)
	return cfg, err
}

func (p AWS) configuration(ctx context.Context, c connections.Connection, credentials connections.Credentials) (aws.Config, string, error) {
	delegated := c.Strategy == connections.DelegatedRole
	if delegated && (c.Configuration.Profile != "" || !validRole(c.Configuration.RoleARN, c.Configuration.AccountID) || credentials.Value("external_id") == "") {
		return aws.Config{}, "", ProviderError("delegated_role_configuration_required")
	}
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
	if delegated {
		if err = p.verifyPrincipal(ctx, cfg, identity); err != nil {
			return cfg, principal, err
		}
	}
	if c.Configuration.RoleARN != "" {
		cfg.Credentials = p.roleCredentials(cfg, c, credentials, principal)
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

type awsRoleSessionKey struct {
	service, connection, role, principal string
	generation                           int64
	external                             [32]byte
}
type awsRoleSession struct {
	credentials *aws.CredentialsCache
	created     time.Time
}
type awsRoleSessions struct {
	mu      sync.Mutex
	entries map[awsRoleSessionKey]awsRoleSession
}

func (p AWS) roleCredentials(cfg aws.Config, c connections.Connection, credentials connections.Credentials, principal string) *aws.CredentialsCache {
	create := func() *aws.CredentialsCache {
		return aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), c.Configuration.RoleARN, func(options *stscreds.AssumeRoleOptions) {
			if external := credentials.Value("external_id"); external != "" {
				options.ExternalID = aws.String(external)
			}
			options.RoleSessionName = "fed-evidence"
		}))
	}
	if p.sessions == nil || c.Strategy != connections.DelegatedRole {
		return create()
	}
	key := awsRoleSessionKey{service: c.ServiceID, connection: c.ID, role: c.Configuration.RoleARN, principal: principal, generation: c.Generation, external: sha256.Sum256([]byte(credentials.Value("external_id")))}
	p.sessions.mu.Lock()
	defer p.sessions.mu.Unlock()
	if p.sessions.entries == nil {
		p.sessions.entries = map[awsRoleSessionKey]awsRoleSession{}
	}
	now := time.Now()
	for key, entry := range p.sessions.entries {
		if now.Sub(entry.created) > time.Hour {
			delete(p.sessions.entries, key)
		}
	}
	if cached, ok := p.sessions.entries[key]; ok {
		return cached.credentials
	}
	cache := create()
	// Limit retained provider instances as well as temporary AWS credentials.
	if len(p.sessions.entries) >= 1024 {
		for old := range p.sessions.entries {
			delete(p.sessions.entries, old)
			break
		}
	}
	p.sessions.entries[key] = awsRoleSession{credentials: cache, created: now}
	return cache
}

func validRole(role, account string) bool {
	prefix := "arn:aws:iam::" + account + ":role/"
	return regexp.MustCompile(`^[0-9]{12}$`).MatchString(account) && strings.HasPrefix(role, prefix) && regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/[A-Za-z0-9_+=,.@/-]+$`).MatchString(role) && len(role) <= 2048 && !strings.HasSuffix(role, "/")
}

// Resolve the configured IAM path via GetRole rather than deriving an IAM ARN
// from an STS session ARN (which omits the role path). RoleId also detects role
// deletion/recreation while old sessions are still alive.
func (p AWS) verifyPrincipal(ctx context.Context, cfg aws.Config, identity *sts.GetCallerIdentityOutput) error {
	account := aws.ToString(identity.Account)
	if !validRole(p.FedRoleARN, account) {
		return ProviderError("fed_runtime_role_mismatch")
	}
	name := p.FedRoleARN[strings.LastIndex(p.FedRoleARN, "/")+1:]
	role, err := iam.NewFromConfig(cfg).GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(name)})
	if err != nil || role.Role == nil {
		return ProviderError("fed_role_verification_denied")
	}
	if aws.ToString(role.Role.Arn) != p.FedRoleARN || aws.ToString(role.Role.RoleId) == "" {
		return ProviderError("fed_runtime_role_mismatch")
	}
	caller := aws.ToString(identity.Arn)
	if caller == p.FedRoleARN && aws.ToString(identity.UserId) == aws.ToString(role.Role.RoleId) {
		return nil
	}
	prefix := "arn:aws:sts::" + account + ":assumed-role/" + name + "/"
	if !strings.HasPrefix(caller, prefix) || len(caller) == len(prefix) || !strings.HasPrefix(aws.ToString(identity.UserId), aws.ToString(role.Role.RoleId)+":") {
		return ProviderError("fed_runtime_role_mismatch")
	}
	return nil
}

func (p AWS) SetupPrincipal(ctx context.Context) (string, error) {
	if p.FedRoleARN == "" {
		return "", ProviderError("missing_fed_role_configuration")
	}
	load := p.Load
	if load == nil {
		load = awsconfig.LoadDefaultConfig
	}
	cfg, err := load(ctx, awsconfig.WithRegion("us-east-1"), awsconfig.WithRetryMaxAttempts(2))
	if err != nil {
		return "", ProviderError("fed_credentials_unavailable")
	}
	identity, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", ProviderError("fed_credentials_unavailable")
	}
	if err = p.verifyPrincipal(ctx, cfg, identity); err != nil {
		return "", err
	}
	return p.FedRoleARN, nil
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

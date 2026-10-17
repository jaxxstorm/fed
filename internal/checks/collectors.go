package checks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/guardduty"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/jaxxstorm/fed/internal/assessment"
)

func Registry(githubToken string) map[string]Collector {
	return map[string]Collector{
		"kubernetes.network-isolation":       Kubernetes{},
		"aws.rds.configuration":              Infrastructure{Kind: "rds", Config: AccountConfig},
		"aws.elasticache.transit-encryption": Infrastructure{Kind: "elasticache", Config: AccountConfig},
		"aws.eks.endpoint-access":            Infrastructure{Kind: "endpoint", Config: AccountConfig},
		"aws.guardduty.configuration":        AWS{Kind: "guardduty", Config: AccountConfig},
		"aws.eks.control-plane-logging":      AWS{Kind: "eks", Config: AccountConfig},
		"github.pulumi.workflow-evidence":    GitHub{Token: githubToken, BaseURL: "https://api.github.com", Client: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}},
	}
}

func AccountConfig(ctx context.Context, a assessment.Account, region string) (aws.Config, error) {
	options := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region), awsconfig.WithRetryMaxAttempts(2)}
	// Account/profile bindings are operator configuration, never agent input.
	profiles := map[string]string{}
	if raw := os.Getenv("KSI_AWS_ACCOUNT_PROFILES"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &profiles); err != nil {
			return aws.Config{}, errors.New("invalid account profile configuration")
		}
	}
	if profile := profiles[a.ID]; profile != "" {
		options = append(options, awsconfig.WithSharedConfigProfile(profile))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return cfg, err
	}
	if a.RoleARN != "" {
		cfg.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), a.RoleARN))
	}
	id, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return cfg, err
	}
	if aws.ToString(id.Account) != a.ID {
		return cfg, errors.New("AWS caller account does not match configured scope")
	}
	return cfg, nil
}

type AWS struct {
	Kind   string
	Config func(context.Context, assessment.Account, string) (aws.Config, error)
}

func (c AWS) Collect(ctx context.Context, scope assessment.Scope, m assessment.Measure) (Result, error) {
	r := Result{Limitations: []string{"Configuration observations support only the configured customer measure; they do not establish complete KSI coverage."}}
	for _, account := range scope.Accounts {
		for _, region := range account.Regions {
			label := account.ID + "/" + region
			cfg, err := c.Config(ctx, account, region)
			if err != nil {
				r.Observations = append(r.Observations, Observation{Scope: label, Outcome: "error", Error: collectorErrorCode(err, "credentials_or_account_validation_failed")})
				continue
			}
			if c.Kind == "guardduty" {
				r.Observations = append(r.Observations, guardDuty(ctx, cfg, label, m.Expected)...)
			} else {
				r.Observations = append(r.Observations, eksLogging(ctx, cfg, label, account.Clusters, m.Expected)...)
			}
		}
	}
	r.Summarize()
	return r, nil
}

func guardDuty(ctx context.Context, cfg aws.Config, label string, expected []string) []Observation {
	client := guardduty.NewFromConfig(cfg)
	p := guardduty.NewListDetectorsPaginator(client, &guardduty.ListDetectorsInput{})
	out := []Observation{}
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return append(out, Observation{Scope: label, Outcome: "error", Error: "detector_enumeration_failed"})
		}
		for _, id := range page.DetectorIds {
			v, err := client.GetDetector(ctx, &guardduty.GetDetectorInput{DetectorId: aws.String(id)})
			if err != nil {
				out = append(out, Observation{Scope: label + "/" + id, Outcome: "error", Error: "detector_read_failed"})
				continue
			}
			features := map[string]string{"detector": string(v.Status)}
			for _, f := range v.Features {
				features[string(f.Name)] = string(f.Status)
			}
			status := "pass"
			if len(expected) == 0 {
				status = "unknown"
			}
			for _, feature := range expected {
				if features[feature] != "ENABLED" {
					status = "fail"
				}
			}
			out = append(out, Observation{Scope: label + "/" + id, Outcome: status, Data: features})
		}
	}
	if len(out) == 0 {
		out = append(out, Observation{Scope: label, Outcome: "unknown", Data: map[string]any{"detectors": 0}, Error: "no_detector_observed"})
	}
	return out
}

func eksLogging(ctx context.Context, cfg aws.Config, label string, clusters, expected []string) []Observation {
	client := eks.NewFromConfig(cfg)
	out := []Observation{}
	if len(clusters) == 0 {
		p := eks.NewListClustersPaginator(client, &eks.ListClustersInput{})
		for p.HasMorePages() {
			page, err := p.NextPage(ctx)
			if err != nil {
				out = append(out, Observation{Scope: label, Outcome: "error", Error: "cluster_enumeration_failed"})
				break
			}
			clusters = append(clusters, page.Clusters...)
		}
	}
	for _, name := range clusters {
		v, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(name)})
		if err != nil || v.Cluster == nil {
			out = append(out, Observation{Scope: label + "/" + name, Outcome: "error", Error: "cluster_read_failed"})
			continue
		}
		enabled := []string{}
		if v.Cluster.Logging != nil {
			for _, entry := range v.Cluster.Logging.ClusterLogging {
				if aws.ToBool(entry.Enabled) {
					for _, kind := range entry.Types {
						enabled = append(enabled, string(kind))
					}
				}
			}
		}
		status := "pass"
		if len(expected) == 0 {
			status = "unknown"
		}
		for _, kind := range expected {
			if !slices.Contains(enabled, kind) {
				status = "fail"
			}
		}
		out = append(out, Observation{Scope: label + "/" + name, Outcome: status, Data: map[string]any{"enabled_log_types": enabled}})
	}
	if len(out) == 0 {
		out = append(out, Observation{Scope: label, Outcome: "unknown", Error: "no_clusters_observed"})
	}
	return out
}

type GitHub struct {
	Token   string
	BaseURL string
	Client  *http.Client
}

func (g GitHub) Collect(ctx context.Context, scope assessment.Scope, m assessment.Measure) (Result, error) {
	r := Result{Limitations: []string{"Repository configuration intent only: does not prove deployment, live coverage, or recurring drift execution."}}
	for _, repo := range scope.Repositories {
		if len(repo.Paths) == 0 {
			r.Observations = append(r.Observations, Observation{Scope: repo.Name + "@" + repo.Commit, Outcome: "unknown", Error: "no_paths_configured"})
		}
		for _, path := range repo.Paths {
			label := repo.Name + "@" + repo.Commit + ":" + path
			if g.Token == "" {
				r.Observations = append(r.Observations, Observation{Scope: label, Outcome: "error", Error: "github_credentials_missing"})
				continue
			}
			endpoint := strings.TrimRight(g.BaseURL, "/") + "/repos/" + repo.Name + "/contents/" + url.PathEscape(path) + "?ref=" + url.QueryEscape(repo.Commit)
			req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
			if err != nil {
				return r, err
			}
			req.Header.Set("Authorization", "Bearer "+g.Token)
			req.Header.Set("Accept", "application/vnd.github+json")
			response, err := g.Client.Do(req)
			if err != nil {
				r.Observations = append(r.Observations, Observation{Scope: label, Outcome: "error", Error: "repository_request_failed"})
				continue
			}
			b, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
			response.Body.Close()
			if readErr != nil || response.StatusCode != http.StatusOK {
				r.Observations = append(r.Observations, Observation{Scope: label, Outcome: "error", Error: "repository_file_unavailable"})
				continue
			}
			var file struct{ Content, Encoding, SHA, Type string }
			if json.Unmarshal(b, &file) != nil || file.Type != "file" || file.Encoding != "base64" {
				r.Observations = append(r.Observations, Observation{Scope: label, Outcome: "error", Error: "unsupported_repository_response"})
				continue
			}
			content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
			if err != nil {
				r.Observations = append(r.Observations, Observation{Scope: label, Outcome: "error", Error: "invalid_repository_content"})
				continue
			}
			matches := map[string]bool{}
			status := "pass"
			if len(m.Expected) == 0 {
				status = "unknown"
			}
			for _, term := range m.Expected {
				matches[term] = strings.Contains(strings.ToLower(string(content)), strings.ToLower(term))
				if !matches[term] {
					status = "fail"
				}
			}
			// Persist matches and immutable Git provenance, not arbitrary source files
			// that could include secrets unrelated to this configuration measure.
			r.Observations = append(r.Observations, Observation{Scope: label, Outcome: status, Data: map[string]any{"blob_sha": file.SHA, "commit": repo.Commit, "path": path, "matches": matches, "evidence_kind": "configuration_intent"}})
		}
	}
	r.Summarize()
	return r, nil
}

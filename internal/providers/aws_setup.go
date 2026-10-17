package providers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jaxxstorm/fed/internal/connections"
)

func (p AWS) SetupMaterial(account, external, roleARN string) (connections.SetupMaterial, error) {
	policy, err := AWSReadPolicy(account)
	if err != nil {
		return connections.SetupMaterial{}, err
	}
	if len(external) != 64 || strings.Trim(external, "0123456789abcdef") != "" || p.FedRoleARN == "" {
		return connections.SetupMaterial{}, connections.ErrInvalid
	}
	trust, err := json.MarshalIndent(map[string]any{"Version": "2012-10-17", "Statement": []any{map[string]any{"Effect": "Allow", "Principal": map[string]string{"AWS": p.FedRoleARN}, "Action": "sts:AssumeRole", "Condition": map[string]any{"StringEquals": map[string]string{"sts:ExternalId": external}}}}}, "", "  ")
	if err != nil {
		return connections.SetupMaterial{}, err
	}
	hash := sha256.Sum256([]byte(external))
	name := "FedEvidence-" + hex.EncodeToString(hash[:6])
	rolePath := "/"
	if roleARN != "" {
		if !validRole(roleARN, account) {
			return connections.SetupMaterial{}, connections.ErrInvalid
		}
		resource := strings.TrimPrefix(roleARN, "arn:aws:iam::"+account+":role/")
		at := strings.LastIndex(resource, "/")
		if at >= 0 {
			rolePath = "/" + resource[:at+1]
		}
		name = resource[at+1:]
	} else {
		roleARN = "arn:aws:iam::" + account + ":role/" + name
	}
	// Existing roles are never overwritten. JSON comparison intentionally uses
	// jq structural equality, not key ordering or URL-encoded CLI output.
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
command -v jq >/dev/null || { echo 'Install jq before continuing.' >&2; exit 1; }
account=$(aws sts get-caller-identity --query Account --output text)
[[ "$account" == '%s' ]] || { echo 'Wrong AWS account. Select the customer administrator credentials.' >&2; exit 1; }
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cat > "$work/trust.json" <<'FED_TRUST_JSON'
%s
FED_TRUST_JSON
cat > "$work/read.json" <<'FED_READ_JSON'
%s
FED_READ_JSON
if aws iam get-role --role-name '%s' > "$work/existing.json" 2> "$work/error"; then
  jq -e --arg arn '%s' '.Role.Arn == $arn' "$work/existing.json" >/dev/null || { echo 'Existing role path differs. No changes made.' >&2; exit 1; }
  jq -e --slurpfile expected "$work/trust.json" '.Role.AssumeRolePolicyDocument == $expected[0]' "$work/existing.json" >/dev/null || { echo 'Existing role has different trust. Review it manually; no changes made.' >&2; exit 1; }
  aws iam get-role-policy --role-name '%s' --policy-name FedEvidenceReadOnly > "$work/existing-policy.json"
  jq -e --slurpfile expected "$work/read.json" '.PolicyDocument == $expected[0]' "$work/existing-policy.json" >/dev/null || { echo 'Existing read policy differs. Review it manually; no changes made.' >&2; exit 1; }
  echo 'Existing role and read policy match. Review any additional attached policies separately.'
else
  grep -q '(NoSuchEntity)' "$work/error" || { cat "$work/error" >&2; exit 1; }
  aws iam create-role --role-name '%s' --path '%s' --assume-role-policy-document "file://$work/trust.json" >/dev/null
  aws iam put-role-policy --role-name '%s' --policy-name FedEvidenceReadOnly --policy-document "file://$work/read.json"
fi
echo '%s'
`, account, trust, policy, name, roleARN, name, name, rolePath, name, roleARN)
	return connections.SetupMaterial{Version: AWSReadPolicyVersion, RoleARN: roleARN, TrustPolicy: trust, ReadPolicy: policy, CLI: script, Instructions: []string{"Sign into the customer account as an administrator authorized to create IAM roles.", "Create the suggested role using this exact trust policy, then add the read policy as FedEvidenceReadOnly.", "For explicit external-ID replacement on an existing role, review and update its trust policy manually with the new document. The CLI intentionally stops on conflicting trust.", "Alternatively run the CLI script using customer administrator credentials. Fed itself performs no IAM writes.", "Submit the role ARN in Fed, verify access, then discover and explicitly select sources. Monitoring is restricted to us-east-1."}}, nil
}

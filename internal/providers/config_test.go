package providers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaxxstorm/fed/internal/connections"
)

func TestGitHubSecretInjectedKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	raw := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	values := map[string]string{"KSI_GITHUB_APP_ID": "42", "KSI_GITHUB_APP_SLUG": "test", "KSI_GITHUB_CLIENT_ID": "client", "KSI_GITHUB_CLIENT_SECRET": "secret", "KSI_PUBLIC_URL": "https://fed.example", "KSI_GITHUB_PRIVATE_KEY": string(raw)}
	get := func(key string) string { return values[key] }
	adapters, err := FromEnv(get)
	if err != nil || adapters[connections.GitHub] == nil || !SetupReadiness(get)[connections.GitHub].Ready {
		t.Fatal("inline secret unavailable", err)
	}
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	values["KSI_GITHUB_PRIVATE_KEY_FILE"] = path
	if _, err := FromEnv(get); err != ProviderError("github_application_configuration_invalid") {
		t.Fatal("ambiguous key accepted", err)
	}
	delete(values, "KSI_GITHUB_PRIVATE_KEY")
	adapters, err = FromEnv(get)
	if err != nil || adapters[connections.GitHub] == nil {
		t.Fatal("file key unavailable", err)
	}
	delete(values, "KSI_GITHUB_PRIVATE_KEY_FILE")
	values["KSI_GITHUB_PRIVATE_KEY"] = "invalid-secret-do-not-echo"
	if _, err := FromEnv(get); err != ProviderError("github_application_configuration_invalid") {
		t.Fatal("invalid key did not return redacted error")
	}
}

func TestApplicationReadiness(t *testing.T) {
	values := map[string]string{}
	get := func(key string) string { return values[key] }
	adapters, err := FromEnv(get)
	if err != nil || len(adapters) != 1 || adapters[connections.AWS] == nil {
		t.Fatal("missing applications prevented environment provider", err)
	}
	values["KSI_GOOGLE_CLIENT_ID"] = "client"
	adapters, err = FromEnv(get)
	if err != nil || adapters[connections.Workspace] != nil || adapters[connections.AWS] == nil {
		t.Fatal("partial OAuth configuration disabled unrelated provider", err)
	}
	if SetupReadiness(get)[connections.Workspace].Ready {
		t.Fatal("incomplete registration marked ready")
	}
	values["KSI_GOOGLE_CLIENT_SECRET"] = "secret"
	adapters, err = FromEnv(get)
	if err != nil || adapters[connections.Workspace] == nil {
		t.Fatal("configured Workspace unavailable")
	}
	values["KSI_GITHUB_APP_ID"] = "42"
	adapters, err = FromEnv(get)
	if err != nil || adapters[connections.GitHub] != nil || adapters[connections.Workspace] == nil {
		t.Fatal("partial GitHub configuration disabled Workspace", err)
	}
}

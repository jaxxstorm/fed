package providers

import (
	"testing"

	"github.com/jaxxstorm/fed/internal/connections"
)

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

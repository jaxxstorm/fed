package config

import "testing"

func TestStartupValidation(t *testing.T) {
	base := map[string]string{"DATABASE_URL": "postgres://ksi:ksi@localhost:5432/ksi", "KSI_DEV_IDENTITY": "true"}
	for _, tt := range []struct {
		name    string
		changes map[string]string
		valid   bool
	}{
		{"local", nil, true},
		{"wildcard", map[string]string{"KSI_LISTEN": "0.0.0.0:8080"}, false},
		{"dns", map[string]string{"KSI_LISTEN": "localhost:8080"}, false},
		{"ipv6 loopback", map[string]string{"KSI_LISTEN": "[::1]:8080"}, true},
		{"production missing identity", map[string]string{"KSI_DEV_IDENTITY": "false"}, false},
		{"invalid flag", map[string]string{"KSI_DEV_IDENTITY": "maybe"}, false},
		{"missing database", map[string]string{"DATABASE_URL": ""}, false},
		{"unknown mode", map[string]string{"KSI_MODE": "something"}, false},
		{"worker", map[string]string{"KSI_MODE": "worker", "KSI_DEV_IDENTITY": "false"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromEnv(func(k string) string {
				if v, ok := tt.changes[k]; ok {
					return v
				}
				return base[k]
			})
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v, error=%v", tt.valid, err)
			}
		})
	}
}

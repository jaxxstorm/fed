package connections

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProviderHTTPBoundsAndRedirects(t *testing.T) {
	calls := 0
	p := NewProviderHTTP(transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://untrusted.example/"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	}), "api.github.com")
	for _, endpoint := range []string{"http://api.github.com/", "https://untrusted.example/", "https://user@api.github.com/", "https://api.github.com:444/"} {
		req, _ := http.NewRequest("GET", endpoint, nil)
		if _, err := p.Do(req); err == nil {
			t.Fatalf("allowed %s", endpoint)
		}
	}
	if calls != 0 {
		t.Fatal("unapproved request reached transport")
	}
	req, _ := http.NewRequest("GET", "https://api.github.com/", nil)
	response, err := p.Do(req)
	if err != nil || response.StatusCode != 302 || calls != 1 {
		t.Fatalf("redirect followed: %v", err)
	}
	response.Body.Close()
	if _, err := ReadProviderBody(io.NopCloser(bytes.NewReader(make([]byte, MaxResponseBytes+1)))); err == nil {
		t.Fatal("oversized response accepted")
	}
}

func TestCatalogAndCredentialIsolation(t *testing.T) {
	r, err := NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []Provider{GitHub, Workspace, AWS, Kubernetes, Documents} {
		d, ok := r.Provider(p)
		if !ok || d.Guide.Version == "" || len(d.Guide.Steps) == 0 {
			t.Fatalf("missing guide for %s", p)
		}
		if _, err := r.Adapter(p); err == nil {
			t.Fatal("missing adapter claimed ready")
		}
	}
	values := map[string]string{"token": "secret-value"}
	c := NewCredentials(values)
	values["token"] = "changed"
	if c.Value("token") != "secret-value" {
		t.Fatal("credentials alias caller map")
	}
	b, _ := json.Marshal(c)
	if strings.Contains(string(b)+fmt.Sprintf("%v %+v %#v", c, c, c), "secret-value") {
		t.Fatal("secret serialized or logged")
	}
	if n, _ := PageSize(0); n != 25 {
		t.Fatal("incorrect default page size")
	}
	if _, err := PageSize(101); err == nil {
		t.Fatal("unbounded page")
	}
}

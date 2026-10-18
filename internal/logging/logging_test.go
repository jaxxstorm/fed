package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
)

type broken struct{}

func (broken) Write([]byte) (int, error) { return 0, errors.New("secret sink details") }

func TestLevelsAndSinkFailure(t *testing.T) {
	for _, level := range []string{"", "DEBUG", "info", "warn", "error"} {
		var out bytes.Buffer
		l, err := New(&out, nil, level, "json", "server")
		if err != nil {
			t.Fatal(err)
		}
		ctx := WithLogger(context.Background(), l)
		for _, v := range []zerolog.Level{zerolog.DebugLevel, zerolog.InfoLevel, zerolog.WarnLevel, zerolog.ErrorLevel} {
			Emit(ctx, v, Fields{Event: "test.completed", Component: "test"})
		}
		for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
			if len(line) > 1024 {
				t.Fatal("unexpectedly large completion event")
			}
			var event map[string]any
			if json.Unmarshal([]byte(line), &event) != nil || event["time"] == nil || event["mode"] != "server" {
				t.Fatal(line)
			}
		}
		if level == "" && strings.Contains(out.String(), `"level":"debug"`) {
			t.Fatal("default logged debug")
		}
	}
	for _, setting := range [][2]string{{"verbose", "json"}, {"info", "xml"}} {
		if _, err := New(nil, nil, setting[0], setting[1], "server"); err == nil {
			t.Fatal("invalid setting accepted")
		}
	}
	var fallback bytes.Buffer
	l, _ := New(broken{}, &fallback, "debug", "json", "worker")
	ctx := WithLogger(context.Background(), l)
	for i := 0; i < 3; i++ {
		Emit(ctx, zerolog.InfoLevel, Fields{Event: "test.completed", Component: "test"})
	}
	if strings.Count(fallback.String(), "output failed") != 1 || strings.Contains(fallback.String(), "secret") {
		t.Fatal(fallback.String())
	}
}

func TestConcurrentContextIsolation(t *testing.T) {
	var out bytes.Buffer
	l, _ := New(&out, nil, "debug", "json", "all")
	root := WithLogger(context.Background(), l)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, id := NewRequest(root)
			ctx = Operation(ctx)
			Actor(ctx, id)
			Route(ctx, "GET /services/{service}")
			Emit(ctx, zerolog.InfoLevel, Fields{Event: "test.completed", Component: "test"})
		}()
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var e map[string]any
		_ = json.Unmarshal([]byte(line), &e)
		if e["actor_id"] != e["request_id"] || seen[e["operation_id"].(string)] {
			t.Fatal(e)
		}
		seen[e["operation_id"].(string)] = true
	}
	if len(seen) != 20 {
		t.Fatal(len(seen))
	}
}

func BenchmarkCompletion(b *testing.B) {
	l, _ := New(nil, nil, "info", "json", "server")
	ctx := WithLogger(context.Background(), l)
	b.ReportAllocs()
	for b.Loop() {
		Emit(ctx, zerolog.InfoLevel, Fields{Event: "http.request.completed", Component: "http", Route: "GET /services/{service}", Status: 200})
	}
}

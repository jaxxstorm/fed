package logging

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

type commitStub struct{ err error }

func (s commitStub) Commit(context.Context) error { return s.err }
func TestCommitOutcomeAndSafeErrors(t *testing.T) {
	var logs bytes.Buffer
	l, _ := New(&logs, nil, "debug", "json", "server")
	ctx := WithLogger(context.Background(), l)
	if err := Commit(ctx, commitStub{errors.New("secret DSN")}, Fields{Event: "test.committed", Component: "test"}); err == nil {
		t.Fatal("commit failure hidden")
	}
	if strings.Contains(logs.String(), `"outcome":"success"`) || strings.Contains(logs.String(), "secret") {
		t.Fatal(logs.String())
	}
	_ = Commit(ctx, commitStub{}, Fields{Event: "test.committed", Component: "test", Outcome: "replayed"})
	Emit(ctx, zerolog.WarnLevel, Fields{Event: "test.failure", Component: "test", ErrorCode: "secret provider body"})
	if !strings.Contains(logs.String(), `"outcome":"replayed"`) || strings.Contains(logs.String(), "secret") {
		t.Fatal(logs.String())
	}
}

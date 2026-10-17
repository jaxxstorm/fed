package observation

import "testing"

func TestRecoveryDoesNotGuessCause(t *testing.T) {
	unknown := ExplainError("unclassified_api_error", "production/cluster", "external agent")
	if unknown.Retryable || unknown.Title != "Evidence collection could not finish" {
		t.Fatal("unknown error guessed a remedy")
	}
	known := ExplainError("expired_credentials", "production", "server collector")
	if !known.Retryable || known.Title != "AWS sign-in expired" || known.Environment != "server collector" {
		t.Fatal("lost known cause or environment")
	}
}

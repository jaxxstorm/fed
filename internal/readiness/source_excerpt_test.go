package readiness

import "testing"

func TestDecodedRulesExcerptsRequireAnExactPointer(t *testing.T) {
	raw := []byte(`{"rules":{"quoted":{"statement":"The term \"incident\" includes a policy violation."},"items":["First\nSecond"],"a/b":{"~key":"\u00e9vidence"}}}`)
	for _, tc := range []struct {
		anchor, excerpt string
		want            bool
	}{
		{"/rules/quoted/statement", `The term "incident" includes`, true},
		{"/rules/missing/statement", `The term "incident" includes`, false},
		{"/rules/items/0", "First\nSecond", true},
		{"/rules/items/00", "First\nSecond", false},
		{"/rules/a~1b/~0key", "évidence", true},
		{"/rules/a~2b/~0key", "évidence", false},
		{"/rules/quoted/statement", `The term "incident" excludes`, false},
	} {
		if got := SourceExcerptMatches(raw, "rules", tc.anchor, tc.excerpt); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.anchor, got, tc.want)
		}
	}
	if SourceExcerptMatches(raw, "site", "/rules/items/0", "First\nSecond") {
		t.Fatal("decoded non-Rules source")
	}
}

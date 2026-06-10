package chain

import "testing"

func TestParseDecision(t *testing.T) {
	cases := map[string]Decision{
		"":  DecisionProceed,
		"y": DecisionProceed,
		"s": DecisionSkip,
		"q": DecisionQuit,
		"e": DecisionEdit,
	}
	for in, want := range cases {
		if got := parseDecision(in); got != want {
			t.Errorf("parseDecision(%q)=%v want %v", in, got, want)
		}
	}
}

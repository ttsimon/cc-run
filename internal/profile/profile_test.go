package profile

import "testing"

func TestRedactToken(t *testing.T) {
	cases := map[string]string{
		"sk-FAKE000000000000000000000000000": "sk-FAKE…",
		"ark-FAKE0000":                       "ark-FAK…",
		"短":                                  "…",
		"":                                   "",
	}
	for in, want := range cases {
		if got := RedactToken(in); got != want {
			t.Errorf("RedactToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactEnvHidesSecrets(t *testing.T) {
	env := map[string]string{
		"ANTHROPIC_AUTH_TOKEN": "sk-FAKE000000000000000000000000000",
		"ANTHROPIC_BASE_URL":   "https://api.deepseek.com/anthropic",
	}
	out := RedactEnv(env)
	if out["ANTHROPIC_AUTH_TOKEN"] != "sk-FAKE…" {
		t.Errorf("token 未打码: %q", out["ANTHROPIC_AUTH_TOKEN"])
	}
	if out["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com/anthropic" {
		t.Errorf("非密钥被改动: %q", out["ANTHROPIC_BASE_URL"])
	}
}

package store

import "testing"

func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"https://www.Claude.ai/chat":   "claude.ai",
		"chatgpt.com":                  "chatgpt.com",
		"http://music.youtube.com:443": "music.youtube.com",
		"  netflix.com  ":              "netflix.com",
		"":                             "",
		"localhost":                    "",
		"not a url":                    "",
		"https://exa mple.com":         "",
		"javascript:alert(1)":          "",
		"https://evil.com&sz=1#x":      "",
	}
	for in, want := range cases {
		if got := NormalizeDomain(in); got != want {
			t.Errorf("NormalizeDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

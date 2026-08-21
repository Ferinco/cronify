package config

import "testing"

func TestWeakAdminToken(t *testing.T) {
	cases := []struct {
		name  string
		token string
		want  bool
	}{
		{"local-dev-token", "dev", true},
		{"short-word", "password123", true},
		{"exactly-at-threshold", "12345678901234567890", false}, // 21 chars, one over threshold
		{"real-random-token", "8abbe6d5e98736e48c58535bbeda7b7cfc795c05addb075c296fedd95226669", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{AdminToken: tc.token}
			if got := cfg.WeakAdminToken(); got != tc.want {
				t.Errorf("WeakAdminToken() for token of length %d = %v, want %v", len(tc.token), got, tc.want)
			}
		})
	}
}

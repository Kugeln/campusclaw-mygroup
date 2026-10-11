package main

import (
	"strings"
	"testing"
)

func TestDeploymentConfiguration(t *testing.T) {
	for _, tc := range []struct {
		mode, secret string
		valid        bool
	}{
		{"fresh", strings.Repeat("x", 32), true},
		{"migrate", strings.Repeat("x", 32), true},
		{"", strings.Repeat("x", 32), false},
		{"infrastructure", strings.Repeat("x", 32), false},
		{"fresh", "", false}, {"migrate", "short", false},
	} {
		t.Run(tc.mode+"/"+string(rune('A'+len(tc.secret))), func(t *testing.T) {
			err := validate(func(key string) string {
				if key == "INITIALIZATION_MODE" {
					return tc.mode
				}
				return tc.secret
			})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
			if err != nil && tc.secret != "" && strings.Contains(err.Error(), tc.secret) {
				t.Fatal("secret leaked")
			}
		})
	}
}

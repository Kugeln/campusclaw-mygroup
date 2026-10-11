package config

import "testing"

func TestUploadLimit(t *testing.T) {
	for _, v := range []string{"-1", "0", "wrong", "9223372036854775807"} {
		if _, e := UploadLimit(func(string) string { return v }); e == nil {
			t.Fatal("invalid upload limit accepted")
		}
	}
	if n, e := UploadLimit(func(string) string { return "" }); e != nil || n != 20971520 {
		t.Fatal("default limit changed")
	}
}

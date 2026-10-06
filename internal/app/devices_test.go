package app

import "testing"

func TestCleanDeviceName(t *testing.T) {
	for in, want := range map[string]string{
		"Work laptop":         "Work laptop",
		"  spaced   out  ":    "spaced out",
		"evil\nname\x00:here": "evilnamehere",
		"café-ü_1.0":          "caf-_1.0",
		"a:b/c\\d\"e'f":       "abcdef",
	} {
		if got := CleanDeviceName(in); got != want {
			t.Errorf("CleanDeviceName(%q) = %q, want %q", in, got, want)
		}
	}
}

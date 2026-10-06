package uuid

import "testing"

func TestRoundTrip(t *testing.T) {
	u := New()
	if len(u.Hex()) != 32 || len(u.String()) != 36 {
		t.Fatalf("Hex()=%q String()=%q", u.Hex(), u.String())
	}
	// Both stored forms must parse back to the same value.
	for _, s := range []string{u.Hex(), u.String()} {
		got, err := Parse(s)
		if err != nil || got != u {
			t.Fatalf("Parse(%q) = %v, %v", s, got, err)
		}
	}
	// The driver must see the bare hex form the char(32) columns expect.
	v, err := u.Value()
	if err != nil || v.(string) != u.Hex() {
		t.Fatalf("Value() = %v, %v; want %q", v, err, u.Hex())
	}
	// Scan must accept what Value produced.
	var back UUID
	if err := back.Scan(v); err != nil || back != u {
		t.Fatalf("Scan(%v) = %v, %v", v, back, err)
	}
	// JSON must use the hyphenated form.
	text, _ := u.MarshalText()
	if string(text) != u.String() {
		t.Fatalf("MarshalText() = %q, want %q", text, u.String())
	}
	if !Nil.IsZero() || u.IsZero() {
		t.Fatal("IsZero is wrong")
	}
}

package api

import "testing"

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v3.1.0", "3.0.0", true},
		{"v3.0.0", "v3.0.0", false},
		{"v3.0.0", "v3.0.1", false},
		{"v3.0.0", "3.0.0-rc.1", true},
		{"v3.0.0-rc.2", "3.0.0-rc.1", false},
		{"v3.0.1", "nightly", false},
		{"garbage", "3.0.0", false},
	}
	for _, c := range cases {
		if got := isNewerVersion(c.latest, c.current); got != c.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestNormalizeUpdateTag(t *testing.T) {
	cases := []struct {
		input string
		want  string
		valid bool
	}{
		{input: "", want: "", valid: true},
		{input: "latest", want: "", valid: true},
		{input: "3.1", want: "v3.1", valid: true},
		{input: "v4.0.0", want: "v4.0.0", valid: true},
		{input: "3.1.0-rc.1", want: "v3.1.0-rc.1", valid: true},
		{input: "commit:0123456789abcdef0123456789abcdef01234567", want: "commit:0123456789abcdef0123456789abcdef01234567", valid: true},
		{input: "v3/1", valid: false},
		{input: "https://example.com", valid: false},
	}

	for _, test := range cases {
		got, err := normalizeUpdateTag(test.input)
		if (err == nil) != test.valid || got != test.want {
			t.Errorf("normalizeUpdateTag(%q) = %q, %v; want %q, valid=%t", test.input, got, err, test.want, test.valid)
		}
	}
}

func TestSameCommit(t *testing.T) {
	full := "0123456789abcdef0123456789abcdef01234567"
	if !sameCommit(full[:7], full) {
		t.Fatal("expected abbreviated and full hashes of the same commit to match")
	}
	if sameCommit("unknown", full) {
		t.Fatal("unknown build hash must not match a commit")
	}
}

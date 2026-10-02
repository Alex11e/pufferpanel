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

package database

import "testing"

func TestIsGeneratedSocialUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     bool
	}{
		{name: "generated username", username: "social-0123456789a", want: true},
		{name: "wrong prefix", username: "member-0123456789a", want: false},
		{name: "wrong suffix length", username: "social-0123456789ab", want: false},
		{name: "non-hex suffix", username: "social-012345678ga", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isGeneratedSocialUsername(test.username); got != test.want {
				t.Errorf("isGeneratedSocialUsername(%q) = %t, want %t", test.username, got, test.want)
			}
		})
	}
}

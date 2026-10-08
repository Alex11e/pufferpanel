package auth

import (
	"strings"
	"testing"

	"github.com/pufferpanel/pufferpanel/v3/config"
)

func TestSocialFlowCookieCodecRoundTrip(t *testing.T) {
	previous := config.SessionKey.Value()
	if err := config.SessionKey.Set(strings.Repeat("22", 32), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.SessionKey.Set(previous, false) })

	codec, err := socialFlowCodec()
	if err != nil {
		t.Fatal(err)
	}
	want := socialFlow{State: "state", Verifier: "verifier", Provider: "google", Mode: "login", IssuedAt: 123}
	encoded, err := codec.Encode(socialFlowCookieName, want)
	if err != nil {
		t.Fatal(err)
	}
	var got socialFlow
	if err := codec.Decode(socialFlowCookieName, encoded, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("decoded flow = %#v, want %#v", got, want)
	}
}

func TestSocialFlowCookieCodecRejectsTampering(t *testing.T) {
	previous := config.SessionKey.Value()
	if err := config.SessionKey.Set(strings.Repeat("33", 32), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.SessionKey.Set(previous, false) })

	codec, err := socialFlowCodec()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := codec.Encode(socialFlowCookieName, socialFlow{State: "state", Provider: "google", Mode: "login", IssuedAt: 123})
	if err != nil {
		t.Fatal(err)
	}
	tampered := "A" + encoded[1:]
	var flow socialFlow
	if err := codec.Decode(socialFlowCookieName, tampered, &flow); err == nil {
		t.Fatal("tampered flow cookie was accepted")
	}
}

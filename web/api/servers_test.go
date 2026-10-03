package api

import "testing"

func TestNormalizeModrinthQuery(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "playit.gg", want: "playit companion"},
		{input: " paper plugin ", want: "paper plugin"},
		{input: "world download", want: "world download"},
	}

	for _, tc := range cases {
		if got := normalizeModrinthQuery(tc.input); got != tc.want {
			t.Fatalf("normalizeModrinthQuery(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSelectBestModrinthPluginVersionPrefersPaperBuild(t *testing.T) {
	versions := []struct {
		VersionNumber string   `json:"version_number"`
		GameVersions  []string `json:"game_versions"`
		Loaders       []string `json:"loaders"`
		Files         []struct {
			URL      string `json:"url"`
			Filename string `json:"filename"`
			Primary  bool   `json:"primary"`
		} `json:"files"`
	}{
		{
			VersionNumber: "0.1.2-forge",
			GameVersions:  []string{"1.21.8"},
			Loaders:       []string{"forge"},
			Files: []struct {
				URL      string `json:"url"`
				Filename string `json:"filename"`
				Primary  bool   `json:"primary"`
			}{
				{URL: "https://example.com/forge.jar", Filename: "playit_companion-forge-0.1.2.jar", Primary: true},
			},
		},
		{
			VersionNumber: "0.1.1-paper",
			GameVersions:  []string{"1.21.8"},
			Loaders:       []string{"paper"},
			Files: []struct {
				URL      string `json:"url"`
				Filename string `json:"filename"`
				Primary  bool   `json:"primary"`
			}{
				{URL: "https://example.com/paper.jar", Filename: "playit_companion-paper-0.1.1.jar", Primary: true},
			},
		},
	}

	url, filename, versionNumber := selectBestModrinthPluginVersion(versions, "1.21.8")
	if url != "https://example.com/paper.jar" {
		t.Fatalf("selected wrong URL: %q", url)
	}
	if filename != "playit_companion-paper-0.1.1.jar" {
		t.Fatalf("selected wrong filename: %q", filename)
	}
	if versionNumber != "0.1.1-paper" {
		t.Fatalf("selected wrong version: %q", versionNumber)
	}
}

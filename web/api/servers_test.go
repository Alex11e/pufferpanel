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

func TestShouldInstallPlayitPlugin(t *testing.T) {
	tests := []struct {
		serverType string
		launcher   string
		want       bool
	}{
		{serverType: "minecraft-java", launcher: "paper", want: true},
		{serverType: "minecraft-java", launcher: "PURPUR", want: true},
		{serverType: "minecraft-java", launcher: "pufferfish", want: true},
		{serverType: "minecraft-java", launcher: "spigot", want: true},
		{serverType: "minecraft-java", launcher: "fabric", want: false},
		{serverType: "minecraft-bedrock", launcher: "paper", want: false},
	}
	for _, test := range tests {
		if got := shouldInstallPlayitPlugin(test.serverType, test.launcher); got != test.want {
			t.Errorf("shouldInstallPlayitPlugin(%q, %q) = %t, want %t", test.serverType, test.launcher, got, test.want)
		}
	}
}

func TestPlayitPluginInstallOperationsCreateFolderBeforeDownload(t *testing.T) {
	filename := "playit_companion-paper.jar"
	operations := playitPluginInstallOperations("https://cdn.modrinth.com/data/project/version/"+filename, filename)
	if len(operations) != 3 {
		t.Fatalf("got %d operations, want 3", len(operations))
	}
	if operations[0].Type != "mkdir" || operations[0].Metadata["target"] != "plugins" {
		t.Fatalf("first operation must create plugins directory: %+v", operations[0])
	}
	if operations[1].Type != "download" {
		t.Fatalf("second operation must download Playit: %+v", operations[1])
	}
	if operations[2].Type != "move" || operations[2].Metadata["target"] != "plugins/"+filename {
		t.Fatalf("last operation must move Playit into plugins: %+v", operations[2])
	}
}

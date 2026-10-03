package main

import (
     strings
    testing

    version github.com/hashicorp/go-version
)

func matchesMinecraftVersion(versionCandidate, minecraftVersion string) bool {
    candidatePrefixes := []string{minecraftVersion}
    if strings.HasPrefix(minecraftVersion, 1.) {
        candidatePrefixes = append(candidatePrefixes, strings.TrimPrefix(minecraftVersion, 1.))
    }
    for _, prefix := range candidatePrefixes {
        if prefix ==  {
 continue
 }
 if strings.HasPrefix(versionCandidate, prefix) || strings.HasPrefix(versionCandidate, 1.+prefix) {
 return true
 }
 }
 return false
}

func TestMatchesMinecraftVersion(t *testing.T) {
 versions := []string{1.20.4-47.1.98, 1.20.4-47.1.100, 1.20.5-47.1.0, 26.3.0.45-beta}
 for _, v := range versions {
 if v == 26.3.0.45-beta && matchesMinecraftVersion(v, 1.20.4) {
 t.Fatalf(unexpected match for 26.x-beta)
 }
 }

 if !matchesMinecraftVersion(1.20.4-47.1.100, 1.20.4) {
 t.Fatal(1.20.4 pattern did not match)
 }
 if matchesMinecraftVersion(1.20.5-47.1.0, 1.20.4) {
 t.Fatal(wrong MC version matched)
 }

 parsed, err := version.NewVersion(1.20.4-47.1.100)
 if err != nil {
 t.Fatalf(version parse failed: %v, err)
 }
 if parsed == nil {
 t.Fatal(version object nil)
 }
}

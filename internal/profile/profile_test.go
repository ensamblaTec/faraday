package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := &Profile{Description: "spotify", Flatpak: "com.spotify.Client", Loopback: false}
	if err := Save("spotify", want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load("spotify")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Flatpak != want.Flatpak || got.Description != want.Description {
		t.Fatalf("round trip mismatch: %+v != %+v", got, want)
	}
}

func TestListEmptyWhenNoDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "nope"))
	names, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("want 0 profiles, got %v", names)
	}
}

func TestSaveRejectsTraversalName(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	if err := Save("../evil", &Profile{Cmd: []string{"true"}}); err == nil {
		t.Fatal(`Save("../evil") succeeded; want rejection`)
	}
	// The profiles dir is base/faraday/profiles; "../evil" would land in
	// base/faraday/evil.yaml — assert nothing escaped the profiles dir.
	if _, err := os.Stat(filepath.Join(base, "faraday", "evil.yaml")); err == nil {
		t.Fatal("Save wrote a profile outside the profiles/ directory")
	}
}

func TestLoadRejectsTraversalName(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	// Plant a YAML file one level above the profiles dir.
	secret := filepath.Join(base, "faraday", "secret.yaml")
	if err := os.MkdirAll(filepath.Dir(secret), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, []byte("description: leaked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("../secret"); err == nil {
		t.Fatal(`Load("../secret") succeeded; traversal name should be rejected`)
	}
}

func TestListNames(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = Save("a", &Profile{Cmd: []string{"true"}})
	_ = Save("b", &Profile{Cmd: []string{"true"}})
	names, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("want 2, got %v", names)
	}
}

func TestListDedupsYamlAndYml(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	if err := Save("dup", &Profile{Cmd: []string{"true"}}); err != nil { // writes dup.yaml
		t.Fatalf("Save: %v", err)
	}
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	// Plant a sibling dup.yml next to dup.yaml; List must report "dup" once.
	if err := os.WriteFile(filepath.Join(dir, "dup.yml"), []byte("cmd: [true]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	count := 0
	for _, n := range names {
		if n == "dup" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf(`want "dup" listed once, got %d occurrences in %v`, count, names)
	}
}

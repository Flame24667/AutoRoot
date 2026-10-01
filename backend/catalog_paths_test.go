package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateCatalogOverridesSharedMetadata(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "setup")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(dir, "firmware-db.json")
	local := filepath.Join(dir, "firmware-db.local.json")
	for _, path := range []string{shared, local} {
		if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if got := selectCatalogPath([]string{root}, ""); got != local {
		t.Fatalf("private config not selected: %s", got)
	}
	if got := selectCatalogPath([]string{root}, "explicit.json"); got != "explicit.json" {
		t.Fatal("explicit override lost")
	}
}

func TestRelativeCatalogPathsAreProjectRelative(t *testing.T) {
	root := t.TempDir()
	db := filepath.Join(root, "setup", "firmware-db.json")
	want := filepath.Join(root, "firmware", "SM-A065F", "stock.zip")
	if got := catalogLocalPath(db, "firmware/SM-A065F/stock.zip"); got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if got := catalogLocalPath(db, want); got != want {
		t.Fatal("absolute private path changed")
	}
	if got := catalogLocalPath(db, ""); got != "" {
		t.Fatal("missing file became a directory")
	}
}

func TestSharedCatalogWithoutPrivateOverride(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "setup")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(dir, "firmware-db.json")
	if err := os.WriteFile(shared, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := selectCatalogPath([]string{root}, ""); got != shared {
		t.Fatalf("public catalog not selected: %s", got)
	}
	externalDB := filepath.Join(root, "custom", "catalog.json")
	if got := catalogLocalPath(externalDB, "stock.zip"); got != filepath.Join(root, "custom", "stock.zip") {
		t.Fatalf("external catalog path resolved incorrectly: %s", got)
	}
}

package storage

import (
	"path/filepath"
	"testing"
)

func TestDataDirectoryPrecedence(t *testing.T) {
	root := t.TempDir()
	environment := filepath.Join(root, "env")
	override := filepath.Join(root, "flag")
	t.Setenv("SKY_DATA_DIR", environment)
	if got, err := DataDir(override); err != nil || got != override {
		t.Fatalf("override: %q %v", got, err)
	}
	if got, err := DataDir(""); err != nil || got != environment {
		t.Fatalf("environment: %q %v", got, err)
	}
	t.Setenv("SKY_DATA_DIR", "   ")
	if _, err := DataDir(""); err == nil {
		t.Fatal("accepted blank environment directory")
	}
	home := filepath.Join(root, "home")
	vars := map[string]string{"XDG_DATA_HOME": filepath.Join(root, "xdg"), "LOCALAPPDATA": filepath.Join(root, "local")}
	getenv := func(key string) string { return vars[key] }
	for _, tc := range []struct{ os, want string }{{"darwin", filepath.Join(home, "Library", "Application Support", "sky")}, {"linux", filepath.Join(root, "xdg", "sky")}, {"windows", filepath.Join(root, "local", "Sky")}} {
		if got := platformDataDir(tc.os, home, getenv); got != tc.want {
			t.Errorf("%s: got %s want %s", tc.os, got, tc.want)
		}
	}
	vars["XDG_DATA_HOME"] = "relative"
	if got := platformDataDir("linux", home, getenv); got != filepath.Join(home, ".local", "share", "sky") {
		t.Fatal("relative XDG_DATA_HOME should be ignored")
	}
}

package main

import (
	"path/filepath"
	"testing"
)

func TestLaunchPath(t *testing.T) {
	path, help, err := launchPath(nil, "/tmp")
	if path != "" || help || err != nil {
		t.Fatalf("no args: %q help %v err %v", path, help, err)
	}

	if _, help, err = launchPath([]string{"--help"}, ""); !help || err != nil {
		t.Fatalf("help %v %v", help, err)
	}

	path, _, err = launchPath([]string{"src/main.go"}, "/repo")
	if err != nil || path != filepath.Join("/repo", "src/main.go") {
		t.Fatalf("relative file: %q %v", path, err)
	}

	path, _, err = launchPath([]string{"/abs/file.go"}, "/repo")
	if err != nil || path != "/abs/file.go" {
		t.Fatalf("absolute: %q %v", path, err)
	}

	path, _, err = launchPath([]string{"--", "notes.txt"}, "/work")
	if err != nil || path != filepath.Join("/work", "notes.txt") {
		t.Fatalf("dashdash: %q %v", path, err)
	}

	if _, _, err = launchPath([]string{"--unknown"}, ""); err == nil {
		t.Fatal("expected an error for an unknown option")
	}
	if _, _, err = launchPath([]string{"--"}, ""); err == nil {
		t.Fatal("expected an error for a missing path")
	}
}

package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/karolsko/gg/internal/gitx"
)

func TestChangesShell(t *testing.T) {
	a := newApp()
	a.repoPath = t.TempDir()
	a.repoName = "demo"
	a.nav = "changes"
	a.status = gitx.Status{
		Head:      "main",
		Upstream:  "origin/main",
		Ahead:     2,
		Behind:    1,
		HasRemote: true,
		Changes:   []gitx.Change{{Path: "readme.md", X: '.', Y: 'M'}},
	}
	a.unstaged = []gitx.Change{{Path: "readme.md", X: '.', Y: 'M'}}
	a.unstagedSel = 0
	a.changeSide = "unstaged"
	a.changePath = "readme.md"
	a.staged = []gitx.Change{{Path: "added.txt", X: 'A', Y: '.'}}
	a.refs = []gitx.Ref{{Name: "main", Kind: "branch", Current: true}}

	tt := ui.NewTester(a.view, 1200, 800)
	for _, want := range []string{"demo", "main", "↑2", "Unstaged", "readme.md", "Staged", "added.txt", "Commit", "Fetch", "Changes"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %q", want, tt.Texts())
		}
	}
}

func TestHistoryShell(t *testing.T) {
	a := newApp()
	a.repoPath = t.TempDir()
	a.repoName = "demo"
	a.nav = "branch:main"
	a.status = gitx.Status{Head: "main"}
	a.selectedHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	a.histSel = 0
	a.commits = []gitx.Commit{{
		Hash:    a.selectedHash,
		Subject: "Lay out the graph",
		Author:  "Ada",
		When:    time.Now().Add(-time.Hour),
		Refs:    []string{"main"},
	}}
	gitx.Layout(a.commits)
	a.files = []gitx.PathChange{{Status: "M", Path: "view.go"}}
	a.fileSel = 0
	a.filePath = "view.go"

	tt := ui.NewTester(a.view, 1200, 800)
	for _, want := range []string{"Lay out the graph", "Ada", "view.go", "main"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %q", want, tt.Texts())
		}
	}
}

func TestRepoList(t *testing.T) {
	a := newApp()
	a.recents = []string{"/work/alpha", "/work/beta"}
	a.repoPath = "/work/alpha"
	a.repoName = "alpha"
	a.nav = "changes"
	a.status = gitx.Status{Head: "main"}
	tt := ui.NewTester(a.view, 1200, 800)
	for _, want := range []string{"Repositories", "alpha", "beta", "/work/beta"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %q", want, tt.Texts())
		}
	}
}

func TestWelcome(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1100, 720)
	if !tt.HasText("gg") || !tt.HasText("Open Repository") || !tt.HasText("Clone") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

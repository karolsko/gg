package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	ctx := context.Background()
	if err := Init(ctx, dir); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")
	git("commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	top, err := Find(filepath.Join(dir, "a.txt"))
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(top)
	if err != nil || got != want {
		t.Fatalf("Find %q %v", top, err)
	}
	st, err := ReadStatus(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Head == "" || len(st.Changes) != 2 {
		t.Fatalf("%+v", st)
	}
	if err := Stage(ctx, dir, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := CreateCommit(ctx, dir, "second"); err != nil {
		t.Fatal(err)
	}
	commits, more, err := ReadLog(ctx, dir, "HEAD", false, 0, 10)
	if err != nil || more || len(commits) != 2 || commits[0].Subject != "second" {
		t.Fatalf("log %v more %v err %v", commits, more, err)
	}
	files, err := CommitFiles(ctx, dir, commits[0].Hash)
	if err != nil || len(files) != 1 || files[0].Path != "a.txt" {
		t.Fatalf("files %+v %v", files, err)
	}
	diff, err := DiffCommitFile(ctx, dir, commits[0].Hash, "a.txt")
	if err != nil || !strings.Contains(diff, "+b") {
		t.Fatalf("diff %q %v", diff, err)
	}
	st, err = ReadStatus(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	var untracked bool
	for _, ch := range st.Changes {
		if ch.Path == "new.txt" && ch.Unstaged() {
			untracked = true
		}
	}
	if !untracked {
		t.Fatalf("%+v", st.Changes)
	}
	refs, err := ReadRefs(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	var current bool
	for _, ref := range refs {
		if ref.Kind == "branch" && ref.Current {
			current = true
		}
	}
	if !current {
		t.Fatalf("refs %+v", refs)
	}
}

package gitx

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseStatus(t *testing.T) {
	raw := strings.Join([]string{
		"# branch.oid abcdef",
		"# branch.head main",
		"# branch.upstream origin/main",
		"# branch.ab +2 -1",
		"1 M. N... 100644 100644 100644 h h staged.go",
		"1 .M N... 100644 100644 100644 h h work.go",
		"1 MM N... 100644 100644 100644 h h both.go",
		"2 R. N... 100644 100644 100644 h h R100 new.go",
		"old.go",
		"u UU N... 100644 100644 100644 100644 h1 h2 h3 conflict.go",
		"? new file.txt",
		"! ignored.log",
	}, "\x00") + "\x00"

	st, err := ParseStatus([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if st.Head != "main" || st.Upstream != "origin/main" || st.Ahead != 2 || st.Behind != 1 || !st.HasRemote {
		t.Fatalf("header %+v", st)
	}
	if len(st.Changes) != 6 {
		t.Fatalf("got %d changes", len(st.Changes))
	}
	staged, unstaged := 0, 0
	for _, ch := range st.Changes {
		if ch.Staged() {
			staged++
		}
		if ch.Unstaged() {
			unstaged++
		}
	}
	// staged.go, both.go, new.go are staged. work.go, both.go, conflict, new file are unstaged.
	if staged != 3 || unstaged != 4 {
		t.Fatalf("staged %d unstaged %d\n%+v", staged, unstaged, st.Changes)
	}
	if st.Changes[3].Path != "new.go" || st.Changes[3].Orig != "old.go" || st.Changes[3].X != 'R' {
		t.Fatalf("rename %+v", st.Changes[3])
	}
	if !st.Changes[4].Unmerged || st.Changes[4].Path != "conflict.go" {
		t.Fatalf("conflict %+v", st.Changes[4])
	}
	if st.Changes[5].Path != "new file.txt" || !st.Changes[5].Unstaged() || st.Changes[5].Staged() {
		t.Fatalf("untracked %+v", st.Changes[5])
	}
}

func TestParseStatusDetached(t *testing.T) {
	st, err := ParseStatus([]byte("# branch.oid abc\x00# branch.head (detached)\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Detached || st.Head != "" {
		t.Fatalf("%+v", st)
	}
}

func TestParseLogAndRefs(t *testing.T) {
	when := time.Unix(1_700_000_000, 0).Unix()
	rec := strings.Join([]string{
		"aaa",
		"bbb ccc",
		"Ada",
		"ada@example.com",
		strconv.FormatInt(when, 10),
		"Add graph",
		"HEAD -> main, origin/main, tag: v1",
	}, "\x1f")
	commits, more := ParseLog([]byte("\x1e"+rec), 10)
	if more || len(commits) != 1 {
		t.Fatalf("commits %d more %v", len(commits), more)
	}
	c := commits[0]
	if c.Subject != "Add graph" || !c.Head || c.When.Unix() != when {
		t.Fatalf("%+v", c)
	}
	if len(c.Parents) != 2 || c.Parents[0] != "bbb" || c.Parents[1] != "ccc" {
		t.Fatalf("parents %v", c.Parents)
	}
	if strings.Join(c.Refs, ",") != "main,origin/main,v1" {
		t.Fatalf("refs %v", c.Refs)
	}

	refs := ParseRefs([]byte(strings.Join([]string{
		"refs/heads/main\tabc\t*\torigin/main",
		"refs/remotes/origin/HEAD\tdef\t ",
		"refs/remotes/origin/main\tabc\t ",
		"refs/tags/v1\tzzz\t ",
	}, "\n") + "\n"))
	if len(refs) != 3 {
		t.Fatalf("refs %+v", refs)
	}
	if refs[0].Name != "main" || !refs[0].Current || refs[0].Upstream != "origin/main" || refs[0].Kind != "branch" {
		t.Fatalf("branch %+v", refs[0])
	}
	if refs[1].Kind != "remote" || refs[1].Name != "origin/main" {
		t.Fatalf("remote %+v", refs[1])
	}
	if refs[2].Kind != "tag" || refs[2].Name != "v1" {
		t.Fatalf("tag %+v", refs[2])
	}

	stashes := ParseStashes([]byte("stash@{0}\x1fOn main: WIP\nstash@{1}\x1fOn main: older\n"))
	if len(stashes) != 2 || stashes[0].ID != "stash@{0}" || stashes[1].Subject != "On main: older" {
		t.Fatalf("%+v", stashes)
	}
}

func TestParseNameStatus(t *testing.T) {
	raw := strings.Join([]string{"M", "a.go", "A", "b.go", "R100", "old.go", "new.go", "D", "gone.go"}, "\x00") + "\x00"
	files, err := ParseNameStatus([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Fatalf("%+v", files)
	}
	if files[2].Letter() != "R" || files[2].Orig != "old.go" || files[2].Path != "new.go" {
		t.Fatalf("rename %+v", files[2])
	}
}

func TestLayout(t *testing.T) {
	linear := []Commit{
		{Hash: "a", Parents: []string{"b"}},
		{Hash: "b", Parents: []string{"c"}},
		{Hash: "c"},
	}
	Layout(linear)
	if linear[0].Col != 0 || linear[0].FromAbove || len(linear[0].Downs) != 1 || linear[0].Downs[0] != 0 {
		t.Fatalf("tip %+v", linear[0])
	}
	if !linear[1].FromAbove || linear[1].Col != 0 || linear[2].Col != 0 || len(linear[2].Downs) != 0 {
		t.Fatalf("linear %+v", linear)
	}
	if GraphWidth(linear) != 1 {
		t.Fatal(GraphWidth(linear))
	}

	// A merges B and C, both of which come from D.
	merge := []Commit{
		{Hash: "a", Parents: []string{"b", "c"}},
		{Hash: "b", Parents: []string{"d"}},
		{Hash: "c", Parents: []string{"d"}},
		{Hash: "d"},
	}
	Layout(merge)
	if merge[0].Col != 0 || len(merge[0].Downs) != 2 || merge[0].Downs[0] != 0 || merge[0].Downs[1] != 1 {
		t.Fatalf("merge tip %+v", merge[0])
	}
	if merge[1].Col != 0 || !merge[1].FromAbove || !contains(merge[1].Passing, 1) {
		t.Fatalf("b %+v", merge[1])
	}
	if merge[2].Col != 1 || !merge[2].FromAbove || len(merge[2].Downs) != 1 || merge[2].Downs[0] != 0 {
		t.Fatalf("c %+v", merge[2])
	}
	if merge[3].Col != 0 || !merge[3].FromAbove {
		t.Fatalf("d %+v", merge[3])
	}
	if GraphWidth(merge) != 2 {
		t.Fatal(GraphWidth(merge))
	}
}

func TestFolderName(t *testing.T) {
	if got := FolderName("https://github.com/karolsko/gg.git"); got != "gg" {
		t.Fatal(got)
	}
	if got := FolderName("git@github.com:karolsko/gg.git"); got != "gg" {
		t.Fatal(got)
	}
	if got := LocalName("origin/feature/login"); got != "feature/login" {
		t.Fatal(got)
	}
}

func contains(ns []int, n int) bool {
	for _, v := range ns {
		if v == n {
			return true
		}
	}
	return false
}

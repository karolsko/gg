package gitx

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseStatus reads `git status --porcelain=v2 --branch -z`.
func ParseStatus(out []byte) (Status, error) {
	recs := bytes.Split(bytes.TrimRight(out, "\x00"), []byte{0})
	var st Status
	for i := 0; i < len(recs); i++ {
		rec := string(recs[i])
		if rec == "" {
			continue
		}
		if strings.HasPrefix(rec, "# ") {
			parseStatusHeader(&st, rec)
			continue
		}
		switch rec[0] {
		case '1':
			ch, err := parseOrdinary(rec)
			if err != nil {
				return Status{}, err
			}
			st.Changes = append(st.Changes, ch)
		case '2':
			if i+1 >= len(recs) {
				return Status{}, errf("rename record without the original path")
			}
			i++
			ch, err := parseRename(rec, string(recs[i]))
			if err != nil {
				return Status{}, err
			}
			st.Changes = append(st.Changes, ch)
		case 'u':
			ch, err := parseUnmerged(rec)
			if err != nil {
				return Status{}, err
			}
			st.Changes = append(st.Changes, ch)
		case '?':
			path := strings.TrimPrefix(rec, "? ")
			if path == "" || path == rec {
				continue
			}
			st.Changes = append(st.Changes, Change{Path: path, X: '?'})
		case '!':
			// Ignored files stay out of the lists.
		default:
			return Status{}, errf("unknown status record %q", rec)
		}
	}
	return st, nil
}

func parseStatusHeader(st *Status, rec string) {
	body := strings.TrimPrefix(rec, "# ")
	key, val, ok := strings.Cut(body, " ")
	if !ok {
		return
	}
	switch key {
	case "branch.oid":
		st.OID = val
	case "branch.head":
		if val == "(detached)" {
			st.Detached = true
			return
		}
		st.Head = val
	case "branch.upstream":
		st.Upstream = val
	case "branch.ab":
		ahead, behind, ok := parseAheadBehind(val)
		if ok {
			st.Ahead, st.Behind, st.HasRemote = ahead, behind, true
		}
	}
}

func parseAheadBehind(val string) (ahead, behind int, ok bool) {
	parts := strings.Fields(val)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "+") || !strings.HasPrefix(parts[1], "-") {
		return 0, 0, false
	}
	a, errA := strconv.Atoi(parts[0][1:])
	b, errB := strconv.Atoi(parts[1][1:])
	if errA != nil || errB != nil {
		return 0, 0, false
	}
	return a, b, true
}

func parseOrdinary(rec string) (Change, error) {
	f := strings.SplitN(rec, " ", 9)
	if len(f) != 9 || len(f[1]) < 2 {
		return Change{}, errf("ordinary status record %q", rec)
	}
	return Change{Path: f[8], X: f[1][0], Y: f[1][1]}, nil
}

func parseRename(rec, orig string) (Change, error) {
	f := strings.SplitN(rec, " ", 10)
	if len(f) != 10 || len(f[1]) < 2 || orig == "" {
		return Change{}, errf("rename status record %q", rec)
	}
	return Change{Path: f[9], Orig: orig, X: f[1][0], Y: f[1][1]}, nil
}

func parseUnmerged(rec string) (Change, error) {
	f := strings.SplitN(rec, " ", 11)
	if len(f) != 11 || len(f[1]) < 2 {
		return Change{}, errf("unmerged status record %q", rec)
	}
	return Change{Path: f[10], X: f[1][0], Y: f[1][1], Unmerged: true}, nil
}

const logFormat = "%x1e%H%x1f%P%x1f%an%x1f%ae%x1f%at%x1f%s%x1f%D"

// ParseLog reads `git log --format=` with logFormat. limit is how many
// commits the caller asked to keep; a longer result sets more.
func ParseLog(out []byte, limit int) (commits []Commit, more bool) {
	raw := strings.Split(string(out), "\x1e")
	for _, rec := range raw {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		c, ok := parseCommit(rec)
		if !ok {
			continue
		}
		commits = append(commits, c)
	}
	if limit > 0 && len(commits) > limit {
		more = true
		commits = commits[:limit]
	}
	Layout(commits)
	return commits, more
}

func parseCommit(rec string) (Commit, bool) {
	f := strings.Split(rec, "\x1f")
	if len(f) < 6 {
		return Commit{}, false
	}
	sec, err := strconv.ParseInt(f[4], 10, 64)
	if err != nil {
		return Commit{}, false
	}
	c := Commit{
		Hash:    f[0],
		Author:  f[2],
		Email:   f[3],
		When:    time.Unix(sec, 0),
		Subject: f[5],
	}
	if f[1] != "" {
		c.Parents = strings.Fields(f[1])
	}
	if len(f) > 6 {
		c.Head, c.Refs = parseDecoration(f[6])
	}
	return c, c.Hash != ""
}

func parseDecoration(d string) (head bool, refs []string) {
	d = strings.TrimSpace(d)
	if d == "" {
		return false, nil
	}
	for _, part := range strings.Split(d, ", ") {
		part = strings.TrimSpace(part)
		switch {
		case part == "HEAD":
			head = true
		case strings.HasPrefix(part, "HEAD -> "):
			head = true
			refs = append(refs, strings.TrimPrefix(part, "HEAD -> "))
		case strings.HasPrefix(part, "tag: "):
			refs = append(refs, strings.TrimPrefix(part, "tag: "))
		case part != "":
			refs = append(refs, part)
		}
	}
	return head, refs
}

// ParseNameStatus reads `git diff --name-status -z` output.
// Renames and copies are status, old path, new path.
func ParseNameStatus(out []byte) ([]PathChange, error) {
	recs := bytes.Split(bytes.TrimRight(out, "\x00"), []byte{0})
	var files []PathChange
	for i := 0; i < len(recs); i++ {
		st := string(recs[i])
		if st == "" {
			continue
		}
		if i+1 >= len(recs) {
			return nil, errf("name-status record %q without a path", st)
		}
		i++
		if st[0] == 'R' || st[0] == 'C' {
			if i+1 >= len(recs) {
				return nil, errf("rename name-status without the new path")
			}
			orig := string(recs[i])
			i++
			files = append(files, PathChange{Status: st, Orig: orig, Path: string(recs[i])})
			continue
		}
		files = append(files, PathChange{Status: st, Path: string(recs[i])})
	}
	return files, nil
}

// ParseRefs reads `git for-each-ref` lines of
// name, hash, HEAD mark, upstream, separated by tabs.
func ParseRefs(out []byte) []Ref {
	recs := bytes.Split(bytes.TrimRight(out, "\n"), []byte{'\n'})
	var refs []Ref
	for _, rec := range recs {
		if len(rec) == 0 {
			continue
		}
		f := strings.Split(string(rec), "\t")
		if len(f) < 3 {
			continue
		}
		full := f[0]
		if strings.HasSuffix(full, "/HEAD") && strings.HasPrefix(full, "refs/remotes/") {
			continue
		}
		ref := Ref{Hash: f[1], Current: f[2] == "*"}
		if len(f) > 3 {
			ref.Upstream = f[3]
		}
		switch {
		case strings.HasPrefix(full, "refs/heads/"):
			ref.Kind = "branch"
			ref.Name = strings.TrimPrefix(full, "refs/heads/")
		case strings.HasPrefix(full, "refs/remotes/"):
			ref.Kind = "remote"
			ref.Name = strings.TrimPrefix(full, "refs/remotes/")
		case strings.HasPrefix(full, "refs/tags/"):
			ref.Kind = "tag"
			ref.Name = strings.TrimPrefix(full, "refs/tags/")
		default:
			continue
		}
		refs = append(refs, ref)
	}
	return refs
}

// ParseStashes reads `git stash list` lines of id, subject separated by \x1f.
func ParseStashes(out []byte) []Stash {
	var stashes []Stash
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		id, subject, ok := strings.Cut(line, "\x1f")
		if !ok {
			continue
		}
		stashes = append(stashes, Stash{ID: id, Subject: subject})
	}
	return stashes
}

func errf(format string, args ...any) error {
	return &parseError{msg: fmt.Sprintf(format, args...)}
}

type parseError struct{ msg string }

func (e *parseError) Error() string { return "gitx: " + e.msg }

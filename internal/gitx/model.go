// Package gitx talks to the git installed on the machine. The system git
// keeps the user's SSH keys, credential helper, hooks and config, which a
// reimplementation would have to reproduce.
package gitx

import "time"

// Status is the branch header and the working tree of one repository.
type Status struct {
	OID       string
	Head      string
	Detached  bool
	Upstream  string
	Ahead     int
	Behind    int
	HasRemote bool
	Changes   []Change
}

// Change is one path from git status. X is the index status and Y the
// worktree status, the same letters porcelain v2 uses. Untracked files
// use X '?' . Orig is set for renames and copies, the name before.
type Change struct {
	Path     string
	Orig     string
	X, Y     byte
	Unmerged bool
}

// Staged reports a change already in the index.
func (c Change) Staged() bool {
	if c.Unmerged || c.X == 0 || c.X == '.' || c.X == '?' {
		return false
	}
	return true
}

// Unstaged reports a change in the worktree, including untracked and unmerged files.
func (c Change) Unstaged() bool {
	if c.Unmerged || c.X == '?' {
		return true
	}
	return c.Y != 0 && c.Y != '.'
}

// Ref is a local branch, a remote-tracking branch or a tag.
type Ref struct {
	Name     string
	Hash     string
	Kind     string // "branch", "remote" or "tag"
	Current  bool
	Upstream string
}

// Stash is one entry of git stash list.
type Stash struct {
	ID      string // stash@{0}
	Subject string
}

// Commit is one row of history, plus the lane layout of the graph.
type Commit struct {
	Hash    string
	Parents []string
	Author  string
	Email   string
	When    time.Time
	Subject string
	Refs    []string
	Head    bool

	// Col is the lane of the commit. FromAbove is a line coming into it
	// from a newer commit. Passing lanes draw a vertical line through the
	// row. Downs are the lanes the commit's parents leave through, the
	// first parent first.
	Col       int
	FromAbove bool
	Passing   []int
	Downs     []int
}

// PathChange is one file in a commit or a stash.
type PathChange struct {
	Status string
	Path   string
	Orig   string
}

// Letter is the short status shown beside a path.
func (p PathChange) Letter() string {
	if p.Status == "" {
		return "?"
	}
	return p.Status[:1]
}

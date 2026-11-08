package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const maxFileBytes = 512 * 1024

// Find returns the working tree that contains path.
func Find(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("choose a folder")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	out, err := output(context.Background(), abs, false, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", errors.New("not a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// ReadStatus runs git status.
func ReadStatus(ctx context.Context, dir string) (Status, error) {
	out, err := output(ctx, dir, false, "status", "--porcelain=v2", "--branch", "-z", "-uall")
	if err != nil {
		return Status{}, err
	}
	return ParseStatus(out)
}

// ReadRefs lists branches, remote-tracking branches and tags.
func ReadRefs(ctx context.Context, dir string) ([]Ref, error) {
	// for-each-ref in current Git has no -z. Ref names cannot contain tabs
	// or newlines, so a line per ref is unambiguous.
	out, err := output(ctx, dir, false, "for-each-ref",
		"--format=%(refname)\t%(objectname)\t%(HEAD)\t%(upstream:short)",
		"refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return nil, err
	}
	return ParseRefs(out), nil
}

// ReadStashes lists stashes, newest first.
func ReadStashes(ctx context.Context, dir string) ([]Stash, error) {
	out, err := output(ctx, dir, false, "stash", "list", "--format=%gd%x1f%gs")
	if err != nil {
		return nil, err
	}
	return ParseStashes(out), nil
}

// ReadLog reads up to limit commits. all ignores rev and passes --all.
// more is true when older commits were left out.
func ReadLog(ctx context.Context, dir, rev string, all bool, skip, limit int) ([]Commit, bool, error) {
	if limit <= 0 {
		limit = 200
	}
	args := []string{
		"log", "--date-order", "--decorate=short",
		"-n", strconv.Itoa(limit + 1),
		"--skip", strconv.Itoa(skip),
		"--format=" + logFormat,
	}
	if all {
		args = append(args, "--all")
	} else {
		if rev == "" {
			rev = "HEAD"
		}
		args = append(args, rev)
	}
	out, err := output(ctx, dir, false, args...)
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits") {
			return nil, false, nil
		}
		return nil, false, err
	}
	commits, more := ParseLog(out, limit)
	return commits, more, nil
}

// CommitFiles lists the files a commit changed against its first parent.
func CommitFiles(ctx context.Context, dir, hash string) ([]PathChange, error) {
	out, err := output(ctx, dir, true, "diff-tree", "-m", "--first-parent", "-M", "-r",
		"--name-status", "--no-commit-id", "-z", "--root", hash)
	if err != nil {
		return nil, err
	}
	return ParseNameStatus(out)
}

// StashFiles lists the files in a stash.
func StashFiles(ctx context.Context, dir, id string) ([]PathChange, error) {
	out, err := output(ctx, dir, true, "diff", "-M", "--name-status", "-z", id+"^", id)
	if err != nil {
		return nil, err
	}
	return ParseNameStatus(out)
}

// DiffWorktree is the unified diff of one path. staged reads the index.
func DiffWorktree(ctx context.Context, dir, path string, staged bool) (string, error) {
	args := []string{"diff", "-U3", "--no-ext-diff", "--no-color"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", path)
	out, err := output(ctx, dir, true, args...)
	return string(out), err
}

// DiffCommitFile is the unified diff of one path inside a commit.
func DiffCommitFile(ctx context.Context, dir, hash, path string) (string, error) {
	out, err := output(ctx, dir, true, "diff", "-U3", "--no-ext-diff", "--no-color", hash+"^", hash, "--", path)
	if err == nil {
		return string(out), nil
	}
	out, err = output(ctx, dir, true, "show", "--format=", "--no-ext-diff", "--no-color", "-U3", hash, "--", path)
	return string(out), err
}

// DiffStashFile is the unified diff of one path in a stash.
func DiffStashFile(ctx context.Context, dir, id, path string) (string, error) {
	out, err := output(ctx, dir, true, "diff", "-U3", "--no-ext-diff", "--no-color", id+"^", id, "--", path)
	return string(out), err
}

// ReadWorktreeFile reads an untracked file for a diff against an empty old side.
func ReadWorktreeFile(dir, path string) (text string, binary bool, err error) {
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
	if err != nil {
		return "", false, err
	}
	if bytes.IndexByte(b, 0) >= 0 {
		return "", true, nil
	}
	if len(b) > maxFileBytes {
		return "", false, fmt.Errorf("file is larger than %dKB", maxFileBytes/1024)
	}
	return string(b), false, nil
}

// Stage adds paths. No paths stages everything.
func Stage(ctx context.Context, dir string, paths ...string) error {
	if len(paths) == 0 {
		_, err := output(ctx, dir, false, "add", "-A")
		return err
	}
	_, err := output(ctx, dir, false, append([]string{"add", "--"}, paths...)...)
	return err
}

// Unstage removes paths from the index. No paths unstages everything.
func Unstage(ctx context.Context, dir string, paths ...string) error {
	args := []string{"restore", "--staged"}
	if len(paths) == 0 {
		args = append(args, ".")
	} else {
		args = append(append(args, "--"), paths...)
	}
	_, err := output(ctx, dir, false, args...)
	return err
}

// RestoreWorktree throws away tracked changes to path, in the index and the worktree.
func RestoreWorktree(ctx context.Context, dir, path string) error {
	_, err := output(ctx, dir, false, "restore", "--source=HEAD", "--staged", "--worktree", "--", path)
	return err
}

// RemoveUntracked deletes an untracked file inside the working tree.
func RemoveUntracked(dir, path string) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	full, err = filepath.Abs(full)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("path is outside the repository")
	}
	return os.Remove(full)
}

// CreateCommit records the index with message.
func CreateCommit(ctx context.Context, dir, message string) error {
	_, err := outputIn(ctx, dir, message, false, "commit", "-F", "-")
	return err
}

// SwitchBranch checks out a local branch.
func SwitchBranch(ctx context.Context, dir, name string) error {
	_, err := output(ctx, dir, false, "switch", "--", name)
	return err
}

// SwitchRemote checks out a remote-tracking branch, creating the local
// branch of the same short name when it does not exist yet.
func SwitchRemote(ctx context.Context, dir, remoteRef string) error {
	local := LocalName(remoteRef)
	if _, err := output(ctx, dir, false, "show-ref", "--verify", "--quiet", "refs/heads/"+local); err == nil {
		_, err = output(ctx, dir, false, "switch", "--", local)
		return err
	}
	_, err := output(ctx, dir, false, "switch", "-c", local, "--track", remoteRef)
	return err
}

// SwitchTag detaches HEAD at a tag.
func SwitchTag(ctx context.Context, dir, name string) error {
	_, err := output(ctx, dir, false, "switch", "--detach", name)
	return err
}

// Merge merges ref into HEAD.
func Merge(ctx context.Context, dir, ref string) error {
	_, err := output(ctx, dir, false, "merge", "--no-edit", ref)
	return err
}

// DeleteBranch deletes a merged local branch.
func DeleteBranch(ctx context.Context, dir, name string) error {
	_, err := output(ctx, dir, false, "branch", "-d", name)
	return err
}

// Fetch fetches every remote and prunes remote-tracking branches that are gone.
func Fetch(ctx context.Context, dir string) error {
	_, err := output(ctx, dir, false, "fetch", "--all", "--prune")
	return err
}

// Pull pulls the current branch.
func Pull(ctx context.Context, dir string) error {
	_, err := output(ctx, dir, false, "pull")
	return err
}

// Push pushes the current branch.
func Push(ctx context.Context, dir string) error {
	_, err := output(ctx, dir, false, "push")
	return err
}

// PushUpstream publishes HEAD to remote and sets the upstream.
func PushUpstream(ctx context.Context, dir, remote string) error {
	_, err := output(ctx, dir, false, "push", "-u", remote, "HEAD")
	return err
}

// StashPush stashes tracked and untracked changes.
func StashPush(ctx context.Context, dir, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "WIP"
	}
	_, err := output(ctx, dir, false, "stash", "push", "-u", "-m", message)
	return err
}

// StashPop applies a stash and drops it. apply leaves it in the list.
func StashPop(ctx context.Context, dir, id string, pop bool) error {
	verb := "apply"
	if pop {
		verb = "pop"
	}
	_, err := output(ctx, dir, false, "stash", verb, id)
	return err
}

// Clone clones url into dest.
func Clone(ctx context.Context, url, dest string) error {
	_, err := output(ctx, "", false, "clone", "--", url, dest)
	return err
}

// Init creates a repository at dir.
func Init(ctx context.Context, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	_, err := output(ctx, dir, false, "init")
	return err
}

// FirstRemote returns origin when it exists, otherwise the first remote.
func FirstRemote(ctx context.Context, dir string) (string, error) {
	out, err := output(ctx, dir, false, "remote")
	if err != nil {
		return "", err
	}
	names := strings.Fields(string(out))
	if len(names) == 0 {
		return "", nil
	}
	for _, name := range names {
		if name == "origin" {
			return name, nil
		}
	}
	return names[0], nil
}

// LocalName is the local branch name for a remote-tracking ref.
// origin/feature/login becomes feature/login.
func LocalName(remoteRef string) string {
	if i := strings.Index(remoteRef, "/"); i >= 0 && i < len(remoteRef)-1 {
		return remoteRef[i+1:]
	}
	return remoteRef
}

// FolderName is the directory name implied by a clone URL.
func FolderName(url string) string {
	base := strings.TrimSpace(url)
	base = strings.TrimSuffix(base, "/")
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.LastIndex(base, ":"); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".git")
	if base == "" || base == "." || base == ".." {
		return "repository"
	}
	return base
}

func output(ctx context.Context, dir string, ok1 bool, args ...string) ([]byte, error) {
	return outputIn(ctx, dir, "", ok1, args...)
}

func outputIn(ctx context.Context, dir, stdin string, ok1 bool, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), nil
	}
	if errors.Is(err, context.Canceled) {
		return nil, context.Canceled
	}
	var exit *exec.ExitError
	if ok1 && errors.As(err, &exit) && exit.ExitCode() == 1 {
		return stdout.Bytes(), nil
	}
	msg := strings.TrimSpace(stderr.String())
	if msg == "" {
		msg = strings.TrimSpace(stdout.String())
	}
	if msg == "" {
		msg = err.Error()
	}
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	if len(msg) > 400 {
		msg = msg[:400]
	}
	return stdout.Bytes(), errors.New(msg)
}

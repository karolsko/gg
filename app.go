package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/karolsko/gg/internal/gitx"
)

type app struct {
	win *mygo.Window

	reposW      float32
	sidebarW    float32
	changesW    float32
	histPercent float32
	diffMode    int
	logLimit    int

	recents  []string
	lastRepo string

	repoPath string
	repoName string

	status  gitx.Status
	refs    []gitx.Ref
	stashes []gitx.Stash
	commits []gitx.Commit
	more    bool
	files   []gitx.PathChange

	unstaged []gitx.Change
	staged   []gitx.Change

	nav          string
	filter       string
	branchesOpen bool
	remotesOpen  bool
	tagsOpen     bool
	stashesOpen  bool

	unstagedSel  int
	stagedSel    int
	histSel      int
	fileSel      int
	changePath   string
	changeSide   string
	filePath     string
	selectedHash string
	filesFor     string

	message string

	diff      *editor.DiffState
	diffNote  string
	diffPath  string
	diffKey   string
	diffSeq   int
	lastPatch string

	busy   bool
	op     string
	notice string

	gen int

	cloneOpen     bool
	cloneURL      string
	cloneDir      string
	cloneName     string
	cloneNameAuto bool

	discardOpen      bool
	discardPath      string
	discardUntracked bool

	deleteOpen bool
	deleteName string

	mergeOpen bool
	mergeName string

	unstagedList ui.ListState
	stagedList   ui.ListState
	histList     ui.ListState
	fileList     ui.ListState

	watchCancel context.CancelFunc
	opCancel    context.CancelFunc
	opMu        sync.Mutex
}

func newApp() *app {
	home, _ := os.UserHomeDir()
	return &app{
		reposW:        220,
		sidebarW:      248,
		changesW:      340,
		histPercent:   62,
		logLimit:      200,
		cloneDir:      home,
		cloneNameAuto: true,
		branchesOpen:  true,
		remotesOpen:   true,
		stashesOpen:   true,
		unstagedSel:   -1,
		stagedSel:     -1,
		histSel:       -1,
		fileSel:       -1,
	}
}

func (a *app) view(c *ui.Context) {
	a.shortcuts(c)
	ui.Split(c, &a.reposW, func() { a.repoList(c) }, func() {
		if a.repoPath == "" {
			a.welcome(c)
		} else {
			a.shell(c)
		}
	}).SplitReset(220).Fill().OnChange(a.savePrefs)
	a.dialogs(c)
	a.flush(c)
}

func (a *app) shortcuts(c *ui.Context) {
	c.OnWindowShortcut(ui.Cmd, ui.KeyEnter, func() {
		if a.nav == "changes" {
			a.commit()
		}
	})
}

func (a *app) flush(c *ui.Context) {
	if a.notice == "" {
		return
	}
	c.Toast(a.notice)
	a.notice = ""
}

func (a *app) toast(msg string) {
	if msg == "" {
		return
	}
	a.notice = msg
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *app) post(fn func()) {
	if a.win == nil {
		fn()
		return
	}
	a.win.Update(fn)
}

func (a *app) requireRepo() bool {
	if a.repoPath != "" {
		return true
	}
	a.toast("Open a repository first")
	return false
}

func (a *app) openRepository() {
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    win,
			Title:     "Open Repository",
			Directory: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.openPath(paths[0])
	}()
}

func (a *app) newRepository() {
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            win,
			Title:             "New Repository",
			Directory:         true,
			CreateDirectories: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		dir := paths[0]
		if err := gitx.Init(context.Background(), dir); err != nil {
			a.post(func() { a.toast(err.Error()) })
			return
		}
		a.post(func() { a.setRepo(dir) })
	}()
}

func (a *app) openPath(p string) {
	go func() {
		repo, err := gitx.Find(p)
		a.post(func() {
			if err != nil {
				a.toast(err.Error())
				return
			}
			a.setRepo(repo)
		})
	}()
}

func (a *app) setRepo(path string) {
	if a.watchCancel != nil {
		a.watchCancel()
		a.watchCancel = nil
	}
	a.repoPath = path
	a.repoName = filepath.Base(path)
	a.nav = ""
	a.commits = nil
	a.files = nil
	a.filesFor = ""
	a.unstaged, a.staged = nil, nil
	a.selectedHash = ""
	a.changePath, a.changeSide, a.filePath = "", "", ""
	a.unstagedSel, a.stagedSel, a.histSel, a.fileSel = -1, -1, -1, -1
	a.diff = nil
	a.diffKey = ""
	a.diffPath = ""
	a.diffSeq++
	a.lastPatch = ""
	a.message = ""
	a.logLimit = 200
	a.remember(path)
	a.watch(path)
	a.refresh()
	if a.win != nil {
		a.win.SetTitle(a.repoName)
	}
}

func (a *app) closeRepo() {
	if a.watchCancel != nil {
		a.watchCancel()
		a.watchCancel = nil
	}
	a.repoPath = ""
	a.repoName = ""
	a.nav = ""
	a.commits = nil
	a.files = nil
	a.diff = nil
	a.diffKey = ""
	a.busy = false
	a.op = ""
	a.gen++
	if a.win != nil {
		a.win.SetTitle("gg")
	}
}

func (a *app) watch(dir string) {
	ctx, cancel := context.WithCancel(context.Background())
	a.watchCancel = cancel
	go func() {
		w, err := mygo.WatchFiles(ctx, dir, mygo.FileWatchOptions{
			Recursive: true,
			HintsOnly: true,
			Ignore:    ignoreWatch,
		})
		if err != nil {
			return
		}
		defer w.Close()
		var timer *time.Timer
		defer func() {
			if timer != nil {
				timer.Stop()
			}
		}()
		for {
			if _, err := w.Next(ctx); err != nil {
				return
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(300*time.Millisecond, func() {
				a.post(func() {
					if a.repoPath == dir && !a.busy {
						a.refresh()
					}
				})
			})
		}
	}()
}

func ignoreWatch(p string, isDir bool) bool {
	if isDir {
		switch filepath.Base(p) {
		case "node_modules", "vendor", "dist", "build", ".mygo", "target", "DerivedData", "Pods":
			return true
		}
	}
	return strings.Contains(filepath.ToSlash(p), "/.git/objects")
}

func (a *app) refresh() {
	if a.repoPath == "" || a.busy {
		return
	}
	a.gen++
	gen := a.gen
	path := a.repoPath
	nav := a.nav
	limit := a.logLimit
	go func() {
		snap, err := loadSnapshot(context.Background(), path, nav, limit)
		a.post(func() {
			if gen != a.gen || path != a.repoPath {
				return
			}
			if err != nil {
				a.toast(err.Error())
				return
			}
			a.apply(snap)
		})
	}()
}

func (a *app) loadOlder() {
	if a.busy || !a.more {
		return
	}
	a.logLimit += 200
	a.refresh()
}

type snapshot struct {
	status  gitx.Status
	refs    []gitx.Ref
	stashes []gitx.Stash
	commits []gitx.Commit
	more    bool
}

func loadSnapshot(ctx context.Context, path, nav string, limit int) (snapshot, error) {
	var s snapshot
	var err error
	s.status, err = gitx.ReadStatus(ctx, path)
	if err != nil {
		return s, err
	}
	s.refs, err = gitx.ReadRefs(ctx, path)
	if err != nil {
		return s, err
	}
	s.stashes, err = gitx.ReadStashes(ctx, path)
	if err != nil {
		return s, err
	}
	rev, all := logRev(nav, s.status)
	s.commits, s.more, err = gitx.ReadLog(ctx, path, rev, all, 0, limit)
	return s, err
}

func logRev(nav string, st gitx.Status) (rev string, all bool) {
	switch {
	case nav == "all":
		return "", true
	case strings.HasPrefix(nav, "branch:"), strings.HasPrefix(nav, "remote:"), strings.HasPrefix(nav, "tag:"):
		return nav[strings.IndexByte(nav, ':')+1:], false
	default:
		if st.Detached && st.OID != "" && st.OID != "(initial)" {
			return st.OID, false
		}
		if st.Head != "" {
			return st.Head, false
		}
		return "HEAD", false
	}
}

func (a *app) apply(s snapshot) {
	a.status = s.status
	a.refs = s.refs
	a.stashes = s.stashes
	a.commits = s.commits
	a.more = s.more
	a.unstaged = a.unstaged[:0]
	a.staged = a.staged[:0]
	for _, ch := range s.status.Changes {
		if ch.Unstaged() {
			a.unstaged = append(a.unstaged, ch)
		}
		if ch.Staged() {
			a.staged = append(a.staged, ch)
		}
	}
	if a.nav == "" {
		if len(s.status.Changes) > 0 {
			a.nav = "changes"
		} else if s.status.Head != "" && !s.status.Detached {
			a.nav = "branch:" + s.status.Head
		} else {
			a.nav = "all"
		}
	}
	a.remapSelection()
	if a.win != nil {
		title := a.repoName
		if label := a.branchLabel(); label != "" {
			title += " — " + label
		}
		a.win.SetTitle(title)
	}
	if a.nav != "changes" {
		if a.filesFor != a.fileIdentity() {
			a.loadFiles()
			return
		}
	}
	a.queueDiff()
}

func (a *app) fileIdentity() string {
	if strings.HasPrefix(a.nav, "stash:") {
		return a.nav
	}
	return a.nav + ":" + a.selectedHash
}

func (a *app) remapSelection() {
	if a.selectedHash != "" {
		a.histSel = -1
		for i, c := range a.commits {
			if c.Hash == a.selectedHash {
				a.histSel = i
				break
			}
		}
	}
	if len(a.commits) == 0 {
		a.histSel = -1
		a.selectedHash = ""
	} else if a.histSel < 0 || a.histSel >= len(a.commits) {
		a.histSel = 0
		a.selectedHash = a.commits[0].Hash
	} else {
		a.selectedHash = a.commits[a.histSel].Hash
	}

	find := func(list []gitx.Change) int {
		for i, ch := range list {
			if ch.Path == a.changePath {
				return i
			}
		}
		return -1
	}
	u, s := find(a.unstaged), find(a.staged)
	switch {
	case a.changeSide == "staged" && s >= 0:
		a.stagedSel, a.unstagedSel = s, -1
	case u >= 0:
		a.unstagedSel, a.stagedSel = u, -1
		a.changeSide = "unstaged"
	case s >= 0:
		a.stagedSel, a.unstagedSel = s, -1
		a.changeSide = "staged"
	case len(a.unstaged) > 0:
		a.unstagedSel, a.stagedSel = 0, -1
		a.changePath = a.unstaged[0].Path
		a.changeSide = "unstaged"
	case len(a.staged) > 0:
		a.stagedSel, a.unstagedSel = 0, -1
		a.changePath = a.staged[0].Path
		a.changeSide = "staged"
	default:
		a.unstagedSel, a.stagedSel = -1, -1
		a.changePath = ""
	}
}

func (a *app) selectNav(id string) {
	if a.nav == id || a.busy {
		return
	}
	oldRev, oldAll := logRev(a.nav, a.status)
	a.nav = id
	a.selectedHash = ""
	a.histSel = -1
	a.fileSel = -1
	a.filePath = ""
	a.files = nil
	a.filesFor = ""
	if id == "changes" {
		a.queueDiff()
		return
	}
	newRev, newAll := logRev(a.nav, a.status)
	if strings.HasPrefix(id, "stash:") || oldRev != newRev || oldAll != newAll {
		a.refresh()
		return
	}
	a.remapSelection()
	a.loadFiles()
}

func (a *app) loadFiles() {
	id := a.fileIdentity()
	path := a.repoPath
	nav := a.nav
	hash := a.selectedHash
	keep := a.filePath
	go func() {
		var files []gitx.PathChange
		var err error
		ctx := context.Background()
		switch {
		case strings.HasPrefix(nav, "stash:"):
			files, err = gitx.StashFiles(ctx, path, strings.TrimPrefix(nav, "stash:"))
		case hash != "":
			files, err = gitx.CommitFiles(ctx, path, hash)
		}
		a.post(func() {
			if path != a.repoPath || nav != a.nav || hash != a.selectedHash {
				return
			}
			if err != nil {
				a.files = nil
				a.filesFor = id
				a.diff = nil
				a.diffNote = err.Error()
				return
			}
			a.files = files
			a.filesFor = id
			a.fileSel = -1
			for i, f := range files {
				if f.Path == keep {
					a.fileSel = i
					a.filePath = f.Path
					break
				}
			}
			if a.fileSel < 0 && len(files) > 0 {
				a.fileSel = 0
				a.filePath = files[0].Path
			}
			a.queueDiff()
		})
	}()
}

func (a *app) onUnstaged() {
	if a.unstagedSel < 0 || a.unstagedSel >= len(a.unstaged) {
		return
	}
	a.changePath = a.unstaged[a.unstagedSel].Path
	a.changeSide = "unstaged"
	a.stagedSel = -1
	a.queueDiff()
}

func (a *app) onStaged() {
	if a.stagedSel < 0 || a.stagedSel >= len(a.staged) {
		return
	}
	a.changePath = a.staged[a.stagedSel].Path
	a.changeSide = "staged"
	a.unstagedSel = -1
	a.queueDiff()
}

func (a *app) onHist() {
	if a.histSel < 0 || a.histSel >= len(a.commits) {
		return
	}
	hash := a.commits[a.histSel].Hash
	if hash == a.selectedHash {
		return
	}
	a.selectedHash = hash
	a.filePath = ""
	a.files = nil
	a.filesFor = ""
	a.loadFiles()
}

func (a *app) onFile() {
	if a.fileSel < 0 || a.fileSel >= len(a.files) {
		return
	}
	a.filePath = a.files[a.fileSel].Path
	a.queueDiff()
}

type diffRequest struct {
	key   string
	repo  string
	kind  string
	path  string
	hash  string
	stash string
}

func (a *app) queueDiff() {
	req, ok := a.currentDiff()
	if !ok {
		a.diff = nil
		a.diffKey = ""
		a.diffPath = ""
		a.diffSeq++
		a.diffNote = a.emptyDiffNote()
		return
	}
	a.diffPath = req.path
	if req.key != a.diffKey {
		a.diff = nil
		a.diffNote = "Loading…"
	}
	a.diffSeq++
	seq := a.diffSeq
	go func() {
		patch, oldText, newText, binary, err := fetchDiff(req)
		a.post(func() {
			if seq != a.diffSeq || a.repoPath != req.repo {
				return
			}
			if err != nil {
				a.diff = nil
				a.diffKey = req.key
				a.diffNote = err.Error()
				return
			}
			if binary {
				a.diff = nil
				a.diffKey = req.key
				a.diffNote = "Binary file"
				return
			}
			if patch == a.lastPatch && req.key == a.diffKey && a.diff != nil {
				a.diff.Layout = a.diffLayout()
				a.diffNote = ""
				return
			}
			a.lastPatch = patch
			a.diffKey = req.key
			a.setDiff(req.path, patch, oldText, newText)
		})
	}()
}

func (a *app) currentDiff() (diffRequest, bool) {
	if a.repoPath == "" {
		return diffRequest{}, false
	}
	if a.nav == "changes" {
		if a.changeSide == "staged" && a.stagedSel >= 0 && a.stagedSel < len(a.staged) {
			ch := a.staged[a.stagedSel]
			return diffRequest{key: "s:" + ch.Path, repo: a.repoPath, kind: "staged", path: ch.Path}, true
		}
		if a.unstagedSel >= 0 && a.unstagedSel < len(a.unstaged) {
			ch := a.unstaged[a.unstagedSel]
			kind := "unstaged"
			if ch.X == '?' {
				kind = "untracked"
			}
			return diffRequest{key: "u:" + ch.Path, repo: a.repoPath, kind: kind, path: ch.Path}, true
		}
		return diffRequest{}, false
	}
	if a.fileSel < 0 || a.fileSel >= len(a.files) {
		return diffRequest{}, false
	}
	f := a.files[a.fileSel]
	if strings.HasPrefix(a.nav, "stash:") {
		id := strings.TrimPrefix(a.nav, "stash:")
		return diffRequest{key: a.nav + ":" + f.Path, repo: a.repoPath, kind: "stash", path: f.Path, stash: id}, true
	}
	if a.selectedHash == "" {
		return diffRequest{}, false
	}
	return diffRequest{key: a.selectedHash + ":" + f.Path, repo: a.repoPath, kind: "commit", path: f.Path, hash: a.selectedHash}, true
}

func (a *app) emptyDiffNote() string {
	if a.nav == "changes" && len(a.status.Changes) == 0 {
		return "Working tree clean"
	}
	if a.nav == "changes" {
		return "Select a file"
	}
	if len(a.commits) == 0 && !strings.HasPrefix(a.nav, "stash:") {
		return "No commits yet"
	}
	return "Select a file"
}

func fetchDiff(req diffRequest) (patch, oldText, newText string, binary bool, err error) {
	ctx := context.Background()
	switch req.kind {
	case "untracked":
		text, bin, err := gitx.ReadWorktreeFile(req.repo, req.path)
		return "", "", text, bin, err
	case "staged":
		patch, err = gitx.DiffWorktree(ctx, req.repo, req.path, true)
	case "unstaged":
		patch, err = gitx.DiffWorktree(ctx, req.repo, req.path, false)
	case "commit":
		patch, err = gitx.DiffCommitFile(ctx, req.repo, req.hash, req.path)
	case "stash":
		patch, err = gitx.DiffStashFile(ctx, req.repo, req.stash, req.path)
	default:
		return "", "", "", false, errors.New("nothing to diff")
	}
	if err != nil {
		return "", "", "", false, err
	}
	if len(patch) > 1024*1024 {
		return "", "", "", false, errors.New("diff is larger than 1MB")
	}
	return patch, oldText, newText, false, nil
}

func (a *app) setDiff(filename, patch, oldText, newText string) {
	layout := a.diffLayout()
	if strings.TrimSpace(patch) != "" {
		files, err := editor.ParsePatch(patch)
		if err != nil {
			a.diff = nil
			a.diffNote = err.Error()
			return
		}
		if len(files) == 0 {
			a.diff = nil
			a.diffNote = "No changes"
			return
		}
		if files[0].Binary {
			a.diff = nil
			a.diffNote = "Binary file"
			return
		}
		d := &editor.DiffState{Layout: layout}
		d.SetPatch(files[0])
		a.diff = d
		a.diffNote = ""
		return
	}
	if oldText != "" || newText != "" {
		d := &editor.DiffState{
			Old:    editor.NewBuffer(oldText),
			New:    editor.NewBuffer(newText),
			Layout: layout,
		}
		if lang := editor.LanguageFor(filename); lang != nil {
			d.Old.SetLanguage(lang)
			d.New.SetLanguage(lang)
		}
		a.diff = d
		a.diffNote = ""
		return
	}
	a.diff = nil
	a.diffNote = "No changes"
}

func (a *app) diffLayout() editor.DiffLayout {
	if a.diffMode == 1 {
		return editor.DiffUnified
	}
	return editor.DiffSplit
}

func (a *app) branchLabel() string {
	if a.status.Detached {
		if len(a.status.OID) >= 7 && a.status.OID != "(initial)" {
			return "detached " + a.status.OID[:7]
		}
		return "detached"
	}
	if a.status.Head == "" {
		return ""
	}
	return a.status.Head
}

func (a *app) aheadBehind() string {
	if !a.status.HasRemote || (a.status.Ahead == 0 && a.status.Behind == 0) {
		return ""
	}
	return "↑" + strconv.Itoa(a.status.Ahead) + "  ↓" + strconv.Itoa(a.status.Behind)
}

func (a *app) refsOf(kind string) []gitx.Ref {
	var out []gitx.Ref
	for _, ref := range a.refs {
		if ref.Kind == kind && a.matches(ref.Name) {
			out = append(out, ref)
		}
	}
	return out
}

func (a *app) matches(name string) bool {
	f := strings.ToLower(strings.TrimSpace(a.filter))
	if f == "" {
		return true
	}
	return strings.Contains(strings.ToLower(name), f)
}

func (a *app) start(label string, fn func(context.Context) error, after func()) {
	if a.busy || a.repoPath == "" {
		return
	}
	a.busy = true
	a.op = label
	a.gen++
	ctx := a.nextOp()
	path := a.repoPath
	go func() {
		err := fn(ctx)
		a.post(func() {
			if a.repoPath != path {
				a.busy = false
				a.op = ""
				return
			}
			a.busy = false
			a.op = ""
			if errors.Is(err, context.Canceled) {
				return
			}
			if err != nil {
				a.toast(err.Error())
			} else if after != nil {
				after()
			}
			a.refresh()
		})
	}()
}

func (a *app) nextOp() context.Context {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	if a.opCancel != nil {
		a.opCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.opCancel = cancel
	return ctx
}

func (a *app) stage(paths ...string) {
	a.start("Staging…", func(ctx context.Context) error {
		return gitx.Stage(ctx, a.repoPath, paths...)
	}, nil)
}

func (a *app) unstage(paths ...string) {
	a.start("Unstaging…", func(ctx context.Context) error {
		return gitx.Unstage(ctx, a.repoPath, paths...)
	}, nil)
}

func (a *app) askDiscard(ch gitx.Change) {
	a.discardPath = ch.Path
	a.discardUntracked = ch.X == '?'
	a.discardOpen = true
}

func (a *app) discard() {
	path := a.discardPath
	untracked := a.discardUntracked
	repo := a.repoPath
	a.start("Discarding…", func(ctx context.Context) error {
		if untracked {
			return gitx.RemoveUntracked(repo, path)
		}
		return gitx.RestoreWorktree(ctx, repo, path)
	}, nil)
}

func (a *app) commit() {
	if !a.requireRepo() || a.nav != "changes" || a.busy {
		return
	}
	msg := strings.TrimSpace(a.message)
	if msg == "" || len(a.staged) == 0 {
		return
	}
	a.start("Committing…", func(ctx context.Context) error {
		return gitx.CreateCommit(ctx, a.repoPath, msg)
	}, func() { a.message = "" })
}

func (a *app) checkout(ref gitx.Ref) {
	nav := "branch:" + ref.Name
	if ref.Kind == "remote" {
		nav = "branch:" + gitx.LocalName(ref.Name)
	}
	if ref.Kind == "tag" {
		nav = "tag:" + ref.Name
	}
	a.start("Checking out "+ref.Name+"…", func(ctx context.Context) error {
		switch ref.Kind {
		case "remote":
			return gitx.SwitchRemote(ctx, a.repoPath, ref.Name)
		case "tag":
			return gitx.SwitchTag(ctx, a.repoPath, ref.Name)
		default:
			return gitx.SwitchBranch(ctx, a.repoPath, ref.Name)
		}
	}, func() {
		a.nav = nav
		a.selectedHash = ""
		a.filesFor = ""
	})
}

func (a *app) fetch() {
	if !a.requireRepo() {
		return
	}
	a.start("Fetching…", func(ctx context.Context) error {
		return gitx.Fetch(ctx, a.repoPath)
	}, func() { a.toast("Fetched") })
}

func (a *app) pull() {
	if !a.requireRepo() {
		return
	}
	a.start("Pulling…", func(ctx context.Context) error {
		return gitx.Pull(ctx, a.repoPath)
	}, func() { a.toast("Pulled") })
}

func (a *app) push() {
	if !a.requireRepo() {
		return
	}
	path := a.repoPath
	if a.status.Upstream != "" || a.status.Detached {
		a.start("Pushing…", func(ctx context.Context) error {
			return gitx.Push(ctx, path)
		}, func() { a.toast("Pushed") })
		return
	}
	a.busy = true
	a.op = "Pushing…"
	go func() {
		remote, err := gitx.FirstRemote(context.Background(), path)
		a.post(func() {
			if a.repoPath != path {
				a.busy = false
				a.op = ""
				return
			}
			a.busy = false
			a.op = ""
			if err != nil {
				a.toast(err.Error())
				return
			}
			if remote == "" {
				a.toast("This repository has no remote")
				return
			}
			a.start("Pushing…", func(ctx context.Context) error {
				return gitx.PushUpstream(ctx, path, remote)
			}, func() { a.toast("Pushed") })
		})
	}()
}

func (a *app) stash() {
	if !a.requireRepo() {
		return
	}
	msg := strings.TrimSpace(a.message)
	a.start("Stashing…", func(ctx context.Context) error {
		return gitx.StashPush(ctx, a.repoPath, msg)
	}, func() {
		a.message = ""
		a.toast("Stashed")
	})
}

func (a *app) stashPop(id string, pop bool) {
	a.start("Applying stash…", func(ctx context.Context) error {
		return gitx.StashPop(ctx, a.repoPath, id, pop)
	}, func() { a.nav = "changes" })
}

func (a *app) showClone() {
	a.cloneOpen = true
	if a.cloneNameAuto {
		a.cloneName = gitx.FolderName(a.cloneURL)
	}
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *app) doClone() {
	url := strings.TrimSpace(a.cloneURL)
	dir := strings.TrimSpace(a.cloneDir)
	name := strings.TrimSpace(a.cloneName)
	if name == "" {
		name = gitx.FolderName(url)
	}
	if url == "" || dir == "" {
		a.toast("Enter a URL and a parent folder")
		return
	}
	dest := filepath.Join(dir, name)
	a.cloneOpen = false
	a.busy = true
	a.op = "Cloning…"
	go func() {
		err := gitx.Clone(context.Background(), url, dest)
		a.post(func() {
			a.busy = false
			a.op = ""
			if err != nil {
				a.toast(err.Error())
				return
			}
			a.setRepo(dest)
		})
	}()
}

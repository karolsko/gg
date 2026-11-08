package main

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/karolsko/gg/internal/gitx"
)

func (a *app) changesPane(c *ui.Context) {
	ui.Split(c, &a.changesW, func() { a.changeLists(c) }, func() { a.diffPane(c) }).
		SplitReset(340).
		Fill().
		OnChange(a.savePrefs)
}

func (a *app) changeLists(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		a.fileHeader(c, "Unstaged", len(a.unstaged), "Stage All", len(a.unstaged) > 0, func() { a.stage() })
		a.unstagedList.Key = func(i int) any { return a.unstaged[i].Path }
		a.unstagedList.Label = func(i int) string { return a.unstaged[i].Path }
		a.unstagedList.Selected = &a.unstagedSel
		ui.List(c, &a.unstagedList, len(a.unstaged), func(i int) {
			a.changeRow(c, i, a.unstaged[i], false)
		}).Grow(1).Children(func() {
			if len(a.unstaged) == 0 {
				ui.Text(c, "Nothing unstaged").TextColor(t.TextMuted).Padding(12)
			}
		}).OnChange(a.onUnstaged)

		hairline(c)
		a.fileHeader(c, "Staged", len(a.staged), "Unstage All", len(a.staged) > 0, func() { a.unstage() })
		a.stagedList.Key = func(i int) any { return "s:" + a.staged[i].Path }
		a.stagedList.Label = func(i int) string { return a.staged[i].Path }
		a.stagedList.Selected = &a.stagedSel
		ui.List(c, &a.stagedList, len(a.staged), func(i int) {
			a.changeRow(c, i, a.staged[i], true)
		}).Grow(1).Children(func() {
			if len(a.staged) == 0 {
				ui.Text(c, "Nothing staged").TextColor(t.TextMuted).Padding(12)
			}
		}).OnChange(a.onStaged)

		hairline(c)
		ui.Column(c).Padding(8).Gap(8).Children(func() {
			ui.TextArea(c, &a.message).Placeholder("Commit message").Height(88).Label("Commit message")
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ready := !a.busy && strings.TrimSpace(a.message) != "" && len(a.staged) > 0
				ui.PrimaryButton(c, "Commit").Disabled(!ready).OnClick(a.commit)
				ui.Text(c, commitHint()).FontSize(12).TextColor(t.TextMuted)
			})
		})
	})
}

func (a *app) fileHeader(c *ui.Context, title string, n int, action string, enabled bool, fn func()) {
	t := c.Theme()
	ui.Row(c).Height(36).Padding(0, 10).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).Bold().Grow(1)
		ui.Text(c, strconv.Itoa(n)).TextColor(t.TextMuted)
		ui.Button(c, action).Disabled(!enabled || a.busy).OnClick(fn)
	})
}

func (a *app) changeRow(c *ui.Context, index int, ch gitx.Change, staged bool) {
	letter := string([]byte{ch.Y})
	if staged {
		letter = string([]byte{ch.X})
	}
	if ch.Unmerged {
		letter = "U"
	}
	if ch.X == '?' {
		letter = "?"
	}
	label := ch.Path
	if staged && ch.Orig != "" {
		label = ch.Orig + " → " + ch.Path
	}
	row := ui.Row(c).Padding(4, 8).Gap(8).AlignItems(ui.Center)
	row.Children(func() {
		ui.Text(c, letter).Font("monospace").Bold().Width(16).TextColor(statusColor(c, letter))
		ui.Text(c, label).MaxLines(1).Grow(1)
		action := "Stage"
		if staged {
			action = "Unstage"
		}
		ui.Button(c, action).Disabled(a.busy).OnClick(func() {
			a.rememberChange(index, ch.Path, staged)
			if staged {
				a.unstage(ch.Path)
				return
			}
			a.stage(ch.Path)
		})
	})
	row.ContextMenu(func(m *ui.Menu) {
		if staged {
			if m.Item("Unstage").Chosen() {
				a.rememberChange(index, ch.Path, true)
				a.unstage(ch.Path)
			}
		} else if m.Item("Stage").Chosen() {
			a.rememberChange(index, ch.Path, false)
			a.stage(ch.Path)
		}
		if m.Item("Discard").Danger().Chosen() {
			a.askDiscard(ch)
		}
		m.Separator()
		if m.Item("Copy Path").Chosen() {
			copyText(ch.Path)
		}
		if m.Item(revealLabel()).Chosen() && a.repoPath != "" {
			showPath(a.repoPath, ch.Path)
		}
	})
}

func (a *app) rememberChange(index int, path string, staged bool) {
	a.changePath = path
	if staged {
		a.changeSide = "staged"
		a.stagedSel = index
		a.unstagedSel = -1
		return
	}
	a.changeSide = "unstaged"
	a.unstagedSel = index
	a.stagedSel = -1
}

func copyText(s string) {
	mygo.Clipboard.WriteText(s)
}

func revealLabel() string {
	switch runtime.GOOS {
	case "darwin":
		return "Show in Finder"
	case "windows":
		return "Show in Explorer"
	default:
		return "Show in File Manager"
	}
}

func showPath(repo, path string) {
	mygo.Shell.ShowItemInFolder(filepath.Join(repo, filepath.FromSlash(path)))
}

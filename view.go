package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func (a *app) shell(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.toolbar(c)
		hairline(c)
		ui.Split(c, &a.sidebarW, func() { a.sidebar(c) }, func() { a.mainPane(c) }).
			SplitReset(248).
			Grow(1).
			FillWidth().
			OnChange(a.savePrefs)
		hairline(c)
		a.statusBar(c)
	})
}

func (a *app) mainPane(c *ui.Context) {
	switch {
	case a.nav == "changes":
		a.changesPane(c)
	case strings.HasPrefix(a.nav, "stash:"):
		a.stashPane(c)
	default:
		a.historyPane(c)
	}
}

func (a *app) toolbar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(48).Padding(0, 12).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.repoName).Bold()
		if label := a.branchLabel(); label != "" {
			ui.Text(c, label).TextColor(t.Accent).FontSize(13)
		}
		if ab := a.aheadBehind(); ab != "" {
			ui.Text(c, ab).TextColor(t.TextMuted).FontSize(12)
		}
		ui.Box(c).Grow(1)
		if a.busy {
			ui.Spinner(c)
			ui.Text(c, a.op).TextColor(t.TextMuted)
		}
		ui.Button(c, "Fetch").Disabled(a.busy).Tooltip("Download objects and refs from the remotes").OnClick(a.fetch)
		ui.Button(c, "Pull").Disabled(a.busy).Tooltip("Fetch and integrate the upstream branch").OnClick(a.pull)
		ui.Button(c, "Push").Disabled(a.busy).Tooltip("Update the remote branch").OnClick(a.push)
		ui.Button(c, "Stash").Disabled(a.busy).Tooltip("Stash working tree changes, including untracked files").OnClick(a.stash)
		ui.Button(c, "Refresh").Disabled(a.busy).OnClick(a.refresh)
	})
}

func (a *app) statusBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(28).Padding(0, 12).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.repoPath).TextColor(t.TextMuted).FontSize(12).MaxLines(1).Grow(1)
		if n := len(a.status.Changes); n > 0 {
			ui.Textf(c, "%d changed", n).TextColor(t.TextMuted).FontSize(12)
		} else if a.repoPath != "" {
			ui.Text(c, "Clean").TextColor(t.TextMuted).FontSize(12)
		}
	})
}

func hairline(c *ui.Context) {
	ui.Box(c).Height(1).FillWidth().Background(c.Theme().Border)
}

func (a *app) diffPane(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(40).Padding(0, 12).Gap(8).AlignItems(ui.Center).Children(func() {
			label := a.diffPath
			if label == "" {
				label = "Diff"
			}
			ui.Text(c, label).MaxLines(1).Grow(1)
			ui.Segmented(c, &a.diffMode, "Split", "Unified")
		})
		hairline(c)
		if a.diff != nil {
			a.diff.Layout = a.diffLayout()
			editor.DiffView(c, a.diff).Grow(1).FillWidth()
			return
		}
		ui.Column(c).Grow(1).Center().Children(func() {
			note := a.diffNote
			if note == "" {
				note = a.emptyDiffNote()
			}
			ui.Text(c, note).TextColor(t.TextMuted)
		})
	})
}

func relTime(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	if d < 14*24*time.Hour {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	return t.Format("2 Jan 2006")
}

func commitHint() string {
	if runtime.GOOS == "darwin" {
		return "⌘↩ to commit"
	}
	return "Ctrl+Enter to commit"
}

func statusColor(c *ui.Context, letter string) ui.Color {
	t := c.Theme()
	switch letter {
	case "A", "?":
		return t.Success
	case "D", "U":
		return t.Danger
	case "M", "R", "C", "T":
		return t.Warning
	default:
		return t.TextMuted
	}
}

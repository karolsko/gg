package main

import (
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

func (a *app) repoList(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		ui.Row(c).Height(48).Padding(0, 10).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Repositories").Bold().Grow(1)
			ui.Button(c, "Open").Disabled(a.busy).Tooltip("Open a repository and add it to this list").OnClick(a.openRepository)
		})
		hairline(c)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(6).Gap(2).Children(func() {
				if len(a.recents) == 0 {
					ui.Text(c, "Opened repositories show up here.").TextColor(t.TextMuted).FontSize(12).Padding(8)
					return
				}
				for _, path := range a.recents {
					a.repoRow(c, path)
				}
			})
		})
	})
}

func (a *app) repoRow(c *ui.Context, path string) {
	t := c.Theme()
	selected := path == a.repoPath
	row := ui.Column(c).Padding(8, 8).Radius(6).Gap(1)
	row.Cursor(ui.CursorPointer)
	if selected {
		row.Background(t.Accent)
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	nameColor := t.Text
	pathColor := t.TextMuted
	if selected {
		nameColor = t.AccentText
		pathColor = t.AccentText
	}
	row.Children(func() {
		ui.Text(c, filepath.Base(path)).MaxLines(1).TextColor(nameColor)
		ui.Text(c, path).FontSize(11).MaxLines(1).TextColor(pathColor)
	})
	row.OnClick(func() {
		if path != a.repoPath {
			a.openPath(path)
		}
	})
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("Open").Chosen() && path != a.repoPath {
			a.openPath(path)
		}
		if m.Item(revealLabel()).Chosen() {
			showPath(filepath.Dir(path), filepath.Base(path))
		}
		m.Separator()
		if m.Item("Remove from List").Danger().Chosen() {
			a.forget(path)
		}
	})
}

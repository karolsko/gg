package main

import (
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

func (a *app) welcome(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Column(c).Grow(1).Center().Padding(32).Children(func() {
			ui.Column(c).Width(440).Gap(16).Children(func() {
				ui.Text(c, "gg").FontSize(40).Bold()
				ui.Text(c, "A fast Git client for the working tree, the history and the diff.").
					TextColor(t.TextMuted)
				if a.busy {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Spinner(c)
						ui.Text(c, a.op).TextColor(t.TextMuted)
					})
				}
				ui.Row(c).Gap(8).Children(func() {
					ui.PrimaryButton(c, "Open Repository").Disabled(a.busy).OnClick(a.openRepository)
					ui.Button(c, "Clone").Disabled(a.busy).OnClick(a.showClone)
					ui.Button(c, "New").Disabled(a.busy).OnClick(a.newRepository)
				})
				if len(a.recents) > 0 {
					ui.Text(c, "Recent").Bold().Padding(8, 0, 0, 0)
					for _, path := range a.recents {
						a.recentRow(c, path)
					}
				}
			})
		})
	})
}

func (a *app) recentRow(c *ui.Context, path string) {
	t := c.Theme()
	row := ui.Column(c).Padding(8, 10).Radius(8).Gap(2)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Cursor(ui.CursorPointer)
	row.Children(func() {
		ui.Text(c, filepath.Base(path)).MaxLines(1)
		ui.Text(c, path).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
	})
	row.OnClick(func() { a.openPath(path) })
}

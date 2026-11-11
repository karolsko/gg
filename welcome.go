package main

import "github.com/egoist/mygo/ui"

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
			})
		})
	})
}

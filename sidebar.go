package main

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/karolsko/gg/internal/gitx"
)

func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		ui.Column(c).Padding(10, 10, 6, 10).Children(func() {
			ui.TextInput(c, &a.filter).Placeholder("Filter").Label("Filter branches")
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(0, 6, 12, 6).Gap(2).Children(func() {
				a.navRow(c, "changes", "Changes", strconv.Itoa(len(a.status.Changes)), false)
				a.section(c, "Branches", &a.branchesOpen, len(a.refsOf("branch")), func() {
					if a.matches("all commits") {
						a.navRow(c, "all", "All commits", "", false)
					}
					for _, ref := range a.refsOf("branch") {
						a.refRow(c, ref)
					}
				})
				if remotes := a.refsOf("remote"); len(remotes) > 0 || strings.TrimSpace(a.filter) != "" {
					a.section(c, "Remotes", &a.remotesOpen, len(remotes), func() {
						for _, ref := range remotes {
							a.refRow(c, ref)
						}
					})
				}
				if tags := a.refsOf("tag"); len(tags) > 0 {
					a.section(c, "Tags", &a.tagsOpen, len(tags), func() {
						for _, ref := range tags {
							a.refRow(c, ref)
						}
					})
				}
				if len(a.stashes) > 0 {
					shown := 0
					for _, st := range a.stashes {
						if a.matches(st.Subject) || a.matches(st.ID) {
							shown++
						}
					}
					a.section(c, "Stashes", &a.stashesOpen, shown, func() {
						for _, st := range a.stashes {
							if !a.matches(st.Subject) && !a.matches(st.ID) {
								continue
							}
							a.stashRow(c, st)
						}
					})
				}
			})
		})
	})
}

func (a *app) section(c *ui.Context, title string, open *bool, count int, body func()) {
	t := c.Theme()
	head := ui.Row(c).Padding(10, 8, 4, 8).Gap(6).AlignItems(ui.Center)
	head.Cursor(ui.CursorPointer)
	mark := "▸"
	if *open {
		mark = "▾"
	}
	head.Children(func() {
		ui.Text(c, mark).Width(12).TextColor(t.TextMuted).FontSize(11)
		ui.Text(c, strings.ToUpper(title)).FontSize(11).TextColor(t.TextMuted).Grow(1)
		if count > 0 {
			ui.Text(c, strconv.Itoa(count)).FontSize(11).TextColor(t.TextMuted)
		}
	})
	if head.Clicked() {
		*open = !*open
	}
	if *open {
		body()
	}
}

func (a *app) navRow(c *ui.Context, id, label, badge string, current bool) {
	t := c.Theme()
	selected := a.nav == id
	row := ui.Row(c).Padding(6, 8).Gap(8).Radius(6).AlignItems(ui.Center)
	row.Cursor(ui.CursorPointer)
	if selected {
		row.Background(t.Accent)
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	color := t.Text
	muted := t.TextMuted
	if selected {
		color = t.AccentText
		muted = t.AccentText
	}
	row.Children(func() {
		if current {
			ui.Box(c).Size(8, 8).Radius(4).Background(t.Success)
		} else {
			ui.Box(c).Size(8, 8)
		}
		ui.Text(c, label).MaxLines(1).Grow(1).TextColor(color)
		if badge != "" && badge != "0" {
			ui.Text(c, badge).FontSize(12).TextColor(muted)
		}
	})
	if row.Clicked() {
		a.selectNav(id)
	}
}

func (a *app) refRow(c *ui.Context, ref gitx.Ref) {
	id := ref.Kind + ":" + ref.Name
	t := c.Theme()
	selected := a.nav == id
	row := ui.Row(c).Padding(6, 8).Gap(8).Radius(6).AlignItems(ui.Center)
	row.Cursor(ui.CursorPointer)
	if selected {
		row.Background(t.Accent)
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	color := t.Text
	if selected {
		color = t.AccentText
	}
	row.Children(func() {
		if ref.Current {
			dot := t.Success
			if selected {
				dot = t.AccentText
			}
			ui.Box(c).Size(8, 8).Radius(4).Background(dot)
		} else {
			ui.Box(c).Size(8, 8)
		}
		ui.Text(c, ref.Name).MaxLines(1).Grow(1).TextColor(color)
	})
	if row.Clicked() {
		a.selectNav(id)
	}
	if row.DoubleClicked() {
		a.checkout(ref)
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("Checkout").Disabled(ref.Current && ref.Kind == "branch").Chosen() {
			a.checkout(ref)
		}
		if ref.Kind == "branch" && !ref.Current {
			if m.Item("Merge into " + a.branchLabel()).Disabled(a.branchLabel() == "").Chosen() {
				a.mergeName = ref.Name
				a.mergeOpen = true
			}
			if m.Item("Delete").Danger().Chosen() {
				a.deleteName = ref.Name
				a.deleteOpen = true
			}
		}
		m.Separator()
		if m.Item("Copy Name").Chosen() {
			copyText(ref.Name)
		}
	})
}

func (a *app) stashRow(c *ui.Context, st gitx.Stash) {
	id := "stash:" + st.ID
	t := c.Theme()
	selected := a.nav == id
	row := ui.Column(c).Padding(6, 8).Radius(6).Gap(1)
	row.Cursor(ui.CursorPointer)
	if selected {
		row.Background(t.Accent)
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	color := t.Text
	muted := t.TextMuted
	if selected {
		color = t.AccentText
		muted = t.AccentText
	}
	row.Children(func() {
		ui.Text(c, st.Subject).MaxLines(1).TextColor(color)
		ui.Text(c, st.ID).FontSize(11).TextColor(muted)
	})
	if row.Clicked() {
		a.selectNav(id)
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("Apply").Chosen() {
			a.stashPop(st.ID, false)
		}
		if m.Item("Pop").Chosen() {
			a.stashPop(st.ID, true)
		}
	})
}

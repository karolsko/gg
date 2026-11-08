package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/karolsko/gg/internal/gitx"
)

var laneColors = []ui.Color{
	ui.Hex("#3b82f6"),
	ui.Hex("#22c55e"),
	ui.Hex("#eab308"),
	ui.Hex("#f97316"),
	ui.Hex("#a855f7"),
	ui.Hex("#06b6d4"),
	ui.Hex("#ef4444"),
	ui.Hex("#ec4899"),
}

func (a *app) historyPane(c *ui.Context) {
	ui.SplitVerticalPercent(c, &a.histPercent, func() { a.commitList(c) }, func() {
		ui.Split(c, &a.changesW, func() { a.pathList(c) }, func() { a.diffPane(c) }).Fill().OnChange(a.savePrefs)
	}).Fill().OnChange(a.savePrefs)
}

func (a *app) stashPane(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(40).Padding(0, 12).Gap(8).AlignItems(ui.Center).Children(func() {
			title := strings.TrimPrefix(a.nav, "stash:")
			for _, st := range a.stashes {
				if "stash:"+st.ID == a.nav {
					title = st.Subject
					break
				}
			}
			ui.Text(c, title).MaxLines(1).Grow(1)
			id := strings.TrimPrefix(a.nav, "stash:")
			ui.Button(c, "Apply").Disabled(a.busy).OnClick(func() { a.stashPop(id, false) })
			ui.Button(c, "Pop").Disabled(a.busy).OnClick(func() { a.stashPop(id, true) })
		})
		hairline(c)
		ui.Split(c, &a.changesW, func() { a.pathList(c) }, func() { a.diffPane(c) }).
			Grow(1).FillWidth().OnChange(a.savePrefs)
	})
}

func (a *app) commitList(c *ui.Context) {
	t := c.Theme()
	width := graphBoxWidth(a.commits)
	a.histList.Key = func(i int) any { return a.commits[i].Hash }
	a.histList.Label = func(i int) string { return a.commits[i].Subject }
	a.histList.Selected = &a.histSel
	ui.Column(c).Fill().Children(func() {
		ui.List(c, &a.histList, len(a.commits), func(i int) {
			a.commitRow(c, a.commits[i], width)
		}).Grow(1).Children(func() {
			if len(a.commits) == 0 {
				ui.Text(c, "No commits yet").TextColor(t.TextMuted).Padding(16)
			}
		}).OnChange(a.onHist)
		if a.more {
			ui.Row(c).Padding(6, 10).Children(func() {
				ui.Button(c, "Load older commits").Disabled(a.busy).OnClick(a.loadOlder)
			})
		}
	})
}

func graphBoxWidth(commits []gitx.Commit) float32 {
	n := gitx.GraphWidth(commits)
	if n < 1 {
		n = 1
	}
	return float32(n)*14 + 12
}

func (a *app) commitRow(c *ui.Context, commit gitx.Commit, graphW float32) {
	t := c.Theme()
	ui.Row(c).MinHeight(40).AlignItems(ui.Stretch).Children(func() {
		ui.Box(c).Width(graphW).Draw(func(p *ui.Painter, r ui.Rect) {
			paintGraph(p, r, commit)
		})
		ui.Column(c).Grow(1).Padding(4, 8, 4, 4).Gap(2).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, commit.Subject).MaxLines(1).Grow(1)
				ui.Text(c, relTime(commit.When)).TextColor(t.TextMuted).FontSize(12)
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				hash := commit.Hash
				if len(hash) > 7 {
					hash = hash[:7]
				}
				ui.Text(c, hash).Font("monospace").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, commit.Author).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
				n := len(commit.Refs)
				if n > 3 {
					n = 3
				}
				for _, name := range commit.Refs[:n] {
					ui.Text(c, name).FontSize(11).Padding(1, 6).Radius(8).Background(t.SurfaceHover).MaxLines(1)
				}
				if extra := len(commit.Refs) - n; extra > 0 {
					ui.Textf(c, "+%d", extra).FontSize(11).TextColor(t.TextMuted)
				}
			})
		})
	})
}

func paintGraph(p *ui.Painter, r ui.Rect, commit gitx.Commit) {
	const lane float32 = 14
	cx := func(col int) float32 { return r.X + 8 + float32(col)*lane + lane/2 }
	top, bot := r.Y, r.Y+r.H
	mid := top + r.H/2
	color := func(col int) ui.Color { return laneColors[col%len(laneColors)] }
	stroke := func(x1, y1, x2, y2 float32, col int) {
		var path ui.Path
		path.MoveTo(x1, y1).LineTo(x2, y2)
		p.StrokePath(&path, 1.6, color(col))
	}
	for _, col := range commit.Passing {
		stroke(cx(col), top, cx(col), bot, col)
	}
	if commit.FromAbove {
		stroke(cx(commit.Col), top, cx(commit.Col), mid, commit.Col)
	}
	for _, col := range commit.Downs {
		if col == commit.Col {
			stroke(cx(col), mid, cx(col), bot, col)
			continue
		}
		var path ui.Path
		path.MoveTo(cx(commit.Col), mid).QuadTo(cx(commit.Col), bot, cx(col), bot)
		p.StrokePath(&path, 1.6, color(commit.Col))
	}
	rad := float32(3.4)
	x, y := cx(commit.Col), mid
	p.Fill(ui.Rect{X: x - rad, Y: y - rad, W: rad * 2, H: rad * 2}, color(commit.Col), rad)
}

func (a *app) pathList(c *ui.Context) {
	t := c.Theme()
	a.fileList.Key = func(i int) any { return a.files[i].Path }
	a.fileList.Label = func(i int) string { return a.files[i].Path }
	a.fileList.Selected = &a.fileSel
	ui.List(c, &a.fileList, len(a.files), func(i int) {
		f := a.files[i]
		ui.Row(c).Padding(5, 10).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, f.Letter()).Font("monospace").Bold().Width(16).TextColor(statusColor(c, f.Letter()))
			label := f.Path
			if f.Orig != "" {
				label = f.Orig + " → " + f.Path
			}
			ui.Text(c, label).MaxLines(1).Grow(1)
		})
	}).Fill().Children(func() {
		if len(a.files) == 0 {
			ui.Text(c, "No files").TextColor(t.TextMuted).Padding(12)
		}
	}).OnChange(a.onFile)
}

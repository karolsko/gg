// Command gg is a desktop Git client. The window is native UI drawn by
// MyGo. Repository operations go through the git installed on the machine.
package main

import (
	"log"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	a := newApp()
	mygo.App.SetMenu(a.menu())
	mygo.App.WhenReady(func() {
		a.loadPrefs()
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "gg",
			Width:     1200,
			Height:    780,
			MinWidth:  880,
			MinHeight: 560,
			StateKey:  "main",
			Content:   ui.View(a.view),
		})
		a.win.OnFileDrop(func(e *mygo.FileDropEvent) {
			if len(e.Paths) > 0 {
				a.openPath(e.Paths[0])
			}
		})
		if path := a.startupRepo(); path != "" {
			a.openPath(path)
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (a *app) menu() *mygo.Menu {
	file := []*mygo.MenuItem{
		{Label: "Open Repository…", Accelerator: "CmdOrCtrl+O", Click: func(*mygo.MenuItem, *mygo.Window) { a.openRepository() }},
		{Label: "Clone Repository…", Accelerator: "CmdOrCtrl+Shift+N", Click: func(*mygo.MenuItem, *mygo.Window) { a.showClone() }},
		{Label: "New Repository…", Click: func(*mygo.MenuItem, *mygo.Window) { a.newRepository() }},
		mygo.Separator(),
		{Label: "Close Repository", Click: func(*mygo.MenuItem, *mygo.Window) { a.closeRepo() }},
	}
	if runtime.GOOS != "darwin" {
		file = append(file, mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleQuit})
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "File", Submenu: file},
		{Label: "Repository", Submenu: []*mygo.MenuItem{
			{Label: "Fetch", Accelerator: "CmdOrCtrl+Shift+F", Click: func(*mygo.MenuItem, *mygo.Window) { a.fetch() }},
			{Label: "Pull", Accelerator: "CmdOrCtrl+Shift+L", Click: func(*mygo.MenuItem, *mygo.Window) { a.pull() }},
			{Label: "Push", Accelerator: "CmdOrCtrl+Shift+P", Click: func(*mygo.MenuItem, *mygo.Window) { a.push() }},
			mygo.Separator(),
			{Label: "Stash", Accelerator: "CmdOrCtrl+Shift+S", Click: func(*mygo.MenuItem, *mygo.Window) { a.stash() }},
			mygo.Separator(),
			{Label: "Refresh", Accelerator: "CmdOrCtrl+R", Click: func(*mygo.MenuItem, *mygo.Window) { a.refresh() }},
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu},
		{Role: mygo.RoleWindowMenu},
	})
}

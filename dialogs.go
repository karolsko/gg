package main

import (
	"context"
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/karolsko/gg/internal/gitx"
)

func (a *app) dialogs(c *ui.Context) {
	a.cloneDialog(c)
	discardMsg := a.discardPath + " will be restored to the last commit. This cannot be undone."
	if a.discardUntracked {
		discardMsg = a.discardPath + " will be deleted. This cannot be undone."
	}
	switch ui.AlertDialog(c, &a.discardOpen, "Discard changes?", discardMsg, "Cancel", "Discard") {
	case 1:
		a.discard()
	}
	switch ui.AlertDialog(c, &a.deleteOpen, "Delete branch?", "Delete "+a.deleteName+" if it is fully merged.", "Cancel", "Delete") {
	case 1:
		name := a.deleteName
		a.start("Deleting "+name+"…", func(ctx context.Context) error {
			return gitx.DeleteBranch(ctx, a.repoPath, name)
		}, func() {
			if a.nav == "branch:"+name {
				a.nav = "changes"
			}
		})
	}
	switch ui.AlertDialog(c, &a.mergeOpen, "Merge branch?", fmt.Sprintf("Merge %s into %s.", a.mergeName, a.branchLabel()), "Cancel", "Merge") {
	case 1:
		name := a.mergeName
		a.start("Merging "+name+"…", func(ctx context.Context) error {
			return gitx.Merge(ctx, a.repoPath, name)
		}, nil)
	}
}

func (a *app) cloneDialog(c *ui.Context) {
	ui.Modal(c, &a.cloneOpen, func() {
		t := c.Theme()
		ui.Column(c).Width(460).Padding(20).Gap(12).Children(func() {
			ui.Text(c, "Clone repository").FontSize(18).Bold()
			ui.Text(c, "The folder is created inside the parent you choose.").TextColor(t.TextMuted).FontSize(13)
			ui.TextInput(c, &a.cloneURL).Placeholder("https://github.com/org/repo.git").Label("URL").AutoFocus().OnChange(func() {
				if a.cloneNameAuto {
					a.cloneName = gitx.FolderName(a.cloneURL)
				}
			})
			ui.TextInput(c, &a.cloneName).Placeholder("repo").Label("Folder name").OnChange(func() {
				if a.cloneName != gitx.FolderName(a.cloneURL) {
					a.cloneNameAuto = false
				}
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &a.cloneDir).Placeholder("Parent folder").Label("Parent folder").Grow(1)
				ui.Button(c, "Browse").OnClick(a.browseCloneDir)
			})
			ui.Row(c).Gap(8).Children(func() {
				ui.Box(c).Grow(1)
				ui.Button(c, "Cancel").OnClick(func() { a.cloneOpen = false })
				ui.PrimaryButton(c, "Clone").Disabled(a.busy).OnClick(a.doClone)
			})
		})
	})
}

func (a *app) browseCloneDir() {
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            win,
			Title:             "Parent folder",
			Directory:         true,
			CreateDirectories: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.post(func() { a.cloneDir = paths[0] })
	}()
}

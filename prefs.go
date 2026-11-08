package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

type prefs struct {
	Recents  []string `json:"recents"`
	Sidebar  float32  `json:"sidebar"`
	Changes  float32  `json:"changes"`
	History  float32  `json:"history"`
	DiffMode int      `json:"diffMode"`
}

func (a *app) loadPrefs() {
	p, err := readPrefs()
	if err != nil {
		return
	}
	a.recents = p.Recents
	if p.Sidebar >= 160 && p.Sidebar <= 480 {
		a.sidebarW = p.Sidebar
	}
	if p.Changes >= 200 && p.Changes <= 700 {
		a.changesW = p.Changes
	}
	if p.History >= 20 && p.History <= 80 {
		a.histPercent = p.History
	}
	if p.DiffMode == 0 || p.DiffMode == 1 {
		a.diffMode = p.DiffMode
	}
}

func (a *app) savePrefs() {
	p := prefs{
		Recents:  a.recents,
		Sidebar:  a.sidebarW,
		Changes:  a.changesW,
		History:  a.histPercent,
		DiffMode: a.diffMode,
	}
	_ = writePrefs(p)
}

func (a *app) remember(path string) {
	next := []string{path}
	for _, old := range a.recents {
		if old == path || len(next) == 12 {
			continue
		}
		next = append(next, old)
	}
	a.recents = next
	a.savePrefs()
}

func prefsFile() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "prefs.json"), nil
}

func readPrefs() (prefs, error) {
	var p prefs
	file, err := prefsFile()
	if err != nil {
		return p, err
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(b, &p)
	return p, err
}

func writePrefs(p prefs) error {
	file, err := prefsFile()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, b, 0o644)
}

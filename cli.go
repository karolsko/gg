package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const usageText = `gg [path]

Open the Git repository that contains path.
path may be a file or a directory. With no path, gg opens the last repository.
`

// launchPath reads the path passed to gg. args are the arguments after the
// program name. A relative path is resolved from workingDir when that is set,
// which is the directory of the gg process that was just started.
func launchPath(args []string, workingDir string) (path string, help bool, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			return "", true, nil
		case "--":
			if i+1 >= len(args) {
				return "", false, errors.New("gg: missing path")
			}
			return resolveLaunchPath(args[i+1], workingDir), false, nil
		}
		if strings.HasPrefix(arg, "-") {
			return "", false, fmt.Errorf("gg: unknown option %s", arg)
		}
		return resolveLaunchPath(arg, workingDir), false, nil
	}
	return "", false, nil
}

func resolveLaunchPath(path, workingDir string) string {
	if path == "" || filepath.IsAbs(path) || workingDir == "" {
		return path
	}
	return filepath.Join(workingDir, path)
}

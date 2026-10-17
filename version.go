package main

import (
	"fmt"
	"runtime"
)

var (
	buildTime = "unknown"
	gitCommit = "unknown"
)

func versionInfo() string {
	return fmt.Sprintf(
		"html-display-server\nBuild time: %s\nGit commit: %s\nGo version: %s",
		buildTime, gitCommit, runtime.Version(),
	)
}

package version

import (
	"runtime"
)

var (
	// Version will be overwritten automatically by the build
	Version = "0.0.0"
	// GitCommit will be overwritten automatically by the build
	GitCommit = "HEAD"
	// Branch will be overwritten automatically by the build
	Branch = ""

	// GoVersion -
	GoVersion = runtime.Version()
)

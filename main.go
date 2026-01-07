package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"bakutil/cmd"
)

var (
	// Build-time variables (set by -ldflags during build)
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Set up crash recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Fatal error: %v\n", r)
			fmt.Fprintf(os.Stderr, "Go version: %s\n", runtime.Version())
			fmt.Fprintf(os.Stderr, "OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			fmt.Fprintf(os.Stderr, "App version: %s\n", version)
			os.Exit(1)
		}
	}()
	
	// Execute CLI
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// Version information
func Version() string {
	return fmt.Sprintf("backup-util %s (built %s, commit %s)", version, buildTime, gitCommit)
}
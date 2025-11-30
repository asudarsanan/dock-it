// Package main is the entry point for dock-it, a terminal-based Docker management tool.
// It initializes the Docker client and launches the TUI interface.
package main

import (
	"fmt"
	"os"
)

func main() {
	// Initialize Docker client from environment
	docker, err := NewDockerClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Docker client error: %v\n", err)
		os.Exit(1)
	}

	// Create and initialize the terminal UI
	ui := NewUI(docker)
	ui.Initialize()

	// Run the application (blocks until quit)
	if err := ui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

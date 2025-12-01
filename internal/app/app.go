package app

import (
	"fmt"

	"dock-it/internal/docker"
	"dock-it/internal/ui"
)

// Run initializes dependencies and starts the UI loop.
func Run() error {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}

	interfaceUI := ui.New(dockerClient)
	interfaceUI.Initialize()
	return interfaceUI.Run()
}

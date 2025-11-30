// Package main provides a terminal-based Docker management interface.
// This file contains the TUI implementation using tview.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UI manages the terminal user interface state and components
type UI struct {
	app         *tview.Application
	table       *tview.Table
	statusBar   *tview.TextView
	logsView    *tview.TextView
	mainView    *tview.Flex
	docker      *DockerClient
	containers  []ContainerInfo
	images      []ImageInfo
	networks    []NetworkInfo
	volumes     []VolumeInfo
	viewMode    string // "containers", "images", "networks", "volumes", "logs"
	currentView string // Track current resource view
}

// NewUI creates a new UI instance with the provided Docker client
func NewUI(docker *DockerClient) *UI {
	return &UI{
		app:         tview.NewApplication(),
		docker:      docker,
		viewMode:    "containers",
		currentView: "containers",
	}
}

// Initialize sets up the UI components and loads initial data
func (u *UI) Initialize() {
	u.table = tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0)
	u.table.SetTitle(" Docker Containers (dock-it) ").SetBorder(true)

	u.logsView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			u.app.Draw()
		})
	u.logsView.SetTitle(" Container Logs ").SetBorder(true)

	u.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetText("[yellow]1[white]:containers [yellow]2[white]:images [yellow]3[white]:networks [yellow]4[white]:volumes | [yellow]s[white]:start [yellow]x[white]:stop [yellow]d[white]:delete [yellow]q[white]:quit")

	u.setupKeyBindings()
	u.loadContainers()
}

func (u *UI) setupKeyBindings() {
	u.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// View switching
		if event.Rune() == '1' {
			u.currentView = "containers"
			go func() {
				u.app.QueueUpdateDraw(func() {
					u.loadContainers()
				})
			}()
			return nil
		} else if event.Rune() == '2' {
			u.currentView = "images"
			go func() {
				u.app.QueueUpdateDraw(func() {
					u.loadImages()
				})
			}()
			return nil
		} else if event.Rune() == '3' {
			u.currentView = "networks"
			go func() {
				u.app.QueueUpdateDraw(func() {
					u.loadNetworks()
				})
			}()
			return nil
		} else if event.Rune() == '4' {
			u.currentView = "volumes"
			go func() {
				u.app.QueueUpdateDraw(func() {
					u.loadVolumes()
				})
			}()
			return nil
		}

		// Container-specific actions
		if u.currentView == "containers" {
			row, _ := u.table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(u.containers) {
				if event.Rune() == 'q' {
					u.app.Stop()
				}
				return event
			}

			selectedContainer := u.containers[idx]

			switch event.Rune() {
			case 's':
				if selectedContainer.State != "running" {
					u.docker.StartContainer(selectedContainer.ID)
					u.loadContainers()
				}
			case 'x':
				if selectedContainer.State == "running" {
					u.docker.StopContainer(selectedContainer.ID)
					u.loadContainers()
				}
			case 'r':
				u.docker.RestartContainer(selectedContainer.ID)
				u.loadContainers()
			case 'd':
				if selectedContainer.State != "running" {
					u.docker.RemoveContainer(selectedContainer.ID)
					u.loadContainers()
				}
			case 'l':
				u.showLogs(selectedContainer)
			case 'e':
				if selectedContainer.State == "running" {
					u.execContainer(selectedContainer)
				}
			case 'R':
				u.loadContainers()
			case 'q':
				u.app.Stop()
			}
		} else if u.currentView == "images" {
			row, _ := u.table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(u.images) {
				if event.Rune() == 'q' {
					u.app.Stop()
				}
				return event
			}

			selectedImage := u.images[idx]

			switch event.Rune() {
			case 'd':
				u.docker.RemoveImage(selectedImage.ID)
				u.loadImages()
			case 'R':
				u.loadImages()
			case 'q':
				u.app.Stop()
			}
		} else if u.currentView == "networks" {
			row, _ := u.table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(u.networks) {
				if event.Rune() == 'q' {
					u.app.Stop()
				}
				return event
			}

			selectedNetwork := u.networks[idx]

			switch event.Rune() {
			case 'd':
				u.docker.RemoveNetwork(selectedNetwork.ID)
				u.loadNetworks()
			case 'R':
				u.loadNetworks()
			case 'q':
				u.app.Stop()
			}
		} else if u.currentView == "volumes" {
			row, _ := u.table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(u.volumes) {
				if event.Rune() == 'q' {
					u.app.Stop()
				}
				return event
			}

			selectedVolume := u.volumes[idx]

			switch event.Rune() {
			case 'd':
				u.docker.RemoveVolume(selectedVolume.Name)
				u.loadVolumes()
			case 'R':
				u.loadVolumes()
			case 'q':
				u.app.Stop()
			}
		}

		return event
	})

	u.logsView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			u.switchToTableView()
		}
		switch event.Rune() {
		case 'q':
			u.switchToTableView()
		}
		return event
	})
}

func (u *UI) showLogs(container ContainerInfo) {
	u.logsView.Clear()
	u.logsView.SetTitle(fmt.Sprintf(" Logs: %s ", container.Name))

	logs, err := u.docker.GetContainerLogs(container.ID, "100")
	if err != nil {
		u.logsView.SetText(fmt.Sprintf("[red]Error fetching logs: %v", err))
	} else {
		u.logsView.SetText(logs)
	}

	u.viewMode = "logs"
	u.statusBar.SetText("[yellow]ESC/q[white]:back [yellow]↑↓[white]:scroll")

	u.mainView.Clear()
	u.mainView.AddItem(u.logsView, 0, 1, true)
	u.mainView.AddItem(u.statusBar, 1, 0, false)

	u.app.SetFocus(u.logsView)
}

func (u *UI) switchToTableView() {
	u.viewMode = "containers"
	u.statusBar.SetText("[yellow]1[white]:containers [yellow]2[white]:images [yellow]3[white]:networks [yellow]4[white]:volumes | [yellow]s[white]:start [yellow]x[white]:stop [yellow]d[white]:delete [yellow]q[white]:quit")

	u.mainView.Clear()
	u.mainView.AddItem(u.table, 0, 1, true)
	u.mainView.AddItem(u.statusBar, 1, 0, false)

	u.app.SetFocus(u.table)
	u.loadContainers()
}

func (u *UI) execContainer(container ContainerInfo) {
	u.app.Suspend(func() {
		// Use docker exec to open an interactive shell
		cmd := fmt.Sprintf("docker exec -it %s /bin/sh || docker exec -it %s /bin/bash", container.ID[:12], container.ID[:12])
		fmt.Printf("\033[2J\033[H") // Clear screen
		fmt.Printf("Opening shell in container: %s\n", container.Name)
		fmt.Printf("Type 'exit' to return to dock-it\n\n")

		// Execute the command in the current terminal
		shellCmd := exec.Command("sh", "-c", cmd)
		shellCmd.Stdin = os.Stdin
		shellCmd.Stdout = os.Stdout
		shellCmd.Stderr = os.Stderr
		shellCmd.Run()
	})
}

func (u *UI) loadContainers() {
	currentRow, _ := u.table.GetSelection()
	u.table.Clear()
	u.table.SetTitle(" Docker Containers ")

	// Set headers
	headers := []string{"STATUS", "NAME", "IMAGE", "CPU", "MEMORY", "NET I/O", "PORTS"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

	containers, err := u.docker.ListContainers()
	if err != nil {
		u.table.SetCell(1, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.containers = containers

	for i, c := range containers {
		statusSymbol := "●"
		statusColor := tcell.ColorRed
		if c.State == "running" {
			statusColor = tcell.ColorGreen
		} else if c.State == "paused" {
			statusColor = tcell.ColorYellow
		}

		row := i + 1
		u.table.SetCell(row, 0, tview.NewTableCell(statusSymbol).
			SetTextColor(statusColor).
			SetAlign(tview.AlignCenter).
			SetExpansion(1))
		u.table.SetCell(row, 1, tview.NewTableCell(c.Name).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1))
		u.table.SetCell(row, 2, tview.NewTableCell(c.Image).
			SetTextColor(tcell.ColorLightBlue).
			SetExpansion(1))
		u.table.SetCell(row, 3, tview.NewTableCell(c.CPU).
			SetTextColor(tcell.ColorAqua).
			SetExpansion(1))
		u.table.SetCell(row, 4, tview.NewTableCell(c.Memory).
			SetTextColor(tcell.ColorAqua).
			SetExpansion(1))
		u.table.SetCell(row, 5, tview.NewTableCell(c.NetIO).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
		u.table.SetCell(row, 6, tview.NewTableCell(c.Ports).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
	}

	// Restore cursor position
	if currentRow > 0 && currentRow <= len(u.containers) {
		u.table.Select(currentRow, 0)
	} else if len(u.containers) > 0 {
		u.table.Select(1, 0)
	}
}

func (u *UI) loadImages() {
	currentRow, _ := u.table.GetSelection()
	u.table.Clear()
	u.table.SetTitle(" Docker Images ")

	// Set headers
	headers := []string{"ID", "TAG", "SIZE", "CREATED"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

	images, err := u.docker.ListImages()
	if err != nil {
		u.table.SetCell(1, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.images = images

	for i, img := range images {
		row := i + 1
		u.table.SetCell(row, 0, tview.NewTableCell(img.ID).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1))
		u.table.SetCell(row, 1, tview.NewTableCell(img.Tag).
			SetTextColor(tcell.ColorLightBlue).
			SetExpansion(1))
		u.table.SetCell(row, 2, tview.NewTableCell(img.Size).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
		u.table.SetCell(row, 3, tview.NewTableCell(img.Created).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
	}

	if currentRow > 0 && currentRow <= len(u.images) {
		u.table.Select(currentRow, 0)
	} else if len(u.images) > 0 {
		u.table.Select(1, 0)
	}
}

func (u *UI) loadNetworks() {
	currentRow, _ := u.table.GetSelection()
	u.table.Clear()
	u.table.SetTitle(" Docker Networks ")

	// Set headers
	headers := []string{"ID", "NAME", "DRIVER", "SCOPE"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

	networks, err := u.docker.ListNetworks()
	if err != nil {
		u.table.SetCell(1, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.networks = networks

	for i, net := range networks {
		row := i + 1
		u.table.SetCell(row, 0, tview.NewTableCell(net.ID).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1))
		u.table.SetCell(row, 1, tview.NewTableCell(net.Name).
			SetTextColor(tcell.ColorLightBlue).
			SetExpansion(1))
		u.table.SetCell(row, 2, tview.NewTableCell(net.Driver).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
		u.table.SetCell(row, 3, tview.NewTableCell(net.Scope).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
	}

	if currentRow > 0 && currentRow <= len(u.networks) {
		u.table.Select(currentRow, 0)
	} else if len(u.networks) > 0 {
		u.table.Select(1, 0)
	}
}

func (u *UI) loadVolumes() {
	currentRow, _ := u.table.GetSelection()
	u.table.Clear()
	u.table.SetTitle(" Docker Volumes ")

	// Set headers
	headers := []string{"NAME", "DRIVER", "MOUNTPOINT"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

	volumes, err := u.docker.ListVolumes()
	if err != nil {
		u.table.SetCell(1, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.volumes = volumes

	for i, vol := range volumes {
		row := i + 1
		u.table.SetCell(row, 0, tview.NewTableCell(vol.Name).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1))
		u.table.SetCell(row, 1, tview.NewTableCell(vol.Driver).
			SetTextColor(tcell.ColorLightBlue).
			SetExpansion(1))
		u.table.SetCell(row, 2, tview.NewTableCell(vol.Mountpoint).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
	}

	if currentRow > 0 && currentRow <= len(u.volumes) {
		u.table.Select(currentRow, 0)
	} else if len(u.volumes) > 0 {
		u.table.Select(1, 0)
	}
}

func (u *UI) Run() error {
	u.mainView = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(u.table, 0, 1, true).
		AddItem(u.statusBar, 1, 0, false)

	if err := u.app.SetRoot(u.mainView, true).Run(); err != nil {
		return fmt.Errorf("TUI error: %v", err)
	}
	return nil
}

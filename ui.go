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

const (
	tableStatusText = "[yellow]1[white]:containers [yellow]2[white]:images [yellow]3[white]:networks [yellow]4[white]:volumes | [yellow]s[white]:start [yellow]x[white]:stop [yellow]d[white]:delete [yellow]q[white]:quit"
	logsStatusText  = "[yellow]ESC/q[white]:back [yellow]↑↓[white]:scroll"
	containersTitle = " Docker Containers (dock-it) "
	imagesTitle     = " Docker Images "
	networksTitle   = " Docker Networks "
	volumesTitle    = " Docker Volumes "
)

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
	u.table.SetTitle(containersTitle).SetBorder(true)

	u.logsView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			u.app.Draw()
		})
	u.logsView.SetTitle(" Container Logs ").SetBorder(true)

	u.statusBar = tview.NewTextView().
		SetDynamicColors(true)
	u.updateStatusBarText()

	u.setupKeyBindings()
	u.loadContainers()
}

func (u *UI) setupKeyBindings() {
	u.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case '1':
			u.currentView = "containers"
			u.loadContainers()
			return nil
		case '2':
			u.currentView = "images"
			u.loadImages()
			return nil
		case '3':
			u.currentView = "networks"
			u.loadNetworks()
			return nil
		case '4':
			u.currentView = "volumes"
			u.loadVolumes()
			return nil
		case 'R':
			u.reloadCurrentView()
			return nil
		case 'q':
			u.app.Stop()
			return nil
		}

		switch u.currentView {
		case "containers":
			row, _ := u.table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(u.containers) {
				return event
			}

			selectedContainer := u.containers[idx]

			switch event.Rune() {
			case 's':
				if selectedContainer.State != "running" {
					u.runAsyncAction(fmt.Sprintf("Start %s", selectedContainer.Name), func() error {
						return u.docker.StartContainer(selectedContainer.ID)
					}, func() {
						u.loadContainers()
					})
				}
				return nil
			case 'x':
				if selectedContainer.State == "running" {
					u.runAsyncAction(fmt.Sprintf("Stop %s", selectedContainer.Name), func() error {
						return u.docker.StopContainer(selectedContainer.ID)
					}, func() {
						u.loadContainers()
					})
				}
				return nil
			case 'r':
				u.runAsyncAction(fmt.Sprintf("Restart %s", selectedContainer.Name), func() error {
					return u.docker.RestartContainer(selectedContainer.ID)
				}, func() {
					u.loadContainers()
				})
				return nil
			case 'd':
				if selectedContainer.State != "running" {
					u.runAsyncAction(fmt.Sprintf("Remove %s", selectedContainer.Name), func() error {
						return u.docker.RemoveContainer(selectedContainer.ID)
					}, func() {
						u.loadContainers()
					})
				}
				return nil
			case 'l':
				u.showLogs(selectedContainer)
				return nil
			case 'e':
				if selectedContainer.State == "running" {
					u.execContainer(selectedContainer)
				}
				return nil
			}
		case "images":
			row, _ := u.table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(u.images) {
				return event
			}

			selectedImage := u.images[idx]

			switch event.Rune() {
			case 'd':
				u.runAsyncAction(fmt.Sprintf("Remove image %s", selectedImage.ID), func() error {
					return u.docker.RemoveImage(selectedImage.ID)
				}, func() {
					u.loadImages()
				})
				return nil
			}
		case "networks":
			row, _ := u.table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(u.networks) {
				return event
			}

			selectedNetwork := u.networks[idx]

			switch event.Rune() {
			case 'd':
				u.runAsyncAction(fmt.Sprintf("Remove network %s", selectedNetwork.Name), func() error {
					return u.docker.RemoveNetwork(selectedNetwork.ID)
				}, func() {
					u.loadNetworks()
				})
				return nil
			}
		case "volumes":
			row, _ := u.table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(u.volumes) {
				return event
			}

			selectedVolume := u.volumes[idx]

			switch event.Rune() {
			case 'd':
				u.runAsyncAction(fmt.Sprintf("Remove volume %s", selectedVolume.Name), func() error {
					return u.docker.RemoveVolume(selectedVolume.Name)
				}, func() {
					u.loadVolumes()
				})
				return nil
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

func (u *UI) setStatusMessage(msg string) {
	u.statusBar.SetText(msg)
}

func (u *UI) updateStatusBarText() {
	if u.viewMode == "logs" {
		u.statusBar.SetText(logsStatusText)
		return
	}
	u.statusBar.SetText(tableStatusText)
}

func (u *UI) runAsyncAction(actionLabel string, action func() error, onSuccess func()) {
	u.setStatusMessage(fmt.Sprintf("[yellow]%s...", actionLabel))
	go func() {
		err := action()
		u.app.QueueUpdateDraw(func() {
			if err != nil {
				u.statusBar.SetText(fmt.Sprintf("[red]%s failed: %v", actionLabel, err))
				return
			}
			if onSuccess != nil {
				onSuccess()
			}
			u.updateStatusBarText()
		})
	}()
}

func (u *UI) showLoading(title string) {
	u.table.Clear()
	u.table.SetTitle(title)
	u.table.SetCell(0, 0, tview.NewTableCell("Loading...").
		SetSelectable(false).
		SetTextColor(tcell.ColorGray))
}

func (u *UI) reloadCurrentView() {
	switch u.currentView {
	case "containers":
		u.loadContainers()
	case "images":
		u.loadImages()
	case "networks":
		u.loadNetworks()
	case "volumes":
		u.loadVolumes()
	}
}

func (u *UI) showLogs(container ContainerInfo) {
	u.logsView.Clear()
	u.logsView.SetTitle(fmt.Sprintf(" Logs: %s ", container.Name))
	u.logsView.SetText("Loading logs...")

	go func() {
		logs, err := u.docker.GetContainerLogs(container.ID, "100")
		u.app.QueueUpdateDraw(func() {
			if err != nil {
				u.logsView.SetText(fmt.Sprintf("[red]Error fetching logs: %v", err))
				return
			}
			u.logsView.SetText(logs)
		})
	}()

	u.viewMode = "logs"
	u.updateStatusBarText()

	u.mainView.Clear()
	u.mainView.AddItem(u.logsView, 0, 1, true)
	u.mainView.AddItem(u.statusBar, 1, 0, false)

	u.app.SetFocus(u.logsView)
}

func (u *UI) switchToTableView() {
	u.viewMode = "containers"
	u.updateStatusBarText()

	u.mainView.Clear()
	u.mainView.AddItem(u.table, 0, 1, true)
	u.mainView.AddItem(u.statusBar, 1, 0, false)

	u.app.SetFocus(u.table)
	u.reloadCurrentView()
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
	u.showLoading(containersTitle)
	go func(selectedRow int) {
		containers, err := u.docker.ListContainers()
		u.app.QueueUpdateDraw(func() {
			u.renderContainers(containers, err, selectedRow)
		})
	}(currentRow)
}

func (u *UI) loadImages() {
	currentRow, _ := u.table.GetSelection()
	u.showLoading(imagesTitle)
	go func(selectedRow int) {
		images, err := u.docker.ListImages()
		u.app.QueueUpdateDraw(func() {
			u.renderImages(images, err, selectedRow)
		})
	}(currentRow)
}

func (u *UI) loadNetworks() {
	currentRow, _ := u.table.GetSelection()
	u.showLoading(networksTitle)
	go func(selectedRow int) {
		networks, err := u.docker.ListNetworks()
		u.app.QueueUpdateDraw(func() {
			u.renderNetworks(networks, err, selectedRow)
		})
	}(currentRow)
}

func (u *UI) loadVolumes() {
	currentRow, _ := u.table.GetSelection()
	u.showLoading(volumesTitle)
	go func(selectedRow int) {
		volumes, err := u.docker.ListVolumes()
		u.app.QueueUpdateDraw(func() {
			u.renderVolumes(volumes, err, selectedRow)
		})
	}(currentRow)
}

func (u *UI) renderContainers(containers []ContainerInfo, err error, selectedRow int) {
	u.table.Clear()
	u.table.SetTitle(containersTitle)
	if err != nil {
		u.table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.containers = containers
	headers := []string{"STATUS", "NAME", "IMAGE", "CPU", "MEMORY", "NET I/O", "PORTS"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

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

	u.restoreSelection(selectedRow, len(containers))
}

func (u *UI) renderImages(images []ImageInfo, err error, selectedRow int) {
	u.table.Clear()
	u.table.SetTitle(imagesTitle)
	if err != nil {
		u.table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.images = images
	headers := []string{"ID", "TAG", "SIZE", "CREATED"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

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

	u.restoreSelection(selectedRow, len(images))
}

func (u *UI) renderNetworks(networks []NetworkInfo, err error, selectedRow int) {
	u.table.Clear()
	u.table.SetTitle(networksTitle)
	if err != nil {
		u.table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.networks = networks
	headers := []string{"ID", "NAME", "DRIVER", "SCOPE"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

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

	u.restoreSelection(selectedRow, len(networks))
}

func (u *UI) renderVolumes(volumes []VolumeInfo, err error, selectedRow int) {
	u.table.Clear()
	u.table.SetTitle(volumesTitle)
	if err != nil {
		u.table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.volumes = volumes
	headers := []string{"NAME", "DRIVER", "MOUNTPOINT"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

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

	u.restoreSelection(selectedRow, len(volumes))
}

func (u *UI) restoreSelection(selectedRow, total int) {
	switch {
	case total == 0:
		u.table.Select(0, 0)
	case selectedRow > 0 && selectedRow <= total:
		u.table.Select(selectedRow, 0)
	default:
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

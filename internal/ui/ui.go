package ui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"dock-it/internal/docker"
	"dock-it/internal/filter"
	"dock-it/internal/logs"
)

// UI manages the terminal interface and orchestrates Docker operations.
type UI struct {
	app         *tview.Application
	table       *tview.Table
	statusBar   *tview.TextView
	detailView  *tview.TextView
	filterInput *tview.InputField
	mainView    *tview.Flex
	docker      *docker.Client
	containers  []docker.ContainerInfo
	images      []docker.ImageInfo
	networks    []docker.NetworkInfo
	volumes     []docker.VolumeInfo
	viewMode    string
	currentView string
	filter      *filter.Filter
	filterMode  bool
}

const (
	tableStatusText  = "[yellow]1[white]:containers [yellow]2[white]:images [yellow]3[white]:networks [yellow]4[white]:volumes | [yellow]/[white]:search [yellow]c[white]:clear [yellow]s[white]:start [yellow]x[white]:stop [yellow]d[white]:delete [yellow]i[white]:describe [yellow]q[white]:quit"
	detailStatusText = "[yellow]ESC/q[white]:back [yellow]↑↓[white]:scroll"
	filterStatusText = "[yellow]Enter[white]:search [yellow]ESC[white]:cancel [yellow]Ctrl+U[white]:clear | Search across name, image, status, etc. or use advanced: [gray]age>1h, status=running[white]"
	containersTitle  = " Docker Containers (dock-it) "
	imagesTitle      = " Docker Images "
	networksTitle    = " Docker Networks "
	volumesTitle     = " Docker Volumes "
)

// New constructs a UI bound to the provided Docker client.
func New(dockerClient *docker.Client) *UI {
	return &UI{
		app:         tview.NewApplication(),
		docker:      dockerClient,
		viewMode:    "list",
		currentView: "containers",
		filter:      filter.New(),
		filterMode:  false,
	}
}

// Initialize configures primitive components and loads initial data.
func (u *UI) Initialize() {
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorBlack
	tview.Styles.ContrastBackgroundColor = tcell.ColorBlack
	tview.Styles.MoreContrastBackgroundColor = tcell.ColorBlack
	tview.Styles.BorderColor = tcell.ColorGray
	tview.Styles.TitleColor = tcell.ColorWhite
	tview.Styles.GraphicsColor = tcell.ColorGray

	u.table = tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0)
	u.table.SetTitle(containersTitle).SetBorder(true)

	u.detailView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			u.app.Draw()
		})
	u.detailView.SetTitle(" Details ").SetBorder(true)

	u.filterInput = tview.NewInputField().
		SetLabel("Search: ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetPlaceholder("Type to search across all fields (or use advanced filters like age>1h)")
	u.filterInput.SetBorder(true).SetTitle(" Search/Filter ")

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
		case '/':
			u.showFilterInput()
			return nil
		case 'c':
			if !u.filter.IsEmpty() {
				u.clearFilter()
			}
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
			case 'i':
				u.describeContainer(selectedContainer)
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
			case 'i':
				u.describeImage(selectedImage)
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
			case 'i':
				u.describeNetwork(selectedNetwork)
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
			case 'i':
				u.describeVolume(selectedVolume)
				return nil
			}
		}

		return event
	})

	u.detailView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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

	u.filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			u.applyFilter()
			return nil
		case tcell.KeyEscape:
			u.hideFilterInput()
			return nil
		case tcell.KeyCtrlU:
			u.filterInput.SetText("")
			return nil
		}
		return event
	})
}

func (u *UI) setStatusMessage(msg string) {
	u.statusBar.SetText(msg)
}

func (u *UI) updateStatusBarText() {
	if u.viewMode == "detail" {
		u.statusBar.SetText(detailStatusText)
		return
	}
	if u.filterMode {
		u.statusBar.SetText(filterStatusText)
		return
	}

	statusText := tableStatusText
	if !u.filter.IsEmpty() {
		statusText = fmt.Sprintf("[green]Filter: %s[white] | [yellow]c[white]:clear | %s", u.filter.String(), tableStatusText)
	}
	u.statusBar.SetText(statusText)
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

func (u *UI) showFilterInput() {
	u.filterMode = true
	u.updateStatusBarText()

	// Set initial text if filter exists
	if !u.filter.IsEmpty() {
		u.filterInput.SetText(u.filter.String())
	}

	u.mainView.Clear()
	u.mainView.AddItem(u.table, 0, 1, false)
	u.mainView.AddItem(u.filterInput, 3, 0, true)
	u.mainView.AddItem(u.statusBar, 1, 0, false)

	u.app.SetFocus(u.filterInput)
}

func (u *UI) hideFilterInput() {
	u.filterMode = false
	u.updateStatusBarText()

	u.mainView.Clear()
	u.mainView.AddItem(u.table, 0, 1, true)
	u.mainView.AddItem(u.statusBar, 1, 0, false)

	u.app.SetFocus(u.table)
}

func (u *UI) applyFilter() {
	filterText := u.filterInput.GetText()

	newFilter, err := filter.ParseFilter(filterText)
	if err != nil {
		u.statusBar.SetText(fmt.Sprintf("[red]Filter error: %v", err))
		return
	}

	u.filter = newFilter
	u.hideFilterInput()
	u.reloadCurrentView()
}

func (u *UI) clearFilter() {
	u.filter = filter.New()
	u.filterInput.SetText("")
	u.updateStatusBarText()
	u.reloadCurrentView()
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

func (u *UI) showDetail(title string, loader func() (string, error)) {
	u.detailView.Clear()
	u.detailView.SetTitle(title)
	u.detailView.SetText("Loading...")

	go func() {
		content, err := loader()
		u.app.QueueUpdateDraw(func() {
			if err != nil {
				u.detailView.SetText(fmt.Sprintf("[red]Error: %v", err))
				return
			}
			if strings.TrimSpace(content) == "" {
				u.detailView.SetText("(no data)")
				return
			}
			u.detailView.SetText(content)
		})
	}()

	u.viewMode = "detail"
	u.updateStatusBarText()

	u.mainView.Clear()
	u.mainView.AddItem(u.detailView, 0, 1, true)
	u.mainView.AddItem(u.statusBar, 1, 0, false)

	u.app.SetFocus(u.detailView)
}

func (u *UI) showLogs(container docker.ContainerInfo) {
	title := fmt.Sprintf(" Logs: %s ", container.Name)
	u.showDetail(title, func() (string, error) {
		logsOutput, err := u.docker.GetContainerLogs(container.ID, "100")
		if err != nil {
			return "", err
		}
		return logs.Colorize(logsOutput), nil
	})
}

func (u *UI) describeContainer(container docker.ContainerInfo) {
	title := fmt.Sprintf(" Describe Container: %s ", container.Name)
	u.showDetail(title, func() (string, error) {
		return u.docker.DescribeContainer(container.ID)
	})
}

func (u *UI) describeImage(image docker.ImageInfo) {
	label := image.Tag
	if label == "<none>" {
		label = image.ID
	}
	title := fmt.Sprintf(" Describe Image: %s ", label)
	u.showDetail(title, func() (string, error) {
		return u.docker.DescribeImage(image.ID)
	})
}

func (u *UI) describeNetwork(network docker.NetworkInfo) {
	title := fmt.Sprintf(" Describe Network: %s ", network.Name)
	u.showDetail(title, func() (string, error) {
		return u.docker.DescribeNetwork(network.ID)
	})
}

func (u *UI) describeVolume(volume docker.VolumeInfo) {
	title := fmt.Sprintf(" Describe Volume: %s ", volume.Name)
	u.showDetail(title, func() (string, error) {
		return u.docker.DescribeVolume(volume.Name)
	})
}

func (u *UI) switchToTableView() {
	u.viewMode = "list"
	u.updateStatusBarText()

	u.mainView.Clear()
	u.mainView.AddItem(u.table, 0, 1, true)
	u.mainView.AddItem(u.statusBar, 1, 0, false)

	u.app.SetFocus(u.table)
	u.reloadCurrentView()
}

func (u *UI) execContainer(container docker.ContainerInfo) {
	u.app.Suspend(func() {
		id := container.ID
		shortID := id
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}

		fmt.Printf("\033[2J\033[H")
		fmt.Printf("Opening shell in container: %s (%s)\n", container.Name, shortID)
		fmt.Printf("Type 'exit' to return to dock-it\n\n")

		shells := preferredShells()
		var lastErr error
		for i, shell := range shells {
			if err := runDockerExec(id, shell); err == nil {
				return
			} else {
				lastErr = err
				if i < len(shells)-1 {
					fmt.Printf("Failed to start %s: %v\nTrying fallback shell...\n", shell, err)
				}
			}
		}

		fmt.Printf("Failed to exec into container after trying %d shell(s): %v\n", len(shells), lastErr)
		fmt.Print("Press Enter to continue...")
		bufio.NewReader(os.Stdin).ReadString('\n')
	})
}

func runDockerExec(containerID, shell string) error {
	cmd := exec.Command("docker", "exec", "-it", containerID, shell)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func preferredShells() []string {
	seen := make(map[string]struct{})
	appendUnique := func(list []string, values ...string) []string {
		for _, v := range values {
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			list = append(list, v)
		}
		return list
	}

	var shells []string
	if shell := os.Getenv("SHELL"); shell != "" {
		shells = appendUnique(shells, shell)
		base := filepath.Base(shell)
		if base != shell {
			shells = appendUnique(shells, base)
		}
	}

	shells = appendUnique(shells, "bash", "sh")
	return shells
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

func (u *UI) renderContainers(containers []docker.ContainerInfo, err error, selectedRow int) {
	u.table.Clear()
	u.table.SetTitle(containersTitle)
	if err != nil {
		u.table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.containers = containers

	// Apply filters
	filtered := make([]docker.ContainerInfo, 0, len(containers))
	for _, c := range containers {
		if u.filter.MatchContainer(c) {
			filtered = append(filtered, c)
		}
	}

	headers := []string{"STATUS", "NAME", "AGE", "IMAGE", "CPU", "MEMORY", "NET I/O", "PORTS"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

	for i, c := range filtered {
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
		u.table.SetCell(row, 2, tview.NewTableCell(c.Age).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
		u.table.SetCell(row, 3, tview.NewTableCell(c.Image).
			SetTextColor(tcell.ColorLightBlue).
			SetExpansion(1))
		u.table.SetCell(row, 4, tview.NewTableCell(c.CPU).
			SetTextColor(tcell.ColorAqua).
			SetExpansion(1))
		u.table.SetCell(row, 5, tview.NewTableCell(c.Memory).
			SetTextColor(tcell.ColorAqua).
			SetExpansion(1))
		u.table.SetCell(row, 6, tview.NewTableCell(c.NetIO).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
		u.table.SetCell(row, 7, tview.NewTableCell(c.Ports).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
	}

	u.restoreSelection(selectedRow, len(filtered))
}

func (u *UI) renderImages(images []docker.ImageInfo, err error, selectedRow int) {
	u.table.Clear()
	u.table.SetTitle(imagesTitle)
	if err != nil {
		u.table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.images = images

	// Apply filters
	filtered := make([]docker.ImageInfo, 0, len(images))
	for _, img := range images {
		if u.filter.MatchImage(img) {
			filtered = append(filtered, img)
		}
	}

	headers := []string{"ID", "TAG", "SIZE", "AGE"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

	for i, img := range filtered {
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
		u.table.SetCell(row, 3, tview.NewTableCell(img.Age).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
	}

	u.restoreSelection(selectedRow, len(filtered))
}

func (u *UI) renderNetworks(networks []docker.NetworkInfo, err error, selectedRow int) {
	u.table.Clear()
	u.table.SetTitle(networksTitle)
	if err != nil {
		u.table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.networks = networks

	// Apply filters
	filtered := make([]docker.NetworkInfo, 0, len(networks))
	for _, net := range networks {
		if u.filter.MatchNetwork(net) {
			filtered = append(filtered, net)
		}
	}

	headers := []string{"ID", "NAME", "AGE", "DRIVER", "SCOPE"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

	for i, net := range filtered {
		row := i + 1
		u.table.SetCell(row, 0, tview.NewTableCell(net.ID).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1))
		u.table.SetCell(row, 1, tview.NewTableCell(net.Name).
			SetTextColor(tcell.ColorLightBlue).
			SetExpansion(1))
		u.table.SetCell(row, 2, tview.NewTableCell(net.Age).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
		u.table.SetCell(row, 3, tview.NewTableCell(net.Driver).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
		u.table.SetCell(row, 4, tview.NewTableCell(net.Scope).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
	}

	u.restoreSelection(selectedRow, len(filtered))
}

func (u *UI) renderVolumes(volumes []docker.VolumeInfo, err error, selectedRow int) {
	u.table.Clear()
	u.table.SetTitle(volumesTitle)
	if err != nil {
		u.table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).
			SetTextColor(tcell.ColorRed))
		return
	}

	u.volumes = volumes

	// Apply filters
	filtered := make([]docker.VolumeInfo, 0, len(volumes))
	for _, vol := range volumes {
		if u.filter.MatchVolume(vol) {
			filtered = append(filtered, vol)
		}
	}

	headers := []string{"NAME", "AGE", "DRIVER", "MOUNTPOINT"}
	for col, header := range headers {
		u.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold))
	}

	for i, vol := range filtered {
		row := i + 1
		u.table.SetCell(row, 0, tview.NewTableCell(vol.Name).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1))
		u.table.SetCell(row, 1, tview.NewTableCell(vol.Age).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
		u.table.SetCell(row, 2, tview.NewTableCell(vol.Driver).
			SetTextColor(tcell.ColorLightBlue).
			SetExpansion(1))
		u.table.SetCell(row, 3, tview.NewTableCell(vol.Mountpoint).
			SetTextColor(tcell.ColorGray).
			SetExpansion(1))
	}

	u.restoreSelection(selectedRow, len(filtered))
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

// Run bootstraps the flex layout and starts the tview event loop.
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

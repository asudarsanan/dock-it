# dock-it 🐳

A fast, terminal-based Docker management tool inspired by k9s, built with Go.

## Features

### Multi-View Resource Management
- **Containers**: View, start, stop, restart, delete containers with real-time metrics
- **Images**: Browse and manage Docker images
- **Networks**: Inspect and manage Docker networks
- **Volumes**: View and manage Docker volumes

### Container Operations
- 📊 **Real-time Metrics**: CPU, Memory, and Network I/O stats
- 📝 **Log Streaming**: View container logs with scrolling
- 🖥️ **Shell Access**: Execute interactive shells into containers
- ⚡ **Quick Actions**: Start, stop, restart, delete with single keystrokes
- 🎨 **Status Indicators**: Color-coded container states (running=green, exited=red)

### Performance
- **Non-blocking UI**: Async operations with 2-second timeouts
- **Responsive**: View switching and operations never freeze the interface
- **Efficient**: Minimal resource usage, goroutine-based updates

## Installation

### Prerequisites
- Go 1.21+ ([installation guide](https://golang.org/doc/install))
- Docker running locally

### Build from Source
```bash
git clone <your-repo-url>
cd dock-it
go build -o dock-it
./dock-it
```

### Quick Run (Development)
```bash
go run main.go
```

## Usage

### Keyboard Shortcuts

#### View Navigation
- `1` - Switch to Containers view
- `2` - Switch to Images view
- `3` - Switch to Networks view
- `4` - Switch to Volumes view

#### Container Actions
- `s` - Start selected container
- `x` - Stop selected container
- `r` - Restart selected container
- `d` - Delete selected container
- `l` - View container logs
- `e` - Execute shell in container (interactive)
- `R` - Refresh current view

#### Image Actions
- `d` - Delete selected image

#### Network Actions
- `d` - Delete selected network

#### Volume Actions
- `d` - Delete selected volume

#### General
- `q` - Quit application
- `ESC` - Exit logs view / return to main view
- `↑/↓` - Navigate items
- `PgUp/PgDn` - Scroll logs (in logs view)

### Container Metrics

Real-time metrics are displayed for running containers:
- **CPU**: Percentage of CPU usage
- **Memory**: Percentage of memory usage
- **Net I/O**: Network traffic (RX/TX in MB)

Metrics use a 2-second timeout to ensure UI responsiveness.

## Architecture

### Project Structure
```
dock-it/
├── main.go       # Application entry point
├── docker.go     # Docker API client wrapper
├── ui.go         # TUI implementation with tview
├── go.mod        # Go module dependencies
└── README.md     # Documentation
```

### Key Components

#### `docker.go` - Docker Client
- Wraps Docker SDK for Go
- Provides high-level methods for all resource types
- Context-aware stats fetching with timeout protection
- Error handling and data parsing

#### `ui.go` - Terminal UI
- Built with [tview](https://github.com/rivo/tview)
- Table-based views with full-width columns
- Async operations using goroutines + QueueUpdateDraw
- Modal logs view with scrolling

#### `main.go` - Orchestration
- Initializes Docker client and UI
- Minimal coordination layer

### Async Design Pattern

All UI updates use this pattern to prevent freezing:
```go
go func() {
    u.app.QueueUpdateDraw(func() {
        u.loadContainers()
    })
}()
```

Stats fetching uses context with timeout:
```go
statsCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()
stats, err := d.getContainerStatsWithContext(statsCtx, c.ID)
```

## Dependencies

- [rivo/tview](https://github.com/rivo/tview) v0.42.0 - Rich TUI framework
- [gdamore/tcell](https://github.com/gdamore/tcell) v2.8.1 - Terminal cell manipulation
- [docker/docker](https://github.com/docker/docker) v28.5.2 - Official Docker SDK

## Troubleshooting

### Stats not showing
- Ensure containers are running (stats only shown for running containers)
- Check Docker daemon is accessible: `docker ps`
- Stats may take up to 2 seconds to appear on first load

### View switching slow
- Normal on first load while fetching Docker resources
- Subsequent switches should be instant (cached data)

### Shell execution fails
- Some containers may not have `/bin/bash` - tries `/bin/sh` as fallback
- Container must be running for shell access

## Future Enhancements

- [ ] Auto-refresh mode with configurable intervals
- [ ] Container inspect view (full details)
- [ ] Image pull/push operations
- [ ] Compose file management
- [ ] Search/filter functionality
- [ ] Custom color themes
- [ ] Export logs to file
- [ ] Multi-container operations (bulk actions)

## Performance Notes

- Initial container list with metrics: ~2 seconds for multiple running containers
- View switching: < 100ms (cached)
- Memory usage: < 20MB typical
- CPU usage: Minimal when idle, brief spike during stats collection

## License

MIT License - feel free to use and modify

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Submit a pull request

---

Built with ❤️ using Go and tview

## CI/CD

This repository ships with a GitHub Actions workflow that:
- runs gofmt, `go vet`, `go test` on Linux and macOS for every push/PR targeting `main`
- builds release binaries for Linux (amd64/arm64) and macOS (amd64/arm64) whenever a `v*` tag is pushed, publishing them as release assets with accompanying SHA-256 checksums

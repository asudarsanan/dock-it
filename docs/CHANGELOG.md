# Changelog

## [1.0.0] - 2025-11-30

### Features
- ✨ Multi-view Docker resource management (containers, images, networks, volumes)
- 📊 Real-time container metrics (CPU, Memory, Network I/O)
- 📝 Container log streaming with scrollable view
- 🖥️ Interactive shell execution into containers
- ⚡ Quick container operations (start, stop, restart, delete)
- 🎨 Color-coded status indicators
- ⌨️ Comprehensive keyboard shortcuts

### Performance Optimizations
- 🚀 Concurrent stats fetching for multiple containers using goroutines
- ⏱️ 2-second timeout context for stats API to prevent UI blocking
- 🔄 Non-blocking UI updates with QueueUpdateDraw pattern
- 💾 Efficient memory usage with mutex-protected concurrent writes

### Architecture
- 📁 Modular design with separated concerns (main.go, docker.go, ui.go)
- 🔌 Clean Docker API wrapper layer
- 🎯 Context-aware operations with proper timeout handling
- 📚 Comprehensive code documentation

### Bug Fixes
- 🐛 Fixed UI freezing when switching between views
- 🐛 Fixed blocking stats collection causing unresponsive interface
- 🐛 Proper defer cancel() usage in goroutines

### Documentation
- 📖 Comprehensive README with usage guide
- 💡 Inline code documentation for all major functions
- 🏗️ Architecture overview and design patterns
- 🔧 Troubleshooting section

### Dependencies
- github.com/rivo/tview v0.42.0
- github.com/gdamore/tcell/v2 v2.8.1
- github.com/docker/docker v28.5.2+incompatible

### Technical Details
- Go 1.21+ required
- Docker daemon required
- Cross-platform (Linux, macOS, Windows with Docker)

### Known Limitations
- Stats collection requires containers to be running
- 2-second timeout may result in "-" for slow-responding containers
- Shell execution requires `/bin/bash` or `/bin/sh` in container

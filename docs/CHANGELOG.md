# Changelog

## [1.1.0] - 2025-12-02

### Features
- ✨ **Age Display**: All resources now show creation age in human-readable format (e.g., "2h ago", "3d ago")
  - Added AGE column to containers, images, networks, and volumes views
  - Smart formatting: seconds/minutes/hours/days/weeks/months/years
  - Calculated from Docker API creation timestamps

- 🔍 **Advanced Filter System**: Interactive filtering for all resource types
  - Press `/` to open filter input bar
  - Rich query language: `age>1h`, `status=running`, `name~redis`, `size>100MB`
  - Multiple filter criteria: `age>1d,state=running`
  - Support for various operators: `=`, `!=`, `>`, `<`, `>=`, `<=`, `~`, `!~`, `=~`
  - Duration parsing: hours (h), minutes (m), days (d), weeks (w), months (mo), years (y)
  - Size parsing: B, KB, MB, GB, TB
  - Press `c` to clear active filters
  - Status bar shows active filters and syntax help

- 🎯 **Filter Capabilities**:
  - **Containers**: Filter by age, status, state, name
  - **Images**: Filter by age, tag/name, size
  - **Networks**: Filter by age, name, driver, scope
  - **Volumes**: Filter by age, name, driver

### Improvements
- 📊 Enhanced table layouts with age information
- 🎨 Updated status bar with filter indicators
- ⌨️ New key bindings: `/` (filter), `c` (clear filter), `Ctrl+U` (clear input)
- 📝 Comprehensive filter documentation

### Technical Details
- 🏗️ New `internal/filter` package with full test coverage
- 🧪 15+ test cases covering filter parsing, matching, and edge cases
- 🔄 In-memory filtering for fast performance
- 🎯 Type-safe filter criteria with proper error handling

### Documentation
- 📚 Added `docs/age_and_filter_features.md` with complete usage guide
- 📖 Updated feature ideas with implementation status

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

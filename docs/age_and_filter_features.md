# Age Display and Filter Features

## Overview

This document describes the age display and comprehensive filter features added to dock-it.

## Age Display

### What's New

All resource views (containers, images, networks, and volumes) now display an AGE column showing how long ago each resource was created.

### Age Format

The age is displayed in a human-readable format:
- `just now` - Less than 1 minute
- `5m ago` - Minutes
- `2h ago` - Hours  
- `3d ago` - Days
- `2w ago` - Weeks
- `1mo ago` - Months
- `1y ago` - Years

### Column Layout

- **Containers**: `STATUS | NAME | AGE | IMAGE | CPU | MEMORY | NET I/O | PORTS`
- **Images**: `ID | TAG | SIZE | AGE`
- **Networks**: `ID | NAME | AGE | DRIVER | SCOPE`
- **Volumes**: `NAME | AGE | DRIVER | MOUNTPOINT`

**Note**: Network and Volume ages may show `-` if creation timestamps are not available from the Docker API.

## Filter System

### Overview

The filter system allows you to quickly slice and filter resources by various attributes using a simple query language.

### Activation

Press `/` to open the filter input bar. The status bar will show filter syntax help.

### Filter Syntax

Filters use the format: `<field><operator><value>`

Multiple filters can be combined with commas: `age>1h,status=running`

### Supported Operators

- `=` - Equal to
- `!=` - Not equal to
- `>` - Greater than (numeric/duration)
- `<` - Less than (numeric/duration)
- `>=` - Greater than or equal to
- `<=` - Less than or equal to
- `~` - Contains (case-insensitive substring)
- `!~` - Does not contain
- `=~` - Regex match

### Filter Fields

#### Containers

- `age` - Time since creation (e.g., `age>1h`, `age<30m`)
- `status` - Container status string (e.g., `status~Up`)
- `state` - Container state (e.g., `state=running`, `state=exited`)
- `name` - Container name (e.g., `name~redis`, `name=mycontainer`)

#### Images

- `age` - Time since creation
- `tag` - Image tag (e.g., `tag~ubuntu`, `tag=latest`)
- `name` - Same as tag
- `size` - Image size (e.g., `size>100MB`, `size<1GB`)

#### Networks

- `age` - Time since creation (if available)
- `name` - Network name
- `driver` - Network driver (e.g., `driver=bridge`)
- `scope` - Network scope (e.g., `scope=local`)

#### Volumes

- `age` - Time since creation (if available)
- `name` - Volume name
- `driver` - Volume driver

### Duration Format

Age filters support these duration formats:
- `m` - Minutes (e.g., `30m`)
- `h` - Hours (e.g., `2h`)
- `d` - Days (e.g., `7d`)
- `w` - Weeks (e.g., `2w`)
- `mo` - Months (e.g., `3mo`)
- `y` - Years (e.g., `1y`)

### Size Format

Size filters support these formats:
- `B` - Bytes
- `KB` - Kilobytes
- `MB` - Megabytes
- `GB` - Gigabytes
- `TB` - Terabytes

Examples: `100MB`, `1.5GB`, `512KB`

### Filter Examples

#### Container Filters

```
age>1h                          # Containers older than 1 hour
age<30m                         # Containers younger than 30 minutes
state=running                   # Only running containers
state=exited                    # Only exited containers
name~redis                      # Containers with "redis" in name
age>1d,state=running            # Running containers older than 1 day
name~nginx,state=exited         # Exited containers with "nginx" in name
```

#### Image Filters

```
age>7d                          # Images older than 7 days
tag~ubuntu                      # Images with "ubuntu" in tag
size>100MB                      # Images larger than 100MB
age>30d,size>500MB              # Large old images (cleanup candidates)
tag~:latest                     # Images tagged as latest
```

#### Network Filters

```
driver=bridge                   # Bridge networks only
name~custom                     # Networks with "custom" in name
scope=local                     # Local scope networks
```

#### Volume Filters

```
age>30d                         # Volumes older than 30 days
name~data                       # Volumes with "data" in name
driver=local                    # Local driver volumes
```

### Key Bindings

- `/` - Open filter input
- `Enter` - Apply filter
- `ESC` - Cancel filter input
- `Ctrl+U` - Clear filter input text
- `c` - Clear active filter (from main view)

### Status Bar

When a filter is active, the status bar shows:
- The current filter criteria
- `c` key to clear the filter
- Other available key bindings

When in filter input mode, the status bar shows:
- Filter syntax examples
- Available key bindings

### Implementation Details

#### Filter Architecture

- **Package**: `internal/filter`
- **Parser**: Converts filter strings into structured criteria
- **Evaluator**: Applies filter criteria to resources
- **UI Integration**: Seamless integration with existing table views

#### Performance

- Filters are applied in-memory after fetching resources
- No additional Docker API calls for filtering
- Debounced to avoid UI blocking
- Works with existing async data loading

#### Error Handling

Invalid filter syntax shows an error message in the status bar with details about what went wrong.

### Testing

Comprehensive test coverage includes:
- Filter parsing with various operators
- Duration and size parsing
- Criterion matching for all resource types
- Edge cases and error conditions

Run tests with:
```bash
go test ./internal/filter/...
```

## Future Enhancements

Potential improvements documented in `docs/feature_ideas.md`:
- Filter persistence across sessions
- Quick filter presets (F1, F2, etc.)
- Filter by resource consumption (CPU%, memory%)
- Export filtered results
- Filter via CLI flags for scripted usage

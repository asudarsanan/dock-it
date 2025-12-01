# Architecture Overview

## High-Level Layout

```
cmd/dock-it/main.go  # entry point
internal/app/        # wiring + orchestration
internal/docker/     # Docker SDK wrapper
internal/logs/       # log parsing & colorization
internal/ui/         # tview terminal UI
```

## Execution Flow

1. `cmd/dock-it/main.go` calls `internal/app.Run()`.
2. `internal/app` constructs a Docker client (`internal/docker`) and passes it to the UI (`internal/ui`).
3. UI orchestrates async resource listing, describe actions, and log streaming, calling back into docker helpers.
4. Logs fetched from containers are colorized via `internal/logs` before display.

## Key Design Considerations

- **Context timeouts** guard all Docker API calls to keep the UI responsive.
- **Worker pools** limit concurrent stats collection for running containers.
- **Async UI updates** use `QueueUpdateDraw` to avoid blocking `tview`'s event loop.
- **Detail pane** consolidates describe/log views, keeping list navigation intact.

## Future Enhancements

- Extract reusable view components for additional resource types.
- Add configuration (refresh intervals, theme) via a dedicated settings package.
- Explore plugin-style commands under `cmd/` for automation utilities.

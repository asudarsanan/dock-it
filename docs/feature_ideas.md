# Next Feature Ideation

Date: 2025-12-01

## Overview
- Capture the near-term feature backlog for dock-it with enough detail to evaluate scope.
- Focus areas: UI themes, table filtering (by age, status, name), resource consumption surfacing, and image-specific tooling.

## Goals
1. Improve readability through theme presets similar to k9s (dark/light, high contrast).
2. Allow operators to quickly slice large container/image lists by relevant attributes.
3. Surface resource consumption context (CPU, memory, network I/O) to inform cleanup decisions.
4. Extend capabilities to Docker images, enabling filtering and bulk actions.

## Theme System
- **Presets**: `dark` (current), `light`, `high-contrast`, and `solarized` candidates.
- **Runtime Toggle**: hotkey (e.g., `T`) to cycle themes plus saved preference (config file or env var).
- **Custom Palette**: optional YAML/JSON theme definition for advanced users.
- **Implementation Notes**:
  - Wrap current `tview.Styles` overrides behind a `Theme` struct.
  - Apply palette to table row colors, status bar, detail view, and future dialogs.
  - Consider per-theme glyphs (status indicators) for accessibility.

## Filtering & Search
- **Interactive Filter Bar**: toggle via `/`, showing current filter chips.
- **Criteria**:
  - `age`: derived from container `Created` timestamp (e.g., `>24h`, `last 15m`).
  - `status`: running, exited, paused, restarting, dead.
  - `name`: substring/regex match with case-insensitive option.
  - `consumption`: thresholds on CPU%, memory%, net I/O (e.g., `cpu>50`).
- **UX Ideas**:
  - Use `:` commands (`status=running`, `age>1h`).
  - Provide quick presets via keys (`F1` recent, `F2` high CPU, etc.).
  - Show active filters in status bar with `x` to clear.
- **Data Requirements**:
  - Already collecting stats for running containers; need cached stats for filtering without blocking UI.
  - For age filtering, persist `Created` metadata from Docker API.

## Image Enhancements
- **Filtering**: by tag regex, size ranges, creation age, dangling state.
- **Consumption Context**: highlight images backing running containers vs unused.
- **Actions**:
  - Bulk delete by filter result with confirmation summary.
  - Export list (text/JSON) for audits.
- **UI**:
  - Consider dedicated panel showing total disk usage, top N images by size.

## Technical Considerations
- Debounce filter application to avoid hammering Docker API.
- Keep asynchronous workers capped (reuse `maxStatsWorkers`).
- Introduce query language parser to translate input into predicates.
- Extend tests around predicate evaluation + theme serialization.

## Open Questions
1. Should filters persist across sessions or reset on launch?
2. Do we need role-specific themes (e.g., colorblind-friendly palette)?
3. How to expose filters via CLI flags for scripted usage?
4. What is the acceptable latency for recomputing stats when filters change?

---
Use this doc as a living plan; append design decisions or spike notes as features progress.

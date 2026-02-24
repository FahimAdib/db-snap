# db-snap

`db-snap` is a local-first PostgreSQL snapshot and restore tool designed for fast, repeatable development and QA workflows.

It provides:

- a terminal UI for profile/snapshot/restore flows
- CLI commands for scripting and automation
- safety checks to prevent accidental restores to risky targets

## Features

- Snapshot creation from a selected PostgreSQL profile
- Snapshot restore with planning and safety warnings
- Schema-drift-aware restore flow with auto-fill/rule support
- Import/export snapshots as `.tar.gz`
- Local audit history for restore/snapshot operations
- Cross-platform binaries (macOS + Linux)

## Installation

### npm (recommended)

```bash
npm i -g @fahimadib01/db-snap
```

### GitHub Releases (binary)

Download the correct archive from the Releases page and place `db-snap` on your `PATH`:

- https://github.com/FahimAdib/db-snap/releases

## Requirements

- macOS or Linux
- PostgreSQL client tools available in `PATH`:
  - `pg_dump`
  - `pg_restore`

## Quick Start

Launch the TUI:

```bash
db-snap tui
```

Or use the CLI directly:

```bash
db-snap profile add --name local --host localhost --port 5432 --database app_db --user postgres
db-snap snapshot create --profile local --tag baseline
db-snap snapshot list --profile local
db-snap restore --profile local --snapshot <snapshot-id> --dry-run
```

## Safety Model

`db-snap` is built for local/dev usage and applies host-policy checks before risky operations. The policy can be managed from the TUI Settings section or via CLI.

## Project Layout

- `cmd/db-snap` — binary entrypoint
- `internal/cli` — Cobra CLI commands
- `internal/tui` — Bubble Tea TUI app
- `internal/snapshot` — snapshot/restore service logic
- `internal/policy` — host safety validation

## Development

### Build

```bash
go build ./...
```

### Test

```bash
go test ./...
```

### Vet

```bash
go vet ./...
```

## Contributing

Contributions are welcome.

1. Fork the repository and create a feature branch.
2. Keep changes focused and aligned with existing project patterns.
3. Run validation locally before opening a PR:

   ```bash
   go test ./...
   go build ./...
   go vet ./...
   ```

4. Open a PR with:
   - clear problem statement
   - implementation summary
   - any UX/behavior changes called out explicitly

## Release Process

Releases are automated through GitHub Actions on version tags (`v*`). Each release publishes:

- GitHub release artifacts via GoReleaser
- npm package `@fahimadib01/db-snap`

## License

MIT

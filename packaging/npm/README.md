# @fahimadib01/db-snap

`db-snap` is a local Postgres snapshot/restore CLI + TUI for repeatable test data setup.

## Install

```bash
npm i -g @fahimadib01/db-snap
```

## Requirements

- macOS or Linux
- `pg_dump` and `pg_restore` available in your `PATH`

## Quick start

Launch the interactive app:

```bash
db-snap tui
```

From the TUI you can:

- create/edit DB profiles
- create/import/export snapshots
- plan and run restores
- manage safety policy and rules

## CLI usage

```bash
db-snap version
db-snap profile list
db-snap snapshot list --profile <name>
db-snap restore --profile <name> --snapshot <snapshot-id> --dry-run
```

## Update

```bash
npm update -g @fahimadib01/db-snap
```

## Data location

`db-snap` stores local app state under:

```bash
~/.db-snap
```

[![Build and Publish](https://github.com/Tensai75/plex-user-sync-go/actions/workflows/build_and_publish.yml/badge.svg)](https://github.com/Tensai75/plex-user-sync-go/actions/workflows/build_and_publish.yml)
[![Latest Release)](https://img.shields.io/github/v/release/Tensai75/plex-user-sync-go?logo=github)](https://github.com/Tensai75/plex-user-sync-go/releases/latest)

# Plex User Sync Go

Plex User Sync Go synchronizes Plex watch history between users by updating the Plex Media Server database. Run one-off syncs or schedule recurring jobs.

This tool was created for my personal use and is provided as-is, without any guarantee that it will work for your environment or use case. Use it at your own risk.

**Warning: This tool directly edits the Plex database. Back up the database before using it.**

## Features

- Sync watch history and in-progress viewing state from one user to another
- Optional bi-directional sync
- Dry-run mode to preview changes
- Bulk mode to replace the target user's watch history with the source user's history
- Daemon mode for periodic syncs
- YAML configuration for multiple sync jobs
- Optional logging with log levels and log file rotation

## Requirements

- Access to the Plex Media Server database file with permission to read and modify it. Elevated privileges may be needed depending on the database file's ownership and permissions.
- Go 1.26 or newer to build from source

## Build

CGO must be disabled when building. Use the command for your shell:

```bash
# Linux/macOS
CGO_ENABLED=0 go build -o plex-user-sync-go .
```

```powershell
# Windows PowerShell
$env:CGO_ENABLED = "0"
go build -o plex-user-sync-go.exe .
```

Pre-built binaries for Linux (amd64 and arm64), macOS (amd64 and arm64), and Windows (amd64) are available from [Releases](https://github.com/Tensai75/plex-user-sync-go/releases). Linux releases also include `.deb` packages.

## Usage

On Linux and macOS:

```bash
./plex-user-sync-go --help
```

On Windows, run `plex-user-sync-go.exe --help`.

Without a jobs file, provide exactly two Plex usernames: the source followed by the target. The users must be different.

### One-way sync

```bash
./plex-user-sync-go user1 user2
```

This copies newer watch state from `user1` to `user2`.

### Dry run

```bash
./plex-user-sync-go --dry-run user1 user2
```

Preview the changes without modifying the watch history.

### Bi-directional sync

```bash
./plex-user-sync-go --bi-directional user1 user2
```

This synchronizes in both directions, comparing watch timestamps and in-progress state.

### Bulk sync

```bash
./plex-user-sync-go --bulk user1 user2
```

Bulk mode deletes the target user's existing watch history, then copies the source user's watch history. The operation is one-way; the command line rejects combining `--bulk` with `--bi-directional` or `--daemon`. Use `--dry-run` to preview how many source items would be copied.

### YAML jobs

```bash
./plex-user-sync-go --jobs-config jobs.yaml
```

This runs every configured job once. Add `--daemon` to run each job repeatedly at its configured interval. `--dry-run` applies to all jobs.

Example:

```yaml
jobs:
  - source_user: John Doe
    target_user: Dave
    bi_directional: false
    interval_minutes: 5

  - source_user: Charlie
    target_user: Alice
    bi_directional: true
    interval_minutes: 10
```

Each job needs distinct `source_user` and `target_user` names and a positive `interval_minutes` value. The interval is used in daemon mode; it is still required when running jobs once. See [jobs.yaml](jobs.yaml) for a working example.

### Daemon mode

```bash
./plex-user-sync-go --daemon --interval 5 user1 user2
```

This runs the sync every 5 minutes. For YAML jobs, each job uses its own `interval_minutes`. A daemon stops an individual job after 3 consecutive errors by default; change this limit with `--max-errors`.

## Options

| Option              | Short | Description                                                                    |
| ------------------- | ----- | ------------------------------------------------------------------------------ |
| `--db`              | `-d`  | Plex database file path                                                        |
| `--dry-run`         | `-n`  | Preview changes without modifying the watch history                            |
| `--bi-directional`  | `-b`  | Synchronize in both directions                                                 |
| `--bulk`            | `-k`  | Replace the target's watch history with the source's                           |
| `--daemon`          | `-D`  | Repeatedly run sync jobs                                                       |
| `--interval`        | `-i`  | Single-job daemon interval in minutes (default: 5)                             |
| `--max-errors`      | `-e`  | Consecutive errors allowed per daemon job (default: 3)                         |
| `--jobs-config`     | `-j`  | YAML file containing sync jobs                                                 |
| `--log`             | `-l`  | Enable logging to a file                                                       |
| `--log-level`       | `-L`  | Log level (0 - 6; default: 2 = Info)                                           |
| `--log-file`        | `-f`  | Log file path (used with `--log`)                                              |
| `--log-max-size`    |       | Maximum log size in MB before rotation (default: 50; `0` disables rotation)    |
| `--log-max-backups` |       | Maximum rotated log files to keep (default: 5; `0` keeps all)                  |
| `--log-max-age`     |       | Maximum age in days for rotated logs (default: 30; `0` disables age cleanup)   |
| `--log-compress`    |       | Compress rotated logs (default: `true`; use `--log-compress=false` to disable) |
| `--log-console`     |       | Write logs to console (default: `true`; use `--log-console=false` to disable)  |
| `--busy-timeout`    |       | SQLite busy timeout in seconds (default: 60)                                   |

## Plex database location

The program attempts to detect the database path for the current platform:

- Linux: checks common Plex and NAS installation locations; if `PLEX_HOME` is set, it checks under that directory.
- macOS: `~/Library/Application Support/Plex Media Server/Plug-in Support/Databases/com.plexapp.plugins.library.db`
- Windows: `%LOCALAPPDATA%\Plex Media Server\Plug-in Support\Databases\com.plexapp.plugins.library.db`

If detection does not find the database, provide its path with `--db` / `-d`.

## Logging

File logging is optional and must be enabled with `--log` / `-l`; this works in both one-off and daemon modes.
The log information is also written to the console by default. To disable console logging, use `--log-console=false`.

Default log file locations:

- Linux and macOS: `/var/log/plex-user-sync-go.log`, falling back to the system temporary directory if `/var/log` does not exist.
- Windows: `%LOCALAPPDATA%\PlexUserSyncGo\plex-user-sync-go.log`, falling back to the system temporary directory if `%LOCALAPPDATA%` is unavailable.

Set a different log path with `--log-file` / `-f` while enabling file logging, for example:

```bash
./plex-user-sync-go --log --log-file ./plex-user-sync-go.log user1 user2
```

By default, logs rotate when the active file reaches 50 MB. Up to 5 rotated files are kept, files older than 30 days are removed during rotation cleanup, and rotated files are compressed. For example, `--log-max-size 10 --log-max-backups 3 --log-max-age 14` sets a 10 MB limit, keeps at most 3 backups, and removes backups older than 14 days. Set `--log-max-size 0` to disable rotation; the backup, age, and compression options then have no effect. Set `--log-compress=false` to keep rotated files uncompressed.

## Important

- **This tool directly edits Plex database files. Back up the database before use.**
- Use `--dry-run` to preview changes before making them.
- Daemon jobs continue until stopped or until their configured consecutive-error limit is reached.

## AI assistance

AI tools were used in part of this project, including assistance with the README, SQL statements and for code review. The application code was written by me.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

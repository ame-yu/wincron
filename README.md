[中文README](./README.zh-CN.md)

# wincron

A lightweight task scheduler for Windows. Create cron-like jobs to run commands on a schedule, and view execution logs.

## Features

- Designed to be lightweight and low-overhead when idle
- Cron-style scheduling for command execution (robfig/cron)
- Run any executable with arguments and a working directory
- Run now + preview run to validate commands before scheduling
- Detailed execution logs (stdout/stderr/exit code) with log cleanup
- Failure protection: auto-disable after N consecutive failures
- YAML import/export for jobs (and optional settings), with conflict strategies
- Open the local data directory for easy backup/migration
- Tray-friendly behavior, optional silent start, and auto-start on login
- Desktop UI built with Wails3 + Vue3

## Screenshots

![Screenshot1-HomePage](./docs/preview1.png)
![Screenshot-Settings](./docs/preview2.png)

## Installation

### Download Release

- Download zip from the GitHub Releases page.

### Build from Source

1. Install Wails3:

   `go install -v github.com/wailsapp/wails/v3/cmd/wails3@latest`

2. Install frontend dependencies:

   `bun install`

3. Run in development mode:

   `task dev`

## Quick Start

1. Start the app.
2. Create a job:
   - Name
   - Cron expression (e.g. `*/5 * * * *`)
   - Command and arguments
   - Working directory (optional)
3. Save and enable the job.

## Development Environment

- Go: `1.25` (see `go.mod`)
- Wails: `v3.0.0-alpha.60` (see `go.mod`)
- Frontend runtime: `@wailsio/runtime@3.0.0-alpha.78` (see `frontend/package.json`)
- Bun: recommended (repo includes `frontend/bun.lock`)
- Node.js: optional if you prefer npm/pnpm over Bun (recommend 18+)

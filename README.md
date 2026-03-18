[中文README](./README.zh-CN.md)

# wincron

A lightweight task scheduler for Windows. I hope it is more intuitive than the built-in Task Scheduler.

## Features

- Lightweight design: single executable; low overhead when idle
- Schedule tasks using cron expressions
- Supports command arguments and working directory
- Execution logs
- Advanced job options: concurrency policy / disable after consecutive failures
- Silent start / run on boot / lightweight mode / tray mode
- Global hotkeys
- Import/export
- IPC command-line control

## Screenshots

![Screenshot1-HomePage](./docs/preview1.png)
![Screenshot-Settings](./docs/preview2.png)

## Installation

> Make sure [WebView2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) is installed.

### Download Release

- Download the zip from the GitHub Releases page.

### Build from Source

1. Install Wails3:

   `go install -v github.com/wailsapp/wails/v3/cmd/wails3@latest`

2. Build (outputs to `bin/` by default):

   `wails3 build`

   This produces `bin/wincron.exe` and the CLI helper `bin/wincronctl.exe`.

## Development Environment

- Go: `1.25`
- Wails: `v3.0.0-alpha.71`
- Bun: `v1.0` or above
- Node.js: optional (if you don't use Bun)

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"wincron/internal/ipc"
)

const cliHelpText = `Usage:
  wincronctl <command>

Commands:
  open
      Show the WinCron window
  status
      Show the global enabled state
  enable [job name|folder]
      Enable WinCron or matching jobs
  disable [job name|folder]
      Disable WinCron or matching jobs
  run <job name>
      Run matching jobs immediately
  import <yaml-file|-> [--overwrite|--coexist]
      Import jobs from YAML
  quit
      Ask the WinCron GUI process to exit

Tips:
  - Run 'wincronctl open' if you want to bring the app to the front
  - Run 'wincronctl import --help' for import YAML details
`

const importHelpText = `Usage:
  wincronctl import <yaml-file|-> [--overwrite|--coexist]

Supported YAML formats:
  1. A raw YAML list of jobs
  2. An exported object with version/settings/jobs

Recommendations:
  - For AI-generated imports, prefer the raw job list format
  - Use stable unique job names, for example AI/daily-report
  - Use @reboot for startup jobs; do not set runAtStartup manually
  - Do not write runtime fields: id, consecutiveFailures, executedCount, lastExecutedAt, nextRunAt

Useful values:
  - concurrencyPolicy: skip | kill_old | allow
  - flagProcessCreation: CREATE_NEW_CONSOLE | CREATE_NO_WINDOW | DETACHED_PROCESS

Examples:
  wincronctl import .\task.yml --overwrite
  Get-Content .\task.yml -Raw | wincronctl import - --overwrite
  wincronctl import --example

See docs/import-yaml.md in the repository for a complete example.
`

const importUsage = "usage: wincronctl import <yaml-file|-> [--overwrite|--coexist]"

const importExampleYAML = `# AI-friendly format: use a raw YAML list of jobs.
# Do not write id, runAtStartup, consecutiveFailures, executedCount, lastExecutedAt, or nextRunAt.
# Use @reboot for startup jobs. WinCron derives runAtStartup automatically.
- name: AI/daily-report
  folder: AI
  cron: "0 9 * * 1-5"
  command: "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe"
  args:
    - -NoProfile
    - -File
    - C:\\Scripts\\daily-report.ps1
  workDir: "C:\\Scripts"
  inheritEnv: true
  timeout: 600
  concurrencyPolicy: skip
  enabled: true
  maxConsecutiveFailures: 3

- name: AI/on-boot-check
  cron: "@reboot"
  command: "cmd.exe"
  args:
    - /c
    - echo system booted
  enabled: true

# Full export-compatible format is also supported:
# version: 6
# settings:
#   runInTray: true
# jobs:
#   - name: Example
#     cron: "0 * * * *"
#     command: "cmd.exe"
#     args: ["/c", "echo hello"]
`

func main() {
	args := os.Args[1:]
	if output, ok := localControlOutput(args); ok {
		fmt.Print(output)
		return
	}
	if isControlCommand(args) {
		os.Exit(runControlCommand(args))
	}

	guiPath, err := guiExecutablePath()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	if err := runDetached(guiPath, args); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func guiExecutablePath() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(self), "wincron.exe"), nil
}

func isControlCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "disable", "enable", "status", "quit", "open", "run", "import":
		return true
	default:
		return false
	}
}

func runControlCommand(args []string) int {
	req, err := parseControlRequest(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	resp, err := ipc.SendRequest(req)
	if err != nil {
		if ipc.IsLikelyPipeNotRunning(err) {
			fmt.Fprintln(os.Stderr, "wincron is not running")
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		return 2
	}
	if !resp.Ok {
		msg := strings.TrimSpace(resp.Error)
		if msg == "" {
			msg = strings.TrimSpace(resp.Message)
		}
		if msg == "" {
			msg = "request failed"
		}
		fmt.Fprintln(os.Stderr, msg)
		return 1
	}

	if req.Cmd == "status" {
		if resp.GlobalEnabled != nil {
			if *resp.GlobalEnabled {
				fmt.Println("enabled")
			} else {
				fmt.Println("disabled")
			}
		} else if strings.TrimSpace(resp.Message) != "" {
			fmt.Println(resp.Message)
		}
	} else if strings.TrimSpace(resp.Message) != "" {
		fmt.Println(resp.Message)
	}

	return 0
}

func localControlOutput(args []string) (string, bool) {
	if len(args) == 0 {
		return cliHelpText, true
	}
	if len(args) == 1 {
		switch strings.ToLower(strings.TrimSpace(args[0])) {
		case "help", "--help", "-h":
			return cliHelpText, true
		}
	}
	if len(args) != 2 {
		return "", false
	}
	if !strings.EqualFold(strings.TrimSpace(args[0]), "import") {
		return "", false
	}

	switch strings.ToLower(strings.TrimSpace(args[1])) {
	case "--help", "-h":
		return importHelpText, true
	case "--example", "--template":
		return importExampleYAML, true
	default:
		return "", false
	}
}

func parseControlRequest(args []string) (ipc.Request, error) {
	if len(args) == 0 {
		return ipc.Request{}, errors.New("usage: wincronctl <command>")
	}

	cmd := strings.ToLower(strings.TrimSpace(args[0]))
	req := ipc.Request{Cmd: cmd}

	switch cmd {
	case "enable", "disable":
		if len(args) > 1 {
			req.Target = strings.Join(args[1:], " ")
		}
	case "run":
		if len(args) < 2 {
			return ipc.Request{}, errors.New("usage: wincronctl run <job name>")
		}
		req.Target = strings.Join(args[1:], " ")
	case "import":
		payload, strategy, err := parseImportArgs(args[1:])
		if err != nil {
			return ipc.Request{}, err
		}
		req.Payload = payload
		req.ConflictStrategy = strategy
	}

	return req, nil
}

func parseImportArgs(args []string) (string, string, error) {
	if len(args) == 0 {
		return "", "", errors.New(importUsage)
	}

	source := ""
	strategy := "overwrite"
	for _, raw := range args {
		arg := strings.TrimSpace(raw)
		switch strings.ToLower(arg) {
		case "--overwrite":
			strategy = "overwrite"
		case "--coexist":
			strategy = "coexist"
		default:
			if strings.HasPrefix(arg, "--") {
				return "", "", fmt.Errorf("unknown import option: %s", raw)
			}
			if source != "" {
				return "", "", errors.New(importUsage)
			}
			source = raw
		}
	}

	if strings.TrimSpace(source) == "" {
		return "", "", errors.New(importUsage)
	}

	payload, err := readImportPayload(source)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(payload) == "" {
		return "", "", errors.New("import payload is empty")
	}
	return payload, strategy, nil
}

func readImportPayload(source string) (string, error) {
	if strings.TrimSpace(source) == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read import stdin: %w", err)
		}
		return string(b), nil
	}

	filePath := source
	if abs, err := filepath.Abs(source); err == nil {
		filePath = abs
	}
	b, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read import file %s: %w", filePath, err)
	}
	return string(b), nil
}

func runDetached(path string, args []string) error {
	cmd := exec.Command(path, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008 | 0x08000000,
		HideWindow:    true,
	}
	return cmd.Start()
}

//go:build windows

package main

import (
	"log"
	"os"
	"syscall"
)

func enableConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	attachConsole := kernel32.NewProc("AttachConsole")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	allocConsole := kernel32.NewProc("AllocConsole")

	_, _, _ = attachConsole.Call(uintptr(0xFFFFFFFF))

	w, _, _ := getConsoleWindow.Call()
	if w == 0 {
		_, _, _ = allocConsole.Call()
	}

	stdout, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err == nil {
		os.Stdout = stdout
		os.Stderr = stdout
		log.SetOutput(stdout)
	}

	stdin, err := os.OpenFile("CONIN$", os.O_RDONLY, 0)
	if err == nil {
		os.Stdin = stdin
	}
}

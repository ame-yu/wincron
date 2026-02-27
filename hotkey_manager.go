package main

type HotkeyManager interface {
	Start()
	SetBinding(jobID string, hotkey string) error
	SetActive(active bool) error
	Stop()
}

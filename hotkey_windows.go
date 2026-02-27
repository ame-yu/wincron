package main

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmHotkey = 0x0312
	wmQuit   = 0x0012
	wmWake   = 0x8001
	modNoRepeat = 0x4000
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey    = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey  = user32.NewProc("UnregisterHotKey")
	procPeekMessageW      = user32.NewProc("PeekMessageW")
	procGetMessageW       = user32.NewProc("GetMessageW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procDispatchMessageW  = user32.NewProc("DispatchMessageW")
	procPostThreadMessage = user32.NewProc("PostThreadMessageW")
)

type hotkeyBinding struct {
	jobID  string
	hotkey string
	mod    uint32
	vk     uint32
	id     int32
}

type hotkeyCommand interface{ isHotkeyCommand() }

type hotkeyCmdSetBinding struct {
	jobID  string
	hotkey string
	resp   chan error
}

func (*hotkeyCmdSetBinding) isHotkeyCommand() {}

type hotkeyCmdSetActive struct {
	active bool
	resp   chan error
}

func (*hotkeyCmdSetActive) isHotkeyCommand() {}

type hotkeyCmdStop struct {
	resp chan struct{}
}

func (*hotkeyCmdStop) isHotkeyCommand() {}

type winPOINT struct {
	X int32
	Y int32
}

type winMSG struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       winPOINT
	LPrivate uint32
}

type windowsHotkeyManager struct {
	startOnce sync.Once

	cmdCh chan hotkeyCommand

	mu       sync.Mutex
	threadID uint32
	running  bool

	onHotkey func(jobID string)
}

func newWindowsHotkeyManager(onHotkey func(jobID string)) *windowsHotkeyManager {
	return &windowsHotkeyManager{
		cmdCh:    make(chan hotkeyCommand, 64),
		onHotkey: onHotkey,
	}
}

func (m *windowsHotkeyManager) Start() {
	m.startOnce.Do(func() {
		go m.loop()
	})
}

func (m *windowsHotkeyManager) SetBinding(jobID string, hotkey string) error {
	m.Start()
	resp := make(chan error, 1)
	m.cmdCh <- &hotkeyCmdSetBinding{jobID: jobID, hotkey: hotkey, resp: resp}
	m.wake()
	return <-resp
}

func (m *windowsHotkeyManager) SetActive(active bool) error {
	m.Start()
	resp := make(chan error, 1)
	m.cmdCh <- &hotkeyCmdSetActive{active: active, resp: resp}
	m.wake()
	return <-resp
}

func (m *windowsHotkeyManager) Stop() {
	m.mu.Lock()
	running := m.running
	m.mu.Unlock()
	if !running {
		return
	}
	resp := make(chan struct{}, 1)
	m.cmdCh <- &hotkeyCmdStop{resp: resp}
	m.wake()
	<-resp
}

func (m *windowsHotkeyManager) wake() {
	m.mu.Lock()
	tid := m.threadID
	m.mu.Unlock()
	if tid == 0 {
		return
	}
	_, _, _ = procPostThreadMessage.Call(uintptr(tid), uintptr(wmWake), 0, 0)
}

func (m *windowsHotkeyManager) loop() {
	runtime.LockOSThread()

	m.mu.Lock()
	m.threadID = windows.GetCurrentThreadId()
	m.running = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		runtime.UnlockOSThread()
	}()

	active := true
	var msg winMSG
	_, _, _ = procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0)

	bindings := map[string]*hotkeyBinding{}
	idToJob := map[int32]string{}
	var nextID int32 = 1

	unregisterAll := func() {
		for _, b := range bindings {
			if b == nil || b.id == 0 {
				continue
			}
			_, _, _ = procUnregisterHotKey.Call(0, uintptr(b.id))
			delete(idToJob, b.id)
			b.id = 0
		}
	}

	registerOne := func(b *hotkeyBinding) error {
		if b == nil {
			return nil
		}
		if b.hotkey == "" {
			return nil
		}
		if b.id != 0 {
			return nil
		}
		b.id = nextID
		nextID++
		mod := b.mod | modNoRepeat
		r1, _, err := procRegisterHotKey.Call(0, uintptr(b.id), uintptr(mod), uintptr(b.vk))
		if r1 == 0 {
			b.id = 0
			return fmt.Errorf("RegisterHotKey failed: %w", err)
		}
		idToJob[b.id] = b.jobID
		return nil
	}

	reconcileActive := func() error {
		if !active {
			unregisterAll()
			return nil
		}
		for _, b := range bindings {
			if b == nil {
				continue
			}
			if err := registerOne(b); err != nil {
				return err
			}
		}
		return nil
	}

	handleSetBinding := func(cmd *hotkeyCmdSetBinding) error {
		jobID := strings.TrimSpace(cmd.jobID)
		if jobID == "" {
			return errors.New("jobID is required")
		}
		hk := strings.TrimSpace(cmd.hotkey)
		if hk == "" {
			if prev := bindings[jobID]; prev != nil {
				if prev.id != 0 {
					_, _, _ = procUnregisterHotKey.Call(0, uintptr(prev.id))
					delete(idToJob, prev.id)
				}
				delete(bindings, jobID)
			}
			return nil
		}

		normalized, mod, vk, err := normalizeHotkeyString(hk)
		if err != nil {
			return err
		}

		if prev := bindings[jobID]; prev != nil {
			if prev.id != 0 {
				_, _, _ = procUnregisterHotKey.Call(0, uintptr(prev.id))
				delete(idToJob, prev.id)
			}
		}

		b := &hotkeyBinding{jobID: jobID, hotkey: normalized, mod: mod, vk: vk}
		bindings[jobID] = b
		if active {
			return registerOne(b)
		}
		return nil
	}

	handleCmd := func(cmd hotkeyCommand) (stop bool) {
		switch c := cmd.(type) {
		case *hotkeyCmdSetBinding:
			err := handleSetBinding(c)
			c.resp <- err
			return false
		case *hotkeyCmdSetActive:
			active = c.active
			err := reconcileActive()
			c.resp <- err
			return false
		case *hotkeyCmdStop:
			active = false
			unregisterAll()
			_, _, _ = procPostThreadMessage.Call(uintptr(m.threadID), uintptr(wmQuit), 0, 0)
			c.resp <- struct{}{}
			return true
		default:
			return false
		}
	}

	for {
		select {
		case cmd := <-m.cmdCh:
			if handleCmd(cmd) {
				return
			}
		default:
			goto startupDone
		}
	}

	startupDone:
	for {
		for {
			select {
			case cmd := <-m.cmdCh:
				if handleCmd(cmd) {
					return
				}
			default:
				goto drainDone
			}
		}
	drainDone:

		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		switch int32(ret) {
		case -1:
			return
		case 0:
			return
		}

		if msg.Message == wmWake {
			continue
		}

		if msg.Message == wmHotkey {
			id := int32(msg.WParam)
			jobID := idToJob[id]
			if jobID != "" && m.onHotkey != nil {
				go m.onHotkey(jobID)
			}
			continue
		}

		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}


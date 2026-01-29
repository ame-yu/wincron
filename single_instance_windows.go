//go:build windows

package main

import (
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procCreateMutexW = kernel32.NewProc("CreateMutexW")
	procOpenMutexW   = kernel32.NewProc("OpenMutexW")
)

func hasAnotherWincronProcess() (bool, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(snapshot)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err = windows.Process32First(snapshot, &pe); err != nil {
		return false, err
	}
	self := uint32(os.Getpid())
	for {
		name := windows.UTF16ToString(pe.ExeFile[:])
		if pe.ProcessID != self && strings.EqualFold(name, "wincron.exe") {
			return true, nil
		}
		if err = windows.Process32Next(snapshot, &pe); err != nil {
			if err == syscall.ERROR_NO_MORE_FILES {
				return false, nil
			}
			return false, err
		}
	}
}

func acquireSingleInstanceLock() (release func(), alreadyRunning bool, err error) {
	createMutex := func(name string) (handle windows.Handle, exists bool, accessDenied bool, err error) {
		p, err := windows.UTF16PtrFromString(name)
		if err != nil {
			return 0, false, false, err
		}
		h, _, callErr := procCreateMutexW.Call(0, 1, uintptr(unsafe.Pointer(p)))
		if h == 0 {
			errno, _ := callErr.(syscall.Errno)
			switch errno {
			case windows.ERROR_ACCESS_DENIED:
				return 0, false, true, nil
			case 0:
				return 0, false, false, windows.GetLastError()
			default:
				return 0, false, false, callErr
			}
		}
		handle = windows.Handle(h)
		if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
			_ = windows.CloseHandle(handle)
			return 0, true, false, nil
		}
		return handle, false, false, nil
	}
	openMutex := func(name string) (exists bool, accessDenied bool, err error) {
		p, err := windows.UTF16PtrFromString(name)
		if err != nil {
			return false, false, err
		}
		h, _, callErr := procOpenMutexW.Call(uintptr(windows.SYNCHRONIZE), 0, uintptr(unsafe.Pointer(p)))
		if h == 0 {
			errno, _ := callErr.(syscall.Errno)
			switch errno {
			case windows.ERROR_FILE_NOT_FOUND:
				return false, false, nil
			case windows.ERROR_ACCESS_DENIED:
				return false, true, nil
			case 0:
				return false, false, windows.GetLastError()
			default:
				return false, false, callErr
			}
		}
		_ = windows.CloseHandle(windows.Handle(h))
		return true, false, nil
	}
	localName := "Local\\wincron_single_instance"
	globalName := "Global\\wincron_single_instance"
	localHandle, exists, accessDenied, err := createMutex(localName)
	if err != nil {
		return nil, false, err
	}
	if exists || accessDenied {
		return nil, true, nil
	}
	exists, accessDenied, err = openMutex(globalName)
	if err != nil {
		_ = windows.CloseHandle(localHandle)
		return nil, false, err
	}
	if exists || accessDenied {
		_ = windows.CloseHandle(localHandle)
		return nil, true, nil
	}
	globalHandle, exists, accessDenied, err := createMutex(globalName)
	if err != nil {
		_ = windows.CloseHandle(localHandle)
		return nil, false, err
	}
	if exists {
		_ = windows.CloseHandle(localHandle)
		return nil, true, nil
	}
	if accessDenied {
		globalHandle = 0
	}
	if ok, _ := hasAnotherWincronProcess(); ok {
		if globalHandle != 0 {
			_ = windows.CloseHandle(globalHandle)
		}
		_ = windows.CloseHandle(localHandle)
		return nil, true, nil
	}
	release = func() {
		if globalHandle != 0 {
			_ = windows.CloseHandle(globalHandle)
		}
		_ = windows.CloseHandle(localHandle)
	}
	return release, false, nil
}

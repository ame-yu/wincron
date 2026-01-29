//go:build !windows

package main

func acquireSingleInstanceLock() (release func(), alreadyRunning bool, err error) {
	return func() {}, false, nil
}

package main

import (
	"errors"

	"golang.org/x/sys/windows"
)

// acquireSingleInstance holds a per-session named mutex. ok is false if another copy is running.
func acquireSingleInstance() (release func(), ok bool) {
	name, _ := windows.UTF16PtrFromString(`Local\WowTokenTracker`)
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return func() {}, false
	}
	if err != nil {
		return func() {}, true // can't tell; don't block startup
	}
	return func() { windows.CloseHandle(h) }, true
}

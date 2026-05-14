//go:build windows

package main

import "syscall"

func enableConsoleUTF8() {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	setOut := k32.NewProc("SetConsoleOutputCP")
	setIn := k32.NewProc("SetConsoleCP")
	const cpUTF8 = 65001
	_, _, _ = setOut.Call(cpUTF8)
	_, _, _ = setIn.Call(cpUTF8)
}

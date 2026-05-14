package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"road-proxy-v3/internal/app"
)

const publicServerLockFile = "public-server.lock"

type publicServerLock struct {
	path string
	pid  int
}

func acquirePublicServerLock(layout app.RuntimeLayout) (*publicServerLock, error) {
	path, err := generatedConfigPath(layout, publicServerLockFile)
	if err != nil {
		return nil, err
	}

	lock := &publicServerLock{
		path: path,
		pid:  os.Getpid(),
	}

	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if _, err := fmt.Fprintf(file, "pid=%d\nstarted=%s\n", lock.pid, time.Now().Format(time.RFC3339)); err != nil {
				_ = file.Close()
				_ = os.Remove(path)
				return nil, err
			}
			if err := file.Close(); err != nil {
				_ = os.Remove(path)
				return nil, err
			}
			return lock, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}

		existingPID, _ := readPublicServerLockPID(path)
		if existingPID > 0 && processIsRunning(existingPID) {
			return nil, fmt.Errorf(msg("public.lock_active"), path, existingPID)
		}
		_ = os.Remove(path)
	}

	return nil, fmt.Errorf(msg("public.lock_stale_failed"), path)
}

func (l *publicServerLock) Release() {
	if l == nil || l.path == "" {
		return
	}
	existingPID, err := readPublicServerLockPID(l.path)
	if err == nil && existingPID == l.pid {
		_ = os.Remove(l.path)
	}
}

func readPublicServerLockPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pid=") {
			return strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "pid=")))
		}
	}
	return 0, fmt.Errorf("pid not found in lock file")
}

func processIsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	if pid == os.Getpid() {
		return true
	}

	switch runtime.GOOS {
	case "windows":
		return windowsProcessIsRunning(pid)
	case "linux":
		_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
		return err == nil
	default:
		return true
	}
}

func windowsProcessIsRunning(pid int) bool {
	output, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return true
	}
	needle := fmt.Sprintf("\",\"%d\",", pid)
	return strings.Contains(string(output), needle)
}

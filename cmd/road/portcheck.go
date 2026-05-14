package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"road-proxy-v3/internal/config"
)

func parsePortFromAddr(addr string) (int, error) {
	_, port, err := splitListenAddr(addr)
	return port, err
}

func splitListenAddr(addr string) (string, int, error) {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return "", 0, fmt.Errorf(msg("port.empty_address"))
	}
	idx := strings.LastIndex(trimmed, ":")
	if idx < 0 || idx == len(trimmed)-1 {
		return "", 0, fmt.Errorf(msg("port.invalid_address"), addr)
	}
	host := strings.TrimSpace(trimmed[:idx])
	port, err := strconv.Atoi(strings.TrimSpace(trimmed[idx+1:]))
	if err != nil {
		return "", 0, err
	}
	if port < 0 || port > 65535 {
		return "", 0, fmt.Errorf(msg("port.out_of_range"), port)
	}
	return host, port, nil
}

func ensurePortFree(reader *bufio.Reader, proto string, port int) error {
	proto = strings.ToLower(strings.TrimSpace(proto))
	if proto != "tcp" && proto != "udp" {
		return fmt.Errorf(msg("protocol.unsupported"), proto)
	}

	owners, err := findPortOwners(proto, port)
	if err != nil {
		return fmt.Errorf(msg("port.check_failed"), proto, port, err)
	}
	if len(owners) == 0 {
		return nil
	}

	protoLabel := strings.ToUpper(proto)
	for _, pid := range owners {
		fmt.Printf(msg("port.in_use"), protoLabel, port, pid)
		stop, askErr := askYesNo(reader, msg("process.stop_prompt"), false)
		if askErr != nil {
			return askErr
		}
		if !stop {
			return fmt.Errorf(msg("port.not_free_cancelled"), protoLabel, port)
		}

		if killErr := killProcessByPID(pid); killErr != nil {
			return fmt.Errorf(msg("process.kill_failed"), pid, killErr)
		}
		fmt.Printf(msg("process.terminated"), pid)
		time.Sleep(250 * time.Millisecond)
	}

	after, err := findPortOwners(proto, port)
	if err != nil {
		return fmt.Errorf(msg("port.recheck_failed"), proto, port, err)
	}
	if len(after) > 0 {
		return fmt.Errorf(msg("port.still_in_use"), protoLabel, port)
	}
	return nil
}

func findPortOwners(proto string, port int) ([]int, error) {
	switch proto {
	case "tcp", "udp":
	default:
		return nil, fmt.Errorf(msg("protocol.unsupported"), proto)
	}

	switch runtime.GOOS {
	case "linux":
		return findPortOwnersOnLinux(proto, port)
	case "windows":
		return findPortOwnersOnWindows(proto, port)
	default:
		return nil, nil
	}
}

func findPortOwnersOnWindows(proto string, port int) ([]int, error) {
	out, err := exec.Command("netstat", "-ano", "-p", proto).Output()
	if err != nil {
		return nil, err
	}
	return parseWindowsNetstatOwners(out, proto, port)
}

func parseWindowsNetstatOwners(output []byte, proto string, port int) ([]int, error) {
	proto = strings.ToLower(strings.TrimSpace(proto))
	ownersSet := map[int]struct{}{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.EqualFold(fields[0], proto) {
			continue
		}

		localPort, parseErr := parsePortFromAddr(fields[1])
		if parseErr != nil || localPort != port {
			continue
		}

		if proto == "tcp" {
			if len(fields) < 4 {
				continue
			}
			remote := strings.TrimSpace(fields[2])
			if !(strings.HasSuffix(remote, ":0") || remote == "*:*") {
				continue
			}
		}

		pid, pidErr := strconv.Atoi(fields[len(fields)-1])
		if pidErr != nil || pid <= 0 {
			continue
		}
		ownersSet[pid] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return sortedPIDs(ownersSet), nil
}

var linuxPIDPattern = regexp.MustCompile(`pid=(\d+)`)

func findPortOwnersOnLinux(proto string, port int) ([]int, error) {
	switch proto {
	case "tcp", "udp":
	default:
		return nil, fmt.Errorf(msg("protocol.unsupported"), proto)
	}

	if ssPath, err := exec.LookPath("ss"); err == nil {
		owners, ssErr := findPortOwnersOnLinuxSS(ssPath, proto, port)
		if ssErr == nil {
			return owners, nil
		}
	}

	if lsofPath, err := exec.LookPath("lsof"); err == nil {
		owners, lsofErr := findPortOwnersOnLinuxLSOF(lsofPath, proto, port)
		if lsofErr == nil {
			return owners, nil
		}
	}

	// If probe tools are unavailable/broken on this distro, do not block startup.
	// Real bind conflicts are still surfaced by net.Listen later.
	return nil, nil
}

func findPortOwnersOnLinuxSS(ssPath, proto string, port int) ([]int, error) {
	args := []string{}
	switch proto {
	case "tcp":
		args = []string{"-ltnp"}
	case "udp":
		args = []string{"-lunp"}
	default:
		return nil, fmt.Errorf("unsupported proto: %s", proto)
	}

	out, err := exec.Command(ssPath, args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%v (%s)", err, msg)
	}
	return parseLinuxSSOwners(out, port), nil
}

func parseLinuxSSOwners(output []byte, port int) []int {
	needle := ":" + strconv.Itoa(port)
	ownersSet := map[int]struct{}{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, needle) {
			continue
		}

		matches := linuxPIDPattern.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			pid, convErr := strconv.Atoi(m[1])
			if convErr != nil || pid <= 0 {
				continue
			}
			ownersSet[pid] = struct{}{}
		}
	}

	return sortedPIDs(ownersSet)
}

func findPortOwnersOnLinuxLSOF(lsofPath, proto string, port int) ([]int, error) {
	args := []string{"-nP", "-t"}
	switch proto {
	case "tcp":
		args = append(args, fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN")
	case "udp":
		args = append(args, fmt.Sprintf("-iUDP:%d", port))
	default:
		return nil, fmt.Errorf("unsupported proto: %s", proto)
	}

	out, err := exec.Command(lsofPath, args...).CombinedOutput()
	if err != nil {
		// lsof returns code 1 when there is no match.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}

		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%v (%s)", err, msg)
	}

	return parsePIDListOutput(out), nil
}

func parsePIDListOutput(output []byte) []int {
	ownersSet := map[int]struct{}{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil || pid <= 0 {
			continue
		}
		ownersSet[pid] = struct{}{}
	}

	return sortedPIDs(ownersSet)
}

func sortedPIDs(src map[int]struct{}) []int {
	owners := make([]int, 0, len(src))
	for pid := range src {
		owners = append(owners, pid)
	}
	sort.Ints(owners)
	return owners
}

func killProcessByPID(pid int) error {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F").CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				return err
			}
			return fmt.Errorf("%v (%s)", err, msg)
		}
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

func ensureClientPortAvailable(reader *bufio.Reader, cfg *config.ClientConfig) error {
	port, err := parsePortFromAddr(cfg.ListenAddr)
	if err != nil || port <= 0 {
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(cfg.ListenNetwork)) {
	case "udp":
		return ensurePortFree(reader, "udp", port)
	default:
		return ensurePortFree(reader, "tcp", port)
	}
}

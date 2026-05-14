package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

func collectEndpoints() ([]endpoint, error) {
	switch runtime.GOOS {
	case "windows":
		return collectWindowsEndpoints()
	case "linux":
		return collectLinuxEndpoints()
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func collectWindowsEndpoints() ([]endpoint, error) {
	if endpoints, err := collectWindowsPowerShellEndpoints(); err == nil && len(endpoints) > 0 {
		return endpoints, nil
	}
	return collectWindowsNetstatEndpoints()
}

func collectWindowsNetstatEndpoints() ([]endpoint, error) {
	tcpRaw, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return nil, err
	}
	udpRaw, err := exec.Command("netstat", "-ano", "-p", "udp").Output()
	if err != nil {
		return nil, err
	}

	tcp, err := parseNetstatOutput("tcp", string(tcpRaw))
	if err != nil {
		return nil, err
	}
	udp, err := parseNetstatOutput("udp", string(udpRaw))
	if err != nil {
		return nil, err
	}
	return append(tcp, udp...), nil
}

func collectWindowsPowerShellEndpoints() ([]endpoint, error) {
	tcpRaw, err := exec.Command(
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		"Get-NetTCPConnection | Select-Object LocalAddress,LocalPort,RemoteAddress,RemotePort,State,OwningProcess | ConvertTo-Csv -NoTypeInformation",
	).Output()
	if err != nil {
		return nil, err
	}
	udpRaw, err := exec.Command(
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		"Get-NetUDPEndpoint | Select-Object LocalAddress,LocalPort,OwningProcess | ConvertTo-Csv -NoTypeInformation",
	).Output()
	if err != nil {
		return nil, err
	}

	tcp, err := parseWindowsPowerShellEndpointCSV("tcp", string(tcpRaw))
	if err != nil {
		return nil, err
	}
	udp, err := parseWindowsPowerShellEndpointCSV("udp", string(udpRaw))
	if err != nil {
		return nil, err
	}
	return append(tcp, udp...), nil
}

func parseWindowsPowerShellEndpointCSV(proto, raw string) ([]endpoint, error) {
	reader := csv.NewReader(strings.NewReader(raw))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return []endpoint{}, nil
	}

	header := csvHeaderIndex(records[0])
	idxLocalAddress, okLocalAddress := header["localaddress"]
	idxLocalPort, okLocalPort := header["localport"]
	idxPID, okPID := header["owningprocess"]
	if !okLocalAddress || !okLocalPort || !okPID {
		return nil, fmt.Errorf("missing LocalAddress, LocalPort, or OwningProcess in Windows endpoint CSV")
	}
	idxRemoteAddress, okRemoteAddress := header["remoteaddress"]
	idxRemotePort, okRemotePort := header["remoteport"]
	idxState, okState := header["state"]

	endpoints := make([]endpoint, 0, len(records)-1)
	for _, record := range records[1:] {
		if idxLocalAddress >= len(record) || idxLocalPort >= len(record) || idxPID >= len(record) {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(record[idxPID]))
		if err != nil || pid <= 0 {
			continue
		}
		localPort, err := strconv.Atoi(strings.TrimSpace(record[idxLocalPort]))
		if err != nil || localPort <= 0 {
			continue
		}
		remotePort := 0
		if okRemotePort && idxRemotePort < len(record) {
			if parsed, err := strconv.Atoi(strings.TrimSpace(record[idxRemotePort])); err == nil {
				remotePort = parsed
			}
		}
		remoteAddress := "*"
		if okRemoteAddress && idxRemoteAddress < len(record) && strings.TrimSpace(record[idxRemoteAddress]) != "" {
			remoteAddress = strings.TrimSpace(record[idxRemoteAddress])
		}
		state := ""
		if okState && idxState < len(record) {
			state = strings.ToUpper(strings.TrimSpace(record[idxState]))
		}

		endpoints = append(endpoints, endpoint{
			Proto:      strings.ToLower(proto),
			LocalAddr:  strings.TrimSpace(record[idxLocalAddress]),
			LocalPort:  localPort,
			RemoteAddr: remoteAddress,
			RemotePort: remotePort,
			State:      state,
			PID:        pid,
		})
	}
	return endpoints, nil
}

func csvHeaderIndex(header []string) map[string]int {
	out := map[string]int{}
	for i, field := range header {
		out[strings.ToLower(strings.TrimSpace(field))] = i
	}
	return out
}

func collectLinuxEndpoints() ([]endpoint, error) {
	if endpoints, err := collectLinuxSSEndpoints(); err == nil {
		return endpoints, nil
	}
	if endpoints, err := collectLinuxLsofEndpoints(); err == nil {
		return endpoints, nil
	}
	return collectLinuxProcEndpoints()
}

func parseNetstatOutput(proto, raw string) ([]endpoint, error) {
	lines := strings.Split(raw, "\n")
	out := make([]endpoint, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		if !strings.EqualFold(fields[0], proto) {
			continue
		}

		local := fields[1]
		remote := fields[2]
		state := ""
		pidRaw := ""
		if strings.EqualFold(proto, "tcp") {
			if len(fields) < 5 {
				continue
			}
			state = fields[3]
			pidRaw = fields[4]
		} else {
			pidRaw = fields[len(fields)-1]
		}

		pid, err := strconv.Atoi(pidRaw)
		if err != nil {
			continue
		}

		localHost, localPort, err := parseAddressPort(local)
		if err != nil {
			continue
		}
		remoteHost, remotePort, err := parseAddressPort(remote)
		if err != nil {
			continue
		}

		out = append(out, endpoint{
			Proto:      strings.ToLower(proto),
			LocalAddr:  localHost,
			LocalPort:  localPort,
			RemoteAddr: remoteHost,
			RemotePort: remotePort,
			State:      state,
			PID:        pid,
		})
	}
	return out, nil
}

var ssPIDPattern = regexp.MustCompile(`pid=(\d+)`)

func collectLinuxSSEndpoints() ([]endpoint, error) {
	out, err := exec.Command("ss", "-H", "-tunap").Output()
	if err != nil {
		return nil, err
	}
	return parseSSOutput(string(out))
}

func parseSSOutput(raw string) ([]endpoint, error) {
	lines := strings.Split(raw, "\n")
	out := make([]endpoint, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		proto := strings.ToLower(fields[0])
		if proto != "tcp" && proto != "udp" {
			continue
		}
		pid := parseFirstRegexInt(ssPIDPattern, line)
		if pid <= 0 {
			continue
		}
		localHost, localPort, err := parseAddressPort(fields[4])
		if err != nil {
			continue
		}
		remoteHost, remotePort, err := parseAddressPort(fields[5])
		if err != nil {
			continue
		}
		out = append(out, endpoint{
			Proto:      proto,
			LocalAddr:  localHost,
			LocalPort:  localPort,
			RemoteAddr: remoteHost,
			RemotePort: remotePort,
			State:      fields[1],
			PID:        pid,
		})
	}
	return out, nil
}

func collectLinuxLsofEndpoints() ([]endpoint, error) {
	out, err := exec.Command("lsof", "-nP", "-iTCP", "-iUDP").Output()
	if err != nil {
		return nil, err
	}
	return parseLsofOutput(string(out))
}

func parseLsofOutput(raw string) ([]endpoint, error) {
	lines := strings.Split(raw, "\n")
	out := make([]endpoint, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "COMMAND ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || pid <= 0 {
			continue
		}
		protoIdx := -1
		for i, field := range fields {
			if field == "TCP" || field == "UDP" {
				protoIdx = i
				break
			}
		}
		if protoIdx < 0 || protoIdx+1 >= len(fields) {
			continue
		}
		proto := strings.ToLower(fields[protoIdx])
		name := strings.Join(fields[protoIdx+1:], " ")
		name = strings.TrimSpace(strings.Split(name, " (")[0])
		localRaw, remoteRaw := splitFlowAddress(name)
		localHost, localPort, err := parseAddressPort(localRaw)
		if err != nil {
			continue
		}
		remoteHost, remotePort := "*", 0
		if remoteRaw != "" {
			if host, port, parseErr := parseAddressPort(remoteRaw); parseErr == nil {
				remoteHost, remotePort = host, port
			}
		}
		out = append(out, endpoint{
			Proto:      proto,
			LocalAddr:  localHost,
			LocalPort:  localPort,
			RemoteAddr: remoteHost,
			RemotePort: remotePort,
			State:      parseParenthesizedState(strings.Join(fields[protoIdx+1:], " ")),
			PID:        pid,
		})
	}
	return out, nil
}

func splitFlowAddress(raw string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(raw), "->", 2)
	if len(parts) == 1 {
		return strings.TrimSpace(parts[0]), ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func parseParenthesizedState(raw string) string {
	start := strings.LastIndex(raw, "(")
	end := strings.LastIndex(raw, ")")
	if start < 0 || end <= start {
		return ""
	}
	return raw[start+1 : end]
}

func collectLinuxProcEndpoints() ([]endpoint, error) {
	inodeToPID := buildLinuxSocketInodePIDMap("/proc")
	sources := []struct {
		path  string
		proto string
		ipv6  bool
	}{
		{path: "/proc/net/tcp", proto: "tcp"},
		{path: "/proc/net/udp", proto: "udp"},
		{path: "/proc/net/tcp6", proto: "tcp", ipv6: true},
		{path: "/proc/net/udp6", proto: "udp", ipv6: true},
	}
	out := []endpoint{}
	for _, source := range sources {
		data, err := os.ReadFile(source.path)
		if err != nil {
			continue
		}
		out = append(out, parseProcNetOutput(source.proto, string(data), inodeToPID, source.ipv6)...)
	}
	if len(out) == 0 {
		return out, nil
	}
	return out, nil
}

func parseProcNetOutput(proto, raw string, inodeToPID map[string]int, ipv6 bool) []endpoint {
	lines := strings.Split(raw, "\n")
	out := make([]endpoint, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 10 || !strings.HasSuffix(fields[0], ":") {
			continue
		}
		pid := inodeToPID[fields[9]]
		if pid <= 0 {
			continue
		}
		localHost, localPort, err := parseProcAddressPort(fields[1], ipv6)
		if err != nil {
			continue
		}
		remoteHost, remotePort, err := parseProcAddressPort(fields[2], ipv6)
		if err != nil {
			continue
		}
		out = append(out, endpoint{
			Proto:      proto,
			LocalAddr:  localHost,
			LocalPort:  localPort,
			RemoteAddr: remoteHost,
			RemotePort: remotePort,
			State:      procSocketState(fields[3]),
			PID:        pid,
		})
	}
	return out
}

func parseProcAddressPort(raw string, ipv6 bool) (string, int, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), ":", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid proc address: %q", raw)
	}
	port64, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", 0, err
	}
	host := parts[0]
	if !ipv6 && len(host) == 8 {
		host = formatProcIPv4(host)
	}
	return host, int(port64), nil
}

func formatProcIPv4(raw string) string {
	if len(raw) != 8 {
		return raw
	}
	parts := []string{}
	for i := 6; i >= 0; i -= 2 {
		value, err := strconv.ParseUint(raw[i:i+2], 16, 8)
		if err != nil {
			return raw
		}
		parts = append(parts, strconv.Itoa(int(value)))
	}
	return strings.Join(parts, ".")
}

func procSocketState(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "01":
		return "ESTABLISHED"
	case "02":
		return "SYN_SENT"
	case "03":
		return "SYN_RECV"
	case "04":
		return "FIN_WAIT1"
	case "05":
		return "FIN_WAIT2"
	case "06":
		return "TIME_WAIT"
	case "07":
		return "CLOSE"
	case "08":
		return "CLOSE_WAIT"
	case "09":
		return "LAST_ACK"
	case "0A":
		return "LISTEN"
	case "0B":
		return "CLOSING"
	default:
		return raw
	}
}

func buildLinuxSocketInodePIDMap(procRoot string) map[string]int {
	out := map[string]int{}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		fdDir := filepath.Join(procRoot, entry.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if inode := parseSocketInode(target); inode != "" {
				out[inode] = pid
			}
		}
	}
	return out
}

func parseSocketInode(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "socket:[") || !strings.HasSuffix(raw, "]") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(raw, "socket:["), "]")
}

func parseFirstRegexInt(pattern *regexp.Regexp, raw string) int {
	match := pattern.FindStringSubmatch(raw)
	if len(match) < 2 {
		return 0
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return value
}

func parseAddressPort(value string) (string, int, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", 0, fmt.Errorf("empty address")
	}
	if v == "*:*" {
		return "*", 0, nil
	}

	host := ""
	portPart := ""
	if strings.HasPrefix(v, "[") {
		h, p, err := net.SplitHostPort(v)
		if err != nil {
			return "", 0, err
		}
		host = h
		portPart = p
	} else {
		idx := strings.LastIndex(v, ":")
		if idx < 0 {
			return "", 0, fmt.Errorf("missing ':' in %q", v)
		}
		host = v[:idx]
		portPart = v[idx+1:]
	}

	host = strings.Trim(host, "[]")
	if portPart == "" || portPart == "*" {
		return host, 0, nil
	}
	port, err := strconv.Atoi(portPart)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func getProcessInfos() (map[int]processInfo, error) {
	switch runtime.GOOS {
	case "windows":
		return getWindowsProcessInfos()
	case "linux":
		return getLinuxProcessInfos()
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func getWindowsProcessInfos() (map[int]processInfo, error) {
	infos, err := getProcessInfosFromCIM()
	if err == nil && len(infos) > 0 {
		return infos, nil
	}

	names, err := getProcessNamesFromTasklist()
	if err != nil {
		return nil, err
	}
	infos = make(map[int]processInfo, len(names))
	for pid, name := range names {
		infos[pid] = processInfo{Name: name}
	}
	return infos, nil
}

func getLinuxProcessInfos() (map[int]processInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	infos := map[int]processInfo{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		procDir := filepath.Join("/proc", entry.Name())
		info := processInfo{}
		if data, err := os.ReadFile(filepath.Join(procDir, "comm")); err == nil {
			info.Name = strings.TrimSpace(string(data))
		}
		if info.Name == "" {
			if data, err := os.ReadFile(filepath.Join(procDir, "cmdline")); err == nil {
				info.Name = filepath.Base(firstNullSeparatedField(data))
			}
		}
		if exePath, err := os.Readlink(filepath.Join(procDir, "exe")); err == nil {
			info.ExePath = strings.TrimSpace(exePath)
			if info.Name == "" {
				info.Name = filepath.Base(info.ExePath)
			}
		}
		if data, err := os.ReadFile(filepath.Join(procDir, "cmdline")); err == nil {
			info.CommandLine = strings.TrimSpace(strings.ReplaceAll(string(data), "\x00", " "))
		}
		if info.Name != "" || info.ExePath != "" || info.CommandLine != "" {
			infos[pid] = info
		}
	}
	return infos, nil
}

func firstNullSeparatedField(data []byte) string {
	for i, b := range data {
		if b == 0 {
			return string(data[:i])
		}
	}
	return string(data)
}

func getProcessInfosFromCIM() (map[int]processInfo, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		"Get-CimInstance Win32_Process | Select-Object ProcessId,Name,ExecutablePath,CommandLine | ConvertTo-Csv -NoTypeInformation",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(strings.NewReader(string(out)))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return map[int]processInfo{}, nil
	}

	header := map[string]int{}
	for i, field := range records[0] {
		header[strings.ToLower(strings.TrimSpace(field))] = i
	}
	idxPID, okPID := header["processid"]
	idxName, okName := header["name"]
	if !okPID || !okName {
		return nil, fmt.Errorf("missing ProcessId or Name in CIM process output")
	}
	idxPath, okPath := header["executablepath"]
	idxCmd, okCmd := header["commandline"]

	infos := map[int]processInfo{}
	for _, record := range records[1:] {
		if idxPID >= len(record) || idxName >= len(record) {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(record[idxPID]))
		if err != nil || pid <= 0 {
			continue
		}
		info := processInfo{Name: strings.TrimSpace(record[idxName])}
		if okPath && idxPath < len(record) {
			info.ExePath = strings.TrimSpace(record[idxPath])
		}
		if okCmd && idxCmd < len(record) {
			info.CommandLine = strings.TrimSpace(record[idxCmd])
		}
		infos[pid] = info
	}
	return infos, nil
}

func getProcessNamesFromTasklist() (map[int]string, error) {
	out, err := exec.Command("tasklist", "/fo", "csv", "/nh").Output()
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(strings.NewReader(string(out)))
	reader.FieldsPerRecord = -1
	names := map[int]string{}
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 2 {
			continue
		}
		name := strings.TrimSpace(record[0])
		pidStr := digitsOnly(record[1])
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 {
			continue
		}
		names[pid] = name
	}
	return names, nil
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

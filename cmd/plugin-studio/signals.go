package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func enrichCaptureWithProcessSignals(summary *captureSummary, selected processCandidate) {
	if summary == nil {
		return
	}
	summary.ProcessPath = strings.TrimSpace(selected.ExePath)
	if summary.ProcessPath != "" {
		if hash, err := fileSHA256(summary.ProcessPath); err == nil {
			summary.ProcessSHA256 = hash
		}
	}
	if appID, source := detectSteamAppID(selected); appID > 0 {
		summary.SteamAppID = appID
		summary.SteamAppIDSource = source
	}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func detectSteamAppID(selected processCandidate) (int, string) {
	if id := parseSteamAppIDText(selected.CommandLine); id > 0 {
		return id, "command_line"
	}
	if id := parseSteamAppIDText(selected.ExePath); id > 0 {
		return id, "process_path"
	}
	if id := readSteamAppIDFile(selected.ExePath); id > 0 {
		return id, "steam_appid_txt"
	}
	return 0, ""
}

var steamAppIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bsteam_appid\s*[:=]\s*"?(\d{2,10})"?`),
	regexp.MustCompile(`(?i)\bsteamappid\s*[:=]\s*"?(\d{2,10})"?`),
	regexp.MustCompile(`(?i)(?:^|\s)-applaunch\s+(\d{2,10})\b`),
	regexp.MustCompile(`(?i)steam://(?:run|rungameid)/(\d{2,10})\b`),
}

func parseSteamAppIDText(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	for _, pattern := range steamAppIDPatterns {
		match := pattern.FindStringSubmatch(text)
		if len(match) < 2 {
			continue
		}
		id, err := strconv.Atoi(match[1])
		if err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func readSteamAppIDFile(exePath string) int {
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return 0
	}
	path := filepath.Join(filepath.Dir(exePath), "steam_appid.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	id, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"road-proxy-v3/internal/app"
)

func runBuiltinMenu() {
	reader := bufio.NewReader(os.Stdin)

	for {
		showTitle()
		fmt.Println(msg("menu.choose_mode"))
		fmt.Println("  1) " + msg("menu.mode_server"))
		fmt.Println("  2) " + msg("menu.mode_client"))
		fmt.Println("  3) " + msg("menu.mode_public_server"))
		fmt.Println("  4) " + msg("menu.mode_settings"))
		fmt.Println("  5) " + msg("menu.exit"))

		choice, err := readChoice(reader, msg("menu.choice_1_5"), 1, 5, 5)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			fmt.Printf(msg("common.error_line"), err)
			continue
		}

		switch choice {
		case 1:
			if err := startServerFlow(reader); err != nil {
				fmt.Printf(msg("common.error_line"), err)
			}
		case 2:
			if err := startClientFlow(reader); err != nil {
				fmt.Printf(msg("common.error_line"), err)
			}
		case 3:
			if err := startPublicServerWizard(reader); err != nil {
				fmt.Printf(msg("common.error_line"), err)
			}
		case 4:
			if err := startSettingsFlow(reader); err != nil {
				fmt.Printf(msg("common.error_line"), err)
			}
		case 5:
			return
		}
	}
}

func tryRunMenuScript() (bool, error) {
	if runtime.GOOS != "windows" {
		// Linux/macOS should always use built-in menu; PowerShell menu is Windows-focused.
		return false, nil
	}

	if activeLanguage() != "tr" {
		// Built-in menu supports i18n; script is Turkish-only.
		return false, nil
	}

	scriptPath := ""
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		localScript := filepath.Join(exeDir, "scripts", "road-menu.ps1")
		if _, statErr := os.Stat(localScript); statErr == nil {
			scriptPath = localScript
		}

		// Keep source-tree convenience for `go run`.
		if scriptPath == "" && strings.Contains(strings.ToLower(exePath), "go-build") {
			candidate := app.ResolveExistingPath("scripts/road-menu.ps1")
			if _, statErr := os.Stat(candidate); statErr == nil {
				scriptPath = candidate
			}
		}
	}
	if scriptPath == "" {
		return false, nil
	}

	// Prefer Windows PowerShell, fallback to pwsh if available.
	shellNames := []string{"powershell", "pwsh"}
	var shellPath string
	for _, candidate := range shellNames {
		path, err := exec.LookPath(candidate)
		if err == nil {
			shellPath = path
			break
		}
	}
	if shellPath == "" {
		return false, nil
	}

	cmd := exec.Command(
		shellPath,
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return true, cmd.Run()
}

func showTitle() {
	fmt.Println("ROAD Proxy v3")
	fmt.Println("=============")
	fmt.Println()
}

func readLine(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readChoice(reader *bufio.Reader, prompt string, min, max, def int) (int, error) {
	for {
		raw, err := readLine(reader, prompt)
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(raw) == "" {
			return def, nil
		}

		n, convErr := strconv.Atoi(strings.TrimSpace(raw))
		if convErr == nil && n >= min && n <= max {
			return n, nil
		}

		fmt.Println(msg("menu.invalid_choice"))
	}
}

type menuPlugin struct {
	Name           string
	TargetNetwork  string
	ServerTemplate string
	ClientTemplate string
}

type menuState struct {
	SelectedPlugin string `json:"selected_plugin"`
}

func askYesNo(reader *bufio.Reader, prompt string, defaultYes bool) (bool, error) {
	suffix := "(y/N)"
	if defaultYes {
		suffix = "(Y/n)"
	}
	raw, err := readLine(reader, fmt.Sprintf("%s %s: ", prompt, suffix))
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(raw) == "" {
		return defaultYes, nil
	}
	v := strings.ToLower(strings.TrimSpace(raw))
	return v == "y" || v == "yes" || v == "e" || v == "evet", nil
}

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func readText(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := reader.ReadString('\n')
	if errors.Is(err, io.EOF) {
		if strings.TrimSpace(line) == "" {
			return "", io.EOF
		}
		return strings.TrimSpace(line), nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readChoice(reader *bufio.Reader, prompt string, min, max, def int) (int, error) {
	for {
		raw, err := readText(reader, prompt)
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(raw) == "" {
			return def, nil
		}
		n, err := strconv.Atoi(raw)
		if err == nil && n >= min && n <= max {
			return n, nil
		}
		fmt.Println(sm("studio.invalid_choice"))
	}
}

func readIntWithDefault(reader *bufio.Reader, prompt string, def int) (int, error) {
	for {
		raw, err := readText(reader, prompt)
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(raw) == "" {
			return def, nil
		}
		n, err := strconv.Atoi(raw)
		if err == nil {
			return n, nil
		}
		fmt.Println(sm("studio.invalid_number"))
	}
}

func askYesNo(reader *bufio.Reader, prompt string, defaultYes bool) (bool, error) {
	suffix := "(y/N)"
	if defaultYes {
		suffix = "(Y/n)"
	}
	raw, err := readText(reader, fmt.Sprintf("%s %s: ", prompt, suffix))
	if err != nil {
		return false, err
	}
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return defaultYes, nil
	}
	switch value {
	case "y", "yes", "true", "1", "e", "evet":
		return true, nil
	case "n", "no", "false", "0", "h", "hayir", "hayır":
		return false, nil
	default:
		fmt.Println(sm("studio.invalid_choice"))
		return defaultYes, nil
	}
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func terminalWidth() int {
	if columns := os.Getenv("COLUMNS"); columns != "" {
		if width, err := strconv.Atoi(columns); err == nil && width > 0 {
			return width
		}
	}
	return 120
}

func showInTerminal(content string) error {
	if !isTerminal(os.Stdout) || !isTerminal(os.Stdin) {
		_, err := fmt.Print(content)
		return err
	}

	pager := pagerCommand()
	if len(pager) == 0 {
		_, err := fmt.Print(content)
		return err
	}

	cmd := exec.Command(pager[0], pager[1:]...)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func pagerCommand() []string {
	if pager := strings.TrimSpace(os.Getenv("PAGER")); pager != "" {
		parts := strings.Fields(pager)
		if len(parts) == 0 {
			return nil
		}
		if filepath.Base(parts[0]) == "less" {
			parts = append(parts, "-RS")
		}
		return parts
	}
	if less, err := exec.LookPath("less"); err == nil {
		return []string{less, "-RS"}
	}
	if more, err := exec.LookPath("more"); err == nil {
		return []string{more}
	}
	return nil
}

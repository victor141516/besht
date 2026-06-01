package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/victor141516/besht/internal/viewer"
)

func terminalWidth() int {
	if width, ok := liveTerminalWidth(); ok {
		return width
	}
	if width, ok := envColumnsWidth(); ok {
		return width
	}
	return 120
}

func liveTerminalWidth() (int, bool) {
	if width, ok := sttyWidthFromTTY(); ok {
		return width, true
	}
	if !isTerminal(os.Stdin) {
		return 0, false
	}
	out, err := runSttySize(os.Stdin)
	if err != nil {
		return 0, false
	}
	return parseSttyWidth(string(out))
}

func sttyWidthFromTTY() (int, bool) {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return 0, false
	}
	defer tty.Close()
	out, err := runSttySize(tty)
	if err != nil {
		return 0, false
	}
	return parseSttyWidth(string(out))
}

func runSttySize(stdin *os.File) ([]byte, error) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = stdin
	return cmd.Output()
}

func parseSttyWidth(out string) (int, bool) {
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, false
	}
	rows, err := strconv.Atoi(fields[0])
	if err != nil || rows <= 0 {
		return 0, false
	}
	width, err := strconv.Atoi(fields[1])
	if err != nil || width <= 0 {
		return 0, false
	}
	return width, true
}

func envColumnsWidth() (int, bool) {
	columns := os.Getenv("COLUMNS")
	if columns == "" {
		return 0, false
	}
	width, err := strconv.Atoi(columns)
	if err != nil || width <= 0 {
		return 0, false
	}
	return width, true
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

func visualRenderOptions() viewer.RenderOptions {
	if !shouldUseColor() {
		return viewer.RenderOptions{}
	}
	highlight, ok := viewer.NewBatHighlighter()
	if !ok {
		return viewer.RenderOptions{}
	}
	return viewer.RenderOptions{Highlight: highlight}
}

func shouldUseColor() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTerminal(os.Stdout)
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

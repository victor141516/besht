package main

import (
	"bytes"
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
	cleanup, err := configurePagerForVisualization(&pager)
	if err != nil {
		return err
	}
	defer cleanup()

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
		if isLessCommand(parts[0]) {
			parts = lessPagerCommand(parts)
		}
		return parts
	}
	if less, err := exec.LookPath("less"); err == nil {
		return lessPagerCommand([]string{less})
	}
	if more, err := exec.LookPath("more"); err == nil {
		return []string{more}
	}
	return nil
}

func lessPagerCommand(parts []string) []string {
	sanitized := make([]string, 0, len(parts)+2)
	for _, part := range parts {
		if cleaned, ok := withoutLessChopOption(part); ok {
			sanitized = append(sanitized, cleaned)
		}
	}
	return append(sanitized, "-R", "-+S")
}

func withoutLessChopOption(arg string) (string, bool) {
	if arg == "-S" || arg == "--chop-long-lines" {
		return "", false
	}
	if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && !strings.HasPrefix(arg, "-+") && strings.Contains(arg, "S") {
		cleaned := "-" + strings.ReplaceAll(arg[1:], "S", "")
		if cleaned == "-" {
			return "", false
		}
		return cleaned, true
	}
	return arg, true
}

const lessNoHorizontalKeySource = `#command
\kr noaction
\kl noaction
\kR noaction
\kL noaction
\e) noaction
\e( noaction
\e} noaction
\e{ noaction
`

func configurePagerForVisualization(pager *[]string) (func(), error) {
	if len(*pager) == 0 || !isLessCommand((*pager)[0]) || !lessSupportsLesskeySource((*pager)[0]) {
		return func() {}, nil
	}
	path, cleanup, err := writeLessNoHorizontalKeySource()
	if err != nil {
		return nil, err
	}
	*pager = append(*pager, "--lesskey-src="+path)
	return cleanup, nil
}

func isLessCommand(command string) bool {
	return filepath.Base(command) == "less"
}

func lessSupportsLesskeySource(command string) bool {
	out, err := exec.Command(command, "--help").Output()
	return err == nil && bytes.Contains(out, []byte("--lesskey-src"))
}

func writeLessNoHorizontalKeySource() (string, func(), error) {
	file, err := os.CreateTemp("", "besht-lesskey-*")
	if err != nil {
		return "", nil, fmt.Errorf("create less key config: %w", err)
	}
	path := file.Name()
	cleanup := func() {
		_ = os.Remove(path)
	}
	if _, err := file.WriteString(lessNoHorizontalKeySource); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("write less key config: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close less key config: %w", err)
	}
	return path, cleanup, nil
}

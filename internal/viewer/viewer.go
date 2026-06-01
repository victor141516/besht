package viewer

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/victor141516/besht/internal/codegen"
)

type sourceRef struct {
	file string
	line int
}

type compiledLine struct {
	number int
	text   string
}

type group struct {
	source *sourceRef
	shell  []compiledLine
}

type sourceCache struct {
	files map[string][]string
}

// Build compiles entryPath and returns a side-by-side source/shell view.
// Source comments are enabled internally for mapping, but never appear in the
// rendered shell pane.
func Build(entryPath string, opts codegen.Options, width int) (string, error) {
	opts.NoSourceMap = false
	compiled, err := codegen.CompileFile(entryPath, opts)
	if err != nil {
		return "", err
	}
	return Render(compiled, width)
}

func Render(compiled string, width int) (string, error) {
	groups, maxShellLine := parseGroups(compiled)
	cache := sourceCache{files: make(map[string][]string)}
	multipleSources := hasMultipleSources(groups)
	leftLabelWidth := 1
	for _, group := range groups {
		if group.source == nil {
			continue
		}
		if _, err := cache.lineText(*group.source); err != nil {
			return "", err
		}
		leftLabelWidth = max(leftLabelWidth, runeLen(sourceLabel(*group.source, multipleSources)))
	}
	rightLabelWidth := max(1, len(strconv.Itoa(maxShellLine)))

	leftTextWidth, rightTextWidth := paneWidths(width, leftLabelWidth, rightLabelWidth)
	var out strings.Builder
	for _, group := range groups {
		rowCount := max(1, len(group.shell))
		for i := 0; i < rowCount; i++ {
			leftLabel := ""
			leftText := ""
			if i == 0 && group.source != nil {
				leftLabel = sourceLabel(*group.source, multipleSources)
				text, err := cache.lineText(*group.source)
				if err != nil {
					return "", err
				}
				leftText = text
			}

			rightLabel := ""
			rightText := ""
			if i < len(group.shell) {
				rightLabel = strconv.Itoa(group.shell[i].number)
				rightText = group.shell[i].text
			}

			fmt.Fprintf(&out, "%*s | %s || %*s | %s\n",
				leftLabelWidth,
				leftLabel,
				padRight(truncate(leftText, leftTextWidth), leftTextWidth),
				rightLabelWidth,
				rightLabel,
				truncate(rightText, rightTextWidth),
			)
		}
	}
	return out.String(), nil
}

func parseGroups(compiled string) ([]group, int) {
	lines := splitLines(compiled)
	var groups []group
	var current *group
	visibleLine := 0
	flush := func() {
		if current == nil {
			return
		}
		if current.source != nil || len(current.shell) > 0 {
			groups = append(groups, *current)
		}
		current = nil
	}

	for _, line := range lines {
		if ref, ok := parseSourceComment(line); ok {
			if current != nil && current.source != nil && len(current.shell) == 0 && sameSource(*current.source, ref) {
				current.source = &ref
				continue
			}
			flush()
			current = &group{source: &ref}
			continue
		}
		visibleLine++
		if current == nil {
			current = &group{}
		}
		current.shell = append(current.shell, compiledLine{number: visibleLine, text: line})
	}
	flush()
	return groups, visibleLine
}

func sameSource(a, b sourceRef) bool {
	return a.file == b.file && a.line == b.line
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func parseSourceComment(line string) (sourceRef, bool) {
	const prefix = "# besht:"
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, prefix) {
		return sourceRef{}, false
	}
	payload := strings.TrimPrefix(trimmed, prefix)
	lastColon := strings.LastIndex(payload, ":")
	if lastColon < 0 {
		return sourceRef{}, false
	}
	columnPart := payload[lastColon+1:]
	payload = payload[:lastColon]
	lineColon := strings.LastIndex(payload, ":")
	if lineColon < 0 {
		return sourceRef{}, false
	}
	linePart := payload[lineColon+1:]
	filePart := payload[:lineColon]
	if filePart == "" {
		return sourceRef{}, false
	}
	if _, err := strconv.Atoi(columnPart); err != nil {
		return sourceRef{}, false
	}
	lineNumber, err := strconv.Atoi(linePart)
	if err != nil {
		return sourceRef{}, false
	}
	return sourceRef{file: filePart, line: lineNumber}, true
}

func (c *sourceCache) lineText(ref sourceRef) (string, error) {
	lines, ok := c.files[ref.file]
	if !ok {
		src, err := os.ReadFile(ref.file)
		if err != nil {
			return "", fmt.Errorf("cannot read mapped source %s: %w", ref.file, err)
		}
		lines = splitSourceLines(string(src))
		c.files[ref.file] = lines
	}
	if ref.line <= 0 || ref.line > len(lines) {
		return "", nil
	}
	return lines[ref.line-1], nil
}

func splitSourceLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, "\n")
}

func hasMultipleSources(groups []group) bool {
	var first string
	for _, group := range groups {
		if group.source == nil {
			continue
		}
		if first == "" {
			first = group.source.file
			continue
		}
		if group.source.file != first {
			return true
		}
	}
	return false
}

func sourceLabel(ref sourceRef, multipleSources bool) string {
	line := strconv.Itoa(ref.line)
	if !multipleSources {
		return line
	}
	return filepath.Base(ref.file) + ":" + line
}

func paneWidths(width, leftLabelWidth, rightLabelWidth int) (int, int) {
	if width <= 0 {
		width = 120
	}
	fixed := leftLabelWidth + len(" | ") + len(" || ") + rightLabelWidth + len(" | ")
	available := width - fixed
	if available < 20 {
		available = 20
	}
	left := available / 2
	right := available - left
	if left < 10 {
		left = 10
	}
	if right < 10 {
		right = 10
	}
	return left, right
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if runeLen(s) <= width {
		return s
	}
	if width == 1 {
		return ">"
	}
	runes := []rune(s)
	return string(runes[:width-1]) + ">"
}

func padRight(s string, width int) string {
	padding := width - runeLen(s)
	if padding <= 0 {
		return s
	}
	return s + strings.Repeat(" ", padding)
}

func runeLen(s string) int {
	if utf8.ValidString(s) {
		return utf8.RuneCountInString(s)
	}
	return len(s)
}

package viewer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
	files     map[string]sourceFile
	highlight HighlightFunc
}

type sourceFile struct {
	lines []string
}

type HighlightFunc func(code, language string) ([]string, error)

type RenderOptions struct {
	Highlight HighlightFunc
}

// Build compiles entryPath and returns a side-by-side source/shell view.
// Source comments are enabled internally for mapping, but never appear in the
// rendered shell pane.
func Build(entryPath string, opts codegen.Options, width int) (string, error) {
	return BuildWithOptions(entryPath, opts, width, RenderOptions{})
}

func BuildWithOptions(entryPath string, opts codegen.Options, width int, renderOpts RenderOptions) (string, error) {
	opts.NoSourceMap = false
	compiled, err := codegen.CompileFile(entryPath, opts)
	if err != nil {
		return "", err
	}
	return RenderWithOptions(compiled, width, renderOpts)
}

func Render(compiled string, width int) (string, error) {
	return RenderWithOptions(compiled, width, RenderOptions{})
}

func RenderWithOptions(compiled string, width int, opts RenderOptions) (string, error) {
	groups, maxShellLine := parseGroups(compiled)
	cache := sourceCache{files: make(map[string]sourceFile), highlight: opts.Highlight}
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
	shellLines := highlightedShellLines(groups, maxShellLine, opts.Highlight)
	colorGutter := opts.Highlight != nil
	var out strings.Builder
	out.WriteString(paneBorder(leftLabelWidth, leftTextWidth, colorGutter))
	out.WriteString(paneDivider(colorGutter))
	out.WriteString(paneBorder(rightLabelWidth, rightTextWidth, colorGutter))
	out.WriteString("\n")
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
				rightText = shellLines[group.shell[i].number-1]
			}

			out.WriteString(paneLine(leftLabel, leftText, leftLabelWidth, leftTextWidth, colorGutter))
			out.WriteString(paneDivider(colorGutter))
			out.WriteString(paneLine(rightLabel, rightText, rightLabelWidth, rightTextWidth, colorGutter))
			out.WriteString("\n")
		}
	}
	return out.String(), nil
}

func NewBatHighlighter() (HighlightFunc, bool) {
	path, err := exec.LookPath("bat")
	if err != nil {
		return nil, false
	}
	return func(code, language string) ([]string, error) {
		cmd := exec.Command(path, "--language="+language, "--color=always", "--style=plain", "--paging=never", "--wrap=never")
		cmd.Stdin = strings.NewReader(code)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("bat highlight %s: %w: %s", language, err, strings.TrimSpace(stderr.String()))
		}
		return splitLines(string(out)), nil
	}, true
}

func highlightedShellLines(groups []group, maxShellLine int, highlight HighlightFunc) []string {
	lines := make([]string, maxShellLine)
	for _, group := range groups {
		for _, shellLine := range group.shell {
			lines[shellLine.number-1] = shellLine.text
		}
	}
	if highlight == nil || len(lines) == 0 {
		return lines
	}
	highlighted, err := highlight(strings.Join(lines, "\n")+"\n", "sh")
	if err != nil || len(highlighted) != len(lines) {
		return lines
	}
	return highlighted
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
	file, ok := c.files[ref.file]
	if !ok {
		src, err := os.ReadFile(ref.file)
		if err != nil {
			return "", fmt.Errorf("cannot read mapped source %s: %w", ref.file, err)
		}
		lines := splitSourceLines(string(src))
		if c.highlight != nil {
			if highlighted, err := c.highlight(string(src), "TypeScript"); err == nil && len(highlighted) == len(lines) {
				lines = highlighted
			}
		}
		file = sourceFile{lines: lines}
		c.files[ref.file] = file
	}
	if ref.line <= 0 || ref.line > len(file.lines) {
		return "", nil
	}
	return file.lines[ref.line-1], nil
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
	fixed := paneFixedWidth(leftLabelWidth) + visibleLen(paneDivider(false)) + paneFixedWidth(rightLabelWidth)
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

func paneLine(label, text string, labelWidth, textWidth int, colorGutter bool) string {
	gutter := fmt.Sprintf("%*s   │ ", labelWidth, label)
	gutter = dim(gutter, colorGutter)
	return gutter + padRight(truncate(text, textWidth), textWidth)
}

func paneBorder(labelWidth, textWidth int, colorGutter bool) string {
	return dim(strings.Repeat("─", labelWidth+3)+"┬"+strings.Repeat("─", textWidth+1), colorGutter)
}

func paneDivider(colorGutter bool) string {
	return dim(" ║ ", colorGutter)
}

func paneFixedWidth(labelWidth int) int {
	return labelWidth + 5
}

func dim(s string, enabled bool) string {
	if !enabled || s == "" {
		return s
	}
	return "\x1b[38;5;246m" + s + "\x1b[0m"
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if visibleLen(s) <= width {
		return s
	}
	if width == 1 {
		return ">"
	}
	limit := width - 1
	visible := 0
	sawANSI := false
	var out strings.Builder
	for i := 0; i < len(s); {
		if end, ok := ansiSequenceEnd(s, i); ok {
			out.WriteString(s[i:end])
			sawANSI = true
			i = end
			continue
		}
		if visible >= limit {
			break
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		out.WriteRune(r)
		visible++
		i += size
	}
	if sawANSI {
		out.WriteString("\x1b[0m")
	}
	out.WriteString(">")
	return out.String()
}

func padRight(s string, width int) string {
	padding := width - visibleLen(s)
	if padding <= 0 {
		return s
	}
	return s + strings.Repeat(" ", padding)
}

func runeLen(s string) int {
	return visibleLen(s)
}

func visibleLen(s string) int {
	if utf8.ValidString(s) {
		count := 0
		for i := 0; i < len(s); {
			if end, ok := ansiSequenceEnd(s, i); ok {
				i = end
				continue
			}
			_, size := utf8.DecodeRuneInString(s[i:])
			count++
			i += size
		}
		return count
	}
	return len(s)
}

func stripANSI(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if end, ok := ansiSequenceEnd(s, i); ok {
			i = end
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		out.WriteRune(r)
		i += size
	}
	return out.String()
}

func ansiSequenceEnd(s string, start int) (int, bool) {
	if start+2 >= len(s) || s[start] != '\x1b' || s[start+1] != '[' {
		return 0, false
	}
	for i := start + 2; i < len(s); i++ {
		if s[i] >= '@' && s[i] <= '~' {
			return i + 1, true
		}
	}
	return 0, false
}

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
	for idx, group := range groups {
		nextSource := nextGroupSource(groups, idx)
		sourceOnly, overlay := gapSourceLines(group.source, nextSource, len(group.shell))
		rowCount := max(1, len(group.shell))
		for i := 0; i < rowCount; i++ {
			if i == 1 {
				for _, ref := range sourceOnly {
					if err := writePaneRow(&out, &cache, &ref, "", "", multipleSources, leftLabelWidth, leftTextWidth, rightLabelWidth, rightTextWidth, colorGutter); err != nil {
						return "", err
					}
				}
			}

			rightLabel := ""
			rightText := ""
			if i < len(group.shell) {
				rightLabel = strconv.Itoa(group.shell[i].number)
				rightText = shellLines[group.shell[i].number-1]
			}

			rowSource := sourceForGroupRow(group.source, overlay, i)
			if err := writePaneRow(&out, &cache, rowSource, rightLabel, rightText, multipleSources, leftLabelWidth, leftTextWidth, rightLabelWidth, rightTextWidth, colorGutter); err != nil {
				return "", err
			}
		}
		if rowCount == 1 {
			for _, ref := range sourceOnly {
				if err := writePaneRow(&out, &cache, &ref, "", "", multipleSources, leftLabelWidth, leftTextWidth, rightLabelWidth, rightTextWidth, colorGutter); err != nil {
					return "", err
				}
			}
		}
	}
	return out.String(), nil
}

func writePaneRow(out *strings.Builder, cache *sourceCache, ref *sourceRef, rightLabel, rightText string, multipleSources bool, leftLabelWidth, leftTextWidth, rightLabelWidth, rightTextWidth int, colorGutter bool) error {
	leftLabel := ""
	leftText := ""
	if ref != nil {
		leftLabel = sourceLabel(*ref, multipleSources)
		text, err := cache.lineText(*ref)
		if err != nil {
			return err
		}
		leftText = text
	}
	for _, row := range paneRows(leftLabel, leftText, rightLabel, rightText, leftLabelWidth, leftTextWidth, rightLabelWidth, rightTextWidth, colorGutter) {
		out.WriteString(row)
		out.WriteString("\n")
	}
	return nil
}

func sourceForGroupRow(current *sourceRef, overlay map[int]sourceRef, row int) *sourceRef {
	if row == 0 {
		return current
	}
	if ref, ok := overlay[row]; ok {
		return &ref
	}
	return nil
}

func nextGroupSource(groups []group, idx int) *sourceRef {
	for i := idx + 1; i < len(groups); i++ {
		if groups[i].source != nil {
			return groups[i].source
		}
	}
	return nil
}

func gapSourceLines(current, next *sourceRef, shellLineCount int) ([]sourceRef, map[int]sourceRef) {
	overlay := make(map[int]sourceRef)
	if current == nil || next == nil || current.file != next.file || next.line <= current.line+1 {
		return nil, overlay
	}
	var refs []sourceRef
	for line := current.line + 1; line < next.line; line++ {
		refs = append(refs, sourceRef{file: current.file, line: line})
	}
	extraShellRows := max(0, shellLineCount-1)
	overlayCount := min(extraShellRows, len(refs))
	sourceOnlyCount := len(refs) - overlayCount
	for i, ref := range refs[sourceOnlyCount:] {
		row := 1 + extraShellRows - overlayCount + i
		overlay[row] = ref
	}
	return refs[:sourceOnlyCount], overlay
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

func paneRows(leftLabel, leftText, rightLabel, rightText string, leftLabelWidth, leftTextWidth, rightLabelWidth, rightTextWidth int, colorGutter bool) []string {
	leftRows := paneWrappedLines(leftLabel, leftText, leftLabelWidth, leftTextWidth, colorGutter)
	rightRows := paneWrappedLines(rightLabel, rightText, rightLabelWidth, rightTextWidth, colorGutter)
	rowCount := max(len(leftRows), len(rightRows))
	rows := make([]string, 0, rowCount)
	for i := 0; i < rowCount; i++ {
		left := blankPaneLine(leftLabelWidth, leftTextWidth, colorGutter)
		if i < len(leftRows) {
			left = leftRows[i]
		}
		right := blankPaneLine(rightLabelWidth, rightTextWidth, colorGutter)
		if i < len(rightRows) {
			right = rightRows[i]
		}
		rows = append(rows, left+paneDivider(colorGutter)+right)
	}
	return rows
}

func paneWrappedLines(label, text string, labelWidth, textWidth int, colorGutter bool) []string {
	parts := wrapVisible(text, textWidth)
	rows := make([]string, 0, len(parts))
	for i, part := range parts {
		rowLabel := label
		if i > 0 {
			rowLabel = "↳"
		}
		rows = append(rows, paneSingleLine(rowLabel, part, labelWidth, textWidth, colorGutter))
	}
	return rows
}

func blankPaneLine(labelWidth, textWidth int, colorGutter bool) string {
	return paneSingleLine("", "", labelWidth, textWidth, colorGutter)
}

func paneSingleLine(label, text string, labelWidth, textWidth int, colorGutter bool) string {
	gutter := fmt.Sprintf("%*s   │ ", labelWidth, label)
	gutter = dim(gutter, colorGutter)
	return gutter + padRight(text, textWidth)
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

func wrapVisible(s string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	if visibleLen(s) <= width {
		return []string{s}
	}

	var rows []string
	var current strings.Builder
	activeANSI := ""
	segmentHasANSI := false
	visible := 0

	flush := func() {
		if segmentHasANSI {
			current.WriteString("\x1b[0m")
		}
		rows = append(rows, current.String())
		current.Reset()
		visible = 0
		segmentHasANSI = false
		if activeANSI != "" {
			current.WriteString(activeANSI)
			segmentHasANSI = true
		}
	}

	for i := 0; i < len(s); {
		if end, ok := ansiSequenceEnd(s, i); ok {
			seq := s[i:end]
			current.WriteString(seq)
			segmentHasANSI = true
			activeANSI = updateActiveANSI(activeANSI, seq)
			i = end
			continue
		}
		if visible >= width {
			flush()
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		current.WriteRune(r)
		visible++
		i += size
	}
	if segmentHasANSI {
		current.WriteString("\x1b[0m")
	}
	rows = append(rows, current.String())
	return rows
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

func updateActiveANSI(active, seq string) string {
	if !strings.HasSuffix(seq, "m") {
		return active
	}
	body := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	if body == "" || body == "0" {
		return ""
	}
	for _, part := range strings.Split(body, ";") {
		if part == "0" {
			return ""
		}
	}
	return seq
}

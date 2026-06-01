package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/victor141516/besht/internal/codegen"
	"github.com/victor141516/besht/internal/stdlib"
)

func writeCmdTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestUsageDescribesNoSourceMapAsSourceComments(t *testing.T) {
	if !strings.Contains(usage, "Omit # besht:file:line:col source comments") {
		t.Fatalf("usage should describe --opt-no-source-map as source comments, got:\n%s", usage)
	}
	if strings.Contains(usage, "sourcemap") {
		t.Fatalf("usage should not describe inline comments as sourcemap, got:\n%s", usage)
	}
}

func TestCheckFileUsesModuleShellImportValidation(t *testing.T) {
	dir := t.TempDir()
	writeCmdTestFile(t, dir, "legacy.sh", `legacy() { printf '%s' "$1"; }
`)
	valid := writeCmdTestFile(t, dir, "valid.bsh", `import { legacy } from "./legacy.sh" assert { type: "shell" }
legacy("ok")
`)
	if err := checkFile(valid, codegen.Options{}); err != nil {
		t.Fatalf("checkFile valid shell import: %v", err)
	}

	invalid := writeCmdTestFile(t, dir, "invalid.bsh", `import { legacy } from "./legacy.sh"
`)
	if err := checkFile(invalid, codegen.Options{}); err == nil || !strings.Contains(err.Error(), `.sh imports require assert { type: "shell" }`) {
		t.Fatalf("checkFile invalid shell import: got %v", err)
	}
}

func TestRunInitWritesStdlibDeclarations(t *testing.T) {
	dir := t.TempDir()
	var stderr bytes.Buffer

	if err := runInit(nil, dir, &stderr); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if stderr.String() != "wrote stdlib.d.bsh\n" {
		t.Fatalf("stderr: got %q", stderr.String())
	}
	path := filepath.Join(dir, "stdlib.d.bsh")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stdlib: %v", err)
	}
	if string(content) != stdlib.Declarations {
		t.Fatalf("content mismatch")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat stdlib: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("mode: got %o, want 0644", info.Mode().Perm())
	}
}

func TestRunInitIdenticalRerunLeavesFileUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdlib.d.bsh")
	if err := os.WriteFile(path, []byte(stdlib.Declarations), 0600); err != nil {
		t.Fatalf("seed stdlib: %v", err)
	}
	var stderr bytes.Buffer

	if err := runInit(nil, dir, &stderr); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if stderr.String() != "stdlib.d.bsh already up to date\n" {
		t.Fatalf("stderr: got %q", stderr.String())
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat stdlib: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode changed: got %o", info.Mode().Perm())
	}
}

func TestRunInitRefusesDifferentExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdlib.d.bsh")
	if err := os.WriteFile(path, []byte("custom\n"), 0644); err != nil {
		t.Fatalf("seed stdlib: %v", err)
	}

	err := runInit(nil, dir, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "stdlib.d.bsh already exists; pass --force to overwrite") {
		t.Fatalf("runInit error: got %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stdlib: %v", err)
	}
	if string(content) != "custom\n" {
		t.Fatalf("content changed: got %q", content)
	}
}

func TestRunInitForceOverwritesDifferentExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdlib.d.bsh")
	if err := os.WriteFile(path, []byte("custom\n"), 0644); err != nil {
		t.Fatalf("seed stdlib: %v", err)
	}
	var stderr bytes.Buffer

	if err := runInit([]string{"--force"}, dir, &stderr); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if stderr.String() != "wrote stdlib.d.bsh\n" {
		t.Fatalf("stderr: got %q", stderr.String())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stdlib: %v", err)
	}
	if string(content) != stdlib.Declarations {
		t.Fatalf("content mismatch")
	}
}

func TestRunInitRejectsUnsupportedArgs(t *testing.T) {
	err := runInit([]string{"--dry-run"}, t.TempDir(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unsupported init argument: --dry-run") {
		t.Fatalf("runInit error: got %v", err)
	}
}

func TestUsageDocumentsInit(t *testing.T) {
	for _, want := range []string{"besht init", "besht init --force"} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %q", want)
		}
	}
}

func TestUsageDocumentsCompileAndVisualizeModes(t *testing.T) {
	for _, want := range []string{
		"besht compile <file.bsh>",
		"besht compile --check <file.bsh>",
		"besht visualize <file.bsh>",
		"Alias for besht compile",
		"Sizes the side-by-side view to the current terminal width",
		"shows file-name headers",
		"disables horizontal scrolling",
		"wraps long lines with continuation markers",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %q", want)
		}
	}
}

func TestParseSttyWidth(t *testing.T) {
	width, ok := parseSttyWidth("48 192\n")
	if !ok || width != 192 {
		t.Fatalf("parseSttyWidth: got width=%d ok=%v, want 192 true", width, ok)
	}
	for _, bad := range []string{"", "48", "x 120", "48 0", "48 -1"} {
		if width, ok := parseSttyWidth(bad); ok {
			t.Fatalf("parseSttyWidth(%q): got width=%d ok=true, want false", bad, width)
		}
	}
}

func TestEnvColumnsWidth(t *testing.T) {
	t.Setenv("COLUMNS", "180")
	width, ok := envColumnsWidth()
	if !ok || width != 180 {
		t.Fatalf("envColumnsWidth: got width=%d ok=%v, want 180 true", width, ok)
	}
	t.Setenv("COLUMNS", "not-a-number")
	if width, ok := envColumnsWidth(); ok {
		t.Fatalf("envColumnsWidth invalid: got width=%d ok=true, want false", width)
	}
}

func TestLessPagerCommandDisablesHorizontalScrolling(t *testing.T) {
	got := lessPagerCommand([]string{"less", "-RS", "--chop-long-lines", "-F"})
	joined := strings.Join(got, " ")
	if strings.Contains(joined, " -S") || strings.Contains(joined, "--chop-long-lines") || strings.Contains(joined, "-RS") {
		t.Fatalf("less pager should remove chop-long-lines options, got %q", joined)
	}
	if !containsArg(got, "-R") || !containsArg(got, "-+S") {
		t.Fatalf("less pager should preserve ANSI and explicitly disable chopping, got %#v", got)
	}
}

func TestPagerCommandSanitizesLessFromPagerEnv(t *testing.T) {
	t.Setenv("PAGER", "less -S")
	got := pagerCommand()
	if strings.Join(got, " ") != "less -R -+S" {
		t.Fatalf("pagerCommand: got %#v, want less -R -+S", got)
	}
}

func TestConfigurePagerForVisualizationDisablesLessHorizontalKeys(t *testing.T) {
	less, err := exec.LookPath("less")
	if err != nil {
		t.Skip("less is not installed")
	}
	if !lessSupportsLesskeySource(less) {
		t.Skip("less does not support --lesskey-src")
	}

	pager := []string{less, "-R", "-+S"}
	cleanup, err := configurePagerForVisualization(&pager)
	if err != nil {
		t.Fatalf("configurePagerForVisualization: %v", err)
	}
	defer cleanup()

	var lesskeyPath string
	for _, arg := range pager {
		if strings.HasPrefix(arg, "--lesskey-src=") {
			lesskeyPath = strings.TrimPrefix(arg, "--lesskey-src=")
			break
		}
	}
	if lesskeyPath == "" {
		t.Fatalf("pager missing --lesskey-src: %#v", pager)
	}
	content, err := os.ReadFile(lesskeyPath)
	if err != nil {
		t.Fatalf("read lesskey source: %v", err)
	}
	for _, want := range []string{`\kr noaction`, `\kl noaction`, `\e} noaction`, `\e{ noaction`} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("lesskey source missing %q:\n%s", want, content)
		}
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

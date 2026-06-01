package main

import (
	"bytes"
	"os"
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
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %q", want)
		}
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/victor141516/besht/internal/codegen"
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

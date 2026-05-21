package codegen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/victor141516/besht/internal/codegen"
)

func compileSplit(t *testing.T, files map[string]string, entry string) map[string]string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writeFile(t, dir, name, content)
	}
	outDir := filepath.Join(dir, "out")
	if err := codegen.CompileFileSplit(filepath.Join(dir, entry), outDir, codegen.Options{}); err != nil {
		t.Fatalf("CompileFileSplit error: %v", err)
	}
	result := make(map[string]string)
	err := filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(outDir, path)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[rel] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("walk output dir: %v", err)
	}
	return result
}

func TestSplit_SingleFileNoImports(t *testing.T) {
	files := compileSplit(t, map[string]string{
		"main.bsh": `let name: string = "world"
console.log("Hello, " + name)
`,
	}, "main.bsh")

	if _, ok := files["main.sh"]; !ok {
		t.Fatalf("main.sh not found; got: %v", files)
	}
	main := files["main.sh"]

	if !strings.HasPrefix(main, "#!/bin/sh") {
		t.Error("entry file must start with #!/bin/sh")
	}
	assertContains(t, main, `_BESHT_ROOT="$(cd "$(dirname "$0")" && pwd)"`)
	assertContains(t, main, `name='world'`)
	assertNotContains(t, main, `. "$_BESHT_ROOT`)
	assertNotContains(t, main, `_BESHT_LOADED_`)
}

func TestSplit_LibraryHasIncludeGuard(t *testing.T) {
	files := compileSplit(t, map[string]string{
		"lib/log.bsh": `export function info(msg: string) {
    $("printf", "[INFO] %s\\n", msg).run()
}
`,
		"main.bsh": `import { info } from "./lib/log"
info("started")
`,
	}, "main.bsh")

	if _, ok := files["lib/log.sh"]; !ok {
		t.Fatalf("lib/log.sh not found; got: %v", files)
	}
	lib := files["lib/log.sh"]

	assertNotContains(t, lib, "#!/bin/sh")
	assertContains(t, lib, `[ -n "$_BESHT_LOADED_lib__log" ] && return 0`)
	assertContains(t, lib, `_BESHT_LOADED_lib__log=1`)
	assertContains(t, lib, `lib__log__info()`)
}

func TestSplit_EntrySourcesLibraries(t *testing.T) {
	files := compileSplit(t, map[string]string{
		"lib/log.bsh": `export function info(msg: string) {
    $("printf", "[INFO] %s\\n", msg).run()
}
`,
		"main.bsh": `import { info } from "./lib/log"
info("started")
`,
	}, "main.bsh")

	main := files["main.sh"]
	assertContains(t, main, `_BESHT_ROOT="$(cd "$(dirname "$0")" && pwd)"`)
	assertContains(t, main, sourceFromRootForTest("lib/log.sh"))
}

func TestSplit_LibrarySourcesTransitiveDep(t *testing.T) {
	files := compileSplit(t, map[string]string{
		"lib/util.bsh": `export function noop() {
    $("true").run()
}
`,
		"lib/log.bsh": `import { noop } from "./util"
export function info(msg: string) {
    noop()
    $("printf", "[INFO] %s\\n", msg).run()
}
`,
		"main.bsh": `import { info } from "./lib/log"
info("hi")
`,
	}, "main.bsh")

	if _, ok := files["lib/util.sh"]; !ok {
		t.Fatalf("lib/util.sh not created; got: %v", files)
	}
	logSh := files["lib/log.sh"]
	assertContains(t, logSh, sourceFromRootForTest("lib/util.sh"))
	assertContains(t, logSh, `[ -n "$_BESHT_LOADED_lib__log" ] && return 0`)

	utilSh := files["lib/util.sh"]
	assertContains(t, utilSh, `[ -n "$_BESHT_LOADED_lib__util" ] && return 0`)
	assertNotContains(t, utilSh, `. "$_BESHT_ROOT`)
}

func TestSplit_DiamondImportBothFilesCreated(t *testing.T) {
	files := compileSplit(t, map[string]string{
		"lib/base.bsh": `export function base() {
    $("true").run()
}
`,
		"lib/a.bsh": `import { base } from "./base"
export function aFunc() {
    base()
}
`,
		"lib/b.bsh": `import { base } from "./base"
export function bFunc() {
    base()
}
`,
		"main.bsh": `import { aFunc } from "./lib/a"
import { bFunc } from "./lib/b"
aFunc()
bFunc()
`,
	}, "main.bsh")

	for _, name := range []string{"lib/base.sh", "lib/a.sh", "lib/b.sh", "main.sh"} {
		if _, ok := files[name]; !ok {
			t.Errorf("expected %s to be created", name)
		}
	}

	main := files["main.sh"]
	assertContains(t, main, sourceFromRootForTest("lib/a.sh"))
	assertContains(t, main, sourceFromRootForTest("lib/b.sh"))

	baseSh := files["lib/base.sh"]
	assertContains(t, baseSh, `[ -n "$_BESHT_LOADED_lib__base" ] && return 0`)
}

func TestSplit_FunctionNamesQualified(t *testing.T) {
	files := compileSplit(t, map[string]string{
		"lib/log.bsh": `export function info(msg: string) {
    $("printf", "[INFO] %s\\n", msg).run()
}
export function error(msg: string) {
    $("printf", "[ERROR] %s\\n", msg).run()
}
`,
		"main.bsh": `import { info, error } from "./lib/log"
info("started")
error("oops")
`,
	}, "main.bsh")

	lib := files["lib/log.sh"]
	assertContains(t, lib, `lib__log__info()`)
	assertContains(t, lib, `lib__log__error()`)

	main := files["main.sh"]
	assertContains(t, main, `lib__log__info 'started'`)
	assertContains(t, main, `lib__log__error 'oops'`)
}

func TestSplit_OutputDirectoryStructureMirrorsSource(t *testing.T) {
	files := compileSplit(t, map[string]string{
		"a/b/c/deep.bsh": `export function deep() {
    $("true").run()
}
`,
		"main.bsh": `import { deep } from "./a/b/c/deep"
deep()
`,
	}, "main.bsh")

	if _, ok := files["a/b/c/deep.sh"]; !ok {
		t.Errorf("expected a/b/c/deep.sh; got: %v", files)
	}
	main := files["main.sh"]
	assertContains(t, main, sourceFromRootForTest("a/b/c/deep.sh"))
}

func TestSplit_RejectsImportOutsideCompilerRoot(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "app")
	writeFile(t, root, "main.bsh", `import { outside } from "../outside"
outside()
`)
	writeFile(t, dir, "outside.bsh", `export function outside() {
    $("true").run()
}
`)
	outDir := filepath.Join(root, "out")
	err := codegen.CompileFileSplit(filepath.Join(root, "main.bsh"), outDir, codegen.Options{})
	if err == nil {
		t.Fatal("expected outside-root import error, got nil")
	}
	if !strings.Contains(err.Error(), "outside compiler root") {
		t.Fatalf("error: got %v, want outside compiler root", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "outside.sh")); !os.IsNotExist(statErr) {
		t.Fatalf("outside-root split output was written or stat failed: %v", statErr)
	}
}

func TestSplit_LibraryNotExecutable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/log.bsh", `export function info(msg: string) {
    $("printf", "INFO: %s\n", msg).run()
}
`)
	writeFile(t, dir, "main.bsh", `import { info } from "./lib/log"
info("hi")
`)
	outDir := filepath.Join(dir, "out")
	if err := codegen.CompileFileSplit(filepath.Join(dir, "main.bsh"), outDir, codegen.Options{}); err != nil {
		t.Fatalf("error: %v", err)
	}

	libPath := filepath.Join(outDir, "lib", "log.sh")
	content, _ := os.ReadFile(libPath)
	lib := string(content)

	if strings.HasPrefix(lib, "#!/") {
		t.Error("library file should not have a shebang")
	}
	assertContains(t, lib, "return 0")
}

func TestSplit_MissingOutputDirCreated(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.bsh", `console.log("hello")
`)
	outDir := filepath.Join(dir, "nested", "deep", "out")
	if err := codegen.CompileFileSplit(filepath.Join(dir, "main.bsh"), outDir, codegen.Options{}); err != nil {
		t.Fatalf("error creating nested output dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "main.sh")); err != nil {
		t.Errorf("main.sh not found in nested output dir: %v", err)
	}
}

func TestSplit_SplitRequiresOutputDir(t *testing.T) {
	t.Skip("CLI flag validation tested separately")
}

func TestSplit_MultipleImportsSameLib(t *testing.T) {
	files := compileSplit(t, map[string]string{
		"lib/log.bsh": `export function info(msg: string) {
    $("printf", "[INFO] %s\\n", msg).run()
}
`,
		"lib/check.bsh": `import { info } from "./log"
export function check(name: string): string {
    info("Checking " + name)
    return "pass"
}
`,
		"main.bsh": `import { info } from "./lib/log"
import { check } from "./lib/check"
let r: string = check("thing")
info(r)
`,
	}, "main.bsh")

	main := files["main.sh"]
	logCount := strings.Count(main, sourceFromRootForTest("lib/log.sh"))
	if logCount != 1 {
		t.Errorf("lib/log.sh sourced %d times in main.sh, want 1 (include guard handles dedup)", logCount)
	}
}

func TestSplit_TSFallbackOutputPathUsesShellExtension(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/dep.ts", `export const cmd = ["echo", "ts"]`)
	writeFile(t, dir, "main.bsh", `import { cmd } from "./lib/dep"
$(...cmd).run()
`)
	outDir := filepath.Join(dir, "out")
	if err := codegen.CompileFileSplit(filepath.Join(dir, "main.bsh"), outDir, codegen.Options{ResolveTsImports: true}); err != nil {
		t.Fatalf("CompileFileSplit with .ts fallback: %v", err)
	}
	dep, err := os.ReadFile(filepath.Join(outDir, "lib", "dep.sh"))
	if err != nil {
		t.Fatalf("expected lib/dep.sh for .ts module: %v", err)
	}
	main, err := os.ReadFile(filepath.Join(outDir, "main.sh"))
	if err != nil {
		t.Fatalf("read main.sh: %v", err)
	}
	assertContains(t, string(dep), `lib__dep__cmd=$(`)
	assertContains(t, string(main), sourceFromRootForTest("lib/dep.sh"))
}

func TestSplit_ShellImportCopiedGuardedAndRuns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/legacy.sh", `_BESHT_LEGACY_SOURCE_COUNT=$(( ${_BESHT_LEGACY_SOURCE_COUNT:-0} + 1 ))
legacy() {
    printf 'legacy:%s:%s' "$_BESHT_LEGACY_SOURCE_COUNT" "$1"
}
`)
	writeFile(t, dir, "lib/wrapper.bsh", `import { legacy } from "./legacy.sh" assert { type: "shell" }
export function wrapped(): string {
    return legacy("wrapped")
}
`)
	writeFile(t, dir, "main.bsh", `import { legacy } from "./lib/legacy.sh" assert { type: "shell" }
import { wrapped } from "./lib/wrapper"
console.log(wrapped())
console.log(legacy("direct"))
`)
	outDir := filepath.Join(dir, "out")
	if err := codegen.CompileFileSplit(filepath.Join(dir, "main.bsh"), outDir, codegen.Options{Strict: true}); err != nil {
		t.Fatalf("CompileFileSplit with shell import: %v", err)
	}

	legacyOut, err := os.ReadFile(filepath.Join(outDir, "lib", "legacy.sh"))
	if err != nil {
		t.Fatalf("raw shell import was not copied: %v", err)
	}
	assertContains(t, string(legacyOut), `legacy()`)

	mainOut, err := os.ReadFile(filepath.Join(outDir, "main.sh"))
	if err != nil {
		t.Fatalf("read main.sh: %v", err)
	}
	wrapperOut, err := os.ReadFile(filepath.Join(outDir, "lib", "wrapper.sh"))
	if err != nil {
		t.Fatalf("read wrapper.sh: %v", err)
	}
	assertContains(t, string(mainOut), `_BESHT_SHELL_LOADED_lib_legacy_sh_`)
	assertContains(t, string(mainOut), sourceFromRootForTest("lib/legacy.sh"))
	assertContains(t, string(wrapperOut), `_BESHT_SHELL_LOADED_lib_legacy_sh_`)
	assertContains(t, string(wrapperOut), sourceFromRootForTest("lib/legacy.sh"))

	cmd := exec.Command("sh", filepath.Join(outDir, "main.sh"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run split shell: %v\n%s\n--- main.sh ---\n%s", err, output, mainOut)
	}
	if string(output) != "legacy:1:wrapped\nlegacy:1:direct\n" {
		t.Fatalf("output: got %q", output)
	}
}

func TestSplit_ExternalShellImportRequiresOptInAndSourcesOriginal(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	legacyPath := writeFile(t, dir, "lib/legacy.sh", `legacy() { printf 'legacy:%s' "$1"; }
`)
	writeFile(t, appDir, "main.bsh", `import { legacy } from "../lib/legacy.sh" assert { type: "shell" }
console.log(legacy("ok"))
`)
	outDir := filepath.Join(dir, "out")

	err := codegen.CompileFileSplit(filepath.Join(appDir, "main.bsh"), outDir, codegen.Options{})
	if err == nil || !strings.Contains(err.Error(), "outside compiler root") {
		t.Fatalf("CompileFileSplit default error: got %v, want outside compiler root", err)
	}

	if err := codegen.CompileFileSplit(filepath.Join(appDir, "main.bsh"), outDir, codegen.Options{AllowExternalShellImports: true}); err != nil {
		t.Fatalf("CompileFileSplit with external shell import opt-in: %v", err)
	}
	mainOut, err := os.ReadFile(filepath.Join(outDir, "main.sh"))
	if err != nil {
		t.Fatalf("read main.sh: %v", err)
	}
	assertContains(t, string(mainOut), `. `+shellSingleQuoteForTest(legacyPath))
	if _, err := os.Stat(filepath.Join(outDir, "lib", "legacy.sh")); !os.IsNotExist(err) {
		t.Fatalf("external shell import should not be copied outside output root: %v", err)
	}
}

func TestSplit_ShellImportGuardCollisionDoesNotSkipSources(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/a-b.sh", `_BESHT_DASH_SOURCE_COUNT=$(( ${_BESHT_DASH_SOURCE_COUNT:-0} + 1 ))
dashFunc() {
    printf 'dash:%s' "$_BESHT_DASH_SOURCE_COUNT"
}
`)
	writeFile(t, dir, "lib/a_b.sh", `_BESHT_UNDER_SOURCE_COUNT=$(( ${_BESHT_UNDER_SOURCE_COUNT:-0} + 1 ))
underFunc() {
    printf 'under:%s' "$_BESHT_UNDER_SOURCE_COUNT"
}
`)
	writeFile(t, dir, "main.bsh", `import { dashFunc } from "./lib/a-b.sh" assert { type: "shell" }
import { underFunc } from "./lib/a_b.sh" assert { type: "shell" }
console.log(dashFunc())
console.log(underFunc())
`)
	outDir := filepath.Join(dir, "out")
	if err := codegen.CompileFileSplit(filepath.Join(dir, "main.bsh"), outDir, codegen.Options{Strict: true}); err != nil {
		t.Fatalf("CompileFileSplit with colliding shell imports: %v", err)
	}
	mainOut, err := os.ReadFile(filepath.Join(outDir, "main.sh"))
	if err != nil {
		t.Fatalf("read main.sh: %v", err)
	}
	cmd := exec.Command("sh", filepath.Join(outDir, "main.sh"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run split shell collision fixture: %v\n%s\n--- main.sh ---\n%s", err, output, mainOut)
	}
	if string(output) != "dash:1\nunder:1\n" {
		t.Fatalf("output: got %q", output)
	}
}

func TestSplit_ShellImportSelfCopyDoesNotTruncateSource(t *testing.T) {
	dir := t.TempDir()
	legacy := `legacy() {
    printf 'legacy:%s' "$1"
}
`
	writeFile(t, dir, "lib/legacy.sh", legacy)
	writeFile(t, dir, "main.bsh", `import { legacy } from "./lib/legacy.sh" assert { type: "shell" }
console.log(legacy("ok"))
`)
	if err := codegen.CompileFileSplit(filepath.Join(dir, "main.bsh"), dir, codegen.Options{Strict: true}); err != nil {
		t.Fatalf("CompileFileSplit into source root: %v", err)
	}
	legacyOut, err := os.ReadFile(filepath.Join(dir, "lib", "legacy.sh"))
	if err != nil {
		t.Fatalf("read legacy.sh after split self-copy: %v", err)
	}
	if string(legacyOut) != legacy {
		t.Fatalf("legacy.sh changed after split self-copy: got %q, want %q", legacyOut, legacy)
	}
	cmd := exec.Command("sh", filepath.Join(dir, "main.sh"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run split self-copy fixture: %v\n%s", err, output)
	}
	if string(output) != "legacy:ok\n" {
		t.Fatalf("output: got %q", output)
	}
}

func TestSplit_ShellImportRejectsOutputSymlink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/legacy.sh", `legacy() { printf 'legacy'; }
`)
	writeFile(t, dir, "main.bsh", `import { legacy } from "./lib/legacy.sh" assert { type: "shell" }
console.log(legacy())
`)
	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(filepath.Join(outDir, "lib"), 0755); err != nil {
		t.Fatalf("mkdir output lib: %v", err)
	}
	target := filepath.Join(dir, "target.sh")
	if err := os.WriteFile(target, []byte("keep me"), 0644); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(outDir, "lib", "legacy.sh")); err != nil {
		t.Fatalf("create output symlink: %v", err)
	}
	err := codegen.CompileFileSplit(filepath.Join(dir, "main.bsh"), outDir, codegen.Options{Strict: true})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite symlink") {
		t.Fatalf("CompileFileSplit error: got %v, want refusing to overwrite symlink", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read symlink target: %v", err)
	}
	if string(content) != "keep me" {
		t.Fatalf("symlink target changed: got %q", content)
	}
}

func TestSplit_ShellImportRejectsSymlinkedOutputDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/legacy.sh", `legacy() { printf 'legacy'; }
`)
	writeFile(t, dir, "main.bsh", `import { legacy } from "./lib/legacy.sh" assert { type: "shell" }
console.log(legacy())
`)
	outDir := filepath.Join(dir, "out")
	escaped := filepath.Join(dir, "escaped")
	if err := os.MkdirAll(escaped, 0755); err != nil {
		t.Fatalf("mkdir escaped dir: %v", err)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}
	if err := os.Symlink(escaped, filepath.Join(outDir, "lib")); err != nil {
		t.Fatalf("create output dir symlink: %v", err)
	}
	err := codegen.CompileFileSplit(filepath.Join(dir, "main.bsh"), outDir, codegen.Options{Strict: true})
	if err == nil || !strings.Contains(err.Error(), "refusing to write through symlinked output dir") {
		t.Fatalf("CompileFileSplit error: got %v, want symlinked output dir rejection", err)
	}
	if _, err := os.Stat(filepath.Join(escaped, "legacy.sh")); !os.IsNotExist(err) {
		t.Fatalf("escaped output was written: %v", err)
	}
}

func TestSplit_ShellImportRejectsNestedSymlinkedOutputDirWithoutSideEffect(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/sub/legacy.sh", `legacy() { printf 'legacy'; }
`)
	writeFile(t, dir, "main.bsh", `import { legacy } from "./lib/sub/legacy.sh" assert { type: "shell" }
console.log(legacy())
`)
	outDir := filepath.Join(dir, "out")
	escaped := filepath.Join(dir, "escaped")
	if err := os.MkdirAll(escaped, 0755); err != nil {
		t.Fatalf("mkdir escaped dir: %v", err)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}
	if err := os.Symlink(escaped, filepath.Join(outDir, "lib")); err != nil {
		t.Fatalf("create output dir symlink: %v", err)
	}
	err := codegen.CompileFileSplit(filepath.Join(dir, "main.bsh"), outDir, codegen.Options{Strict: true})
	if err == nil {
		t.Fatalf("expected symlinked output dir rejection")
	}
	if _, err := os.Stat(filepath.Join(escaped, "sub")); !os.IsNotExist(err) {
		t.Fatalf("escaped subdir was created: %v", err)
	}
}

func TestSplit_ShellImportSpecialPathSourceIsSafe(t *testing.T) {
	dir := t.TempDir()
	shellRel := "lib/legacy $(touch PWNED) 'quote.sh"
	writeFile(t, dir, shellRel, `legacy() {
    printf 'split-safe:%s\n' "$1"
}
`)
	writeFile(t, dir, "main.bsh", `import { legacy } from "./`+filepath.ToSlash(shellRel)+`" assert { type: "shell" }
console.log(legacy("ok"))
`)
	outDir := filepath.Join(dir, "out")
	if err := codegen.CompileFileSplit(filepath.Join(dir, "main.bsh"), outDir, codegen.Options{Strict: true}); err != nil {
		t.Fatalf("CompileFileSplit with special shell import path: %v", err)
	}
	mainOut, err := os.ReadFile(filepath.Join(outDir, "main.sh"))
	if err != nil {
		t.Fatalf("read main.sh: %v", err)
	}
	assertContains(t, string(mainOut), sourceFromRootForTest(shellRel))
	assertNotContains(t, string(mainOut), `. "$_BESHT_ROOT/lib/legacy $(touch PWNED)`)
	cmd := exec.Command("sh", filepath.Join(outDir, "main.sh"))
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run split shell: %v\n%s\n--- main.sh ---\n%s", err, output, mainOut)
	}
	if string(output) != "split-safe:ok\n" {
		t.Fatalf("output: got %q", output)
	}
	if _, err := os.Stat(filepath.Join(dir, "PWNED")); !os.IsNotExist(err) {
		t.Fatalf("unsafe command substitution side effect present: %v", err)
	}
}

func TestSplit_ShellImportOutputCollisionRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/legacy.sh", `legacy() { printf '%s' "$1"; }
`)
	writeFile(t, dir, "lib/legacy.bsh", `export function fromBsh(): string {
    return "bsh"
}
`)
	writeFile(t, dir, "lib/use.bsh", `import { legacy } from "./legacy.sh" assert { type: "shell" }
export function callLegacy(): string {
    return legacy("x")
}
`)
	writeFile(t, dir, "main.bsh", `import { fromBsh } from "./lib/legacy"
import { callLegacy } from "./lib/use"
console.log(fromBsh())
console.log(callLegacy())
`)
	outDir := filepath.Join(dir, "out")
	err := codegen.CompileFileSplit(filepath.Join(dir, "main.bsh"), outDir, codegen.Options{})
	if err == nil || !strings.Contains(err.Error(), "split output collision") {
		t.Fatalf("CompileFileSplit error: got %v, want split output collision", err)
	}
}

func TestSplit_SingleFileCompileFileSplitAPI(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "script.bsh", `let x: string = "hello"
console.log(x)
`)
	outDir := filepath.Join(dir, "build")
	if err := codegen.CompileFileSplit(filepath.Join(dir, "script.bsh"), outDir, codegen.Options{}); err != nil {
		t.Fatalf("CompileFileSplit: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(outDir, "script.sh"))
	if err != nil {
		t.Fatalf("script.sh not created: %v", err)
	}
	out := string(content)
	assertContains(t, out, "#!/bin/sh")
	assertContains(t, out, `_BESHT_ROOT`)
	assertContains(t, out, `x='hello'`)
}
